package aperture

import (
	"context"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/trace"
)

// pendingSpan holds start event data waiting for the corresponding end event.
type pendingSpan struct {
	startTime  time.Time       // time.Time (24 bytes)
	receivedAt time.Time       // For cleanup timeout
	startCtx   context.Context // interface (16 bytes)
	spanName      string       // strings (16 bytes each)
	correlationID string
}

// pendingEnd holds end event data waiting for the corresponding start event.
type pendingEnd struct {
	endTime    time.Time       // time.Time (24 bytes)
	receivedAt time.Time       // For cleanup timeout
	endCtx     context.Context // interface (16 bytes)
	correlationID string       // strings (16 bytes each)
	spanName      string
}

// tracesHandler manages trace correlation from signal pairs.
type tracesHandler struct {
	// Interface first (16 bytes, all pointers)
	tracer trace.Tracer

	// Pointers and maps (8 bytes each)
	pendingStarts map[string]*pendingSpan
	pendingEnds   map[string]*pendingEnd
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	internal      *internalObserver

	// Slices (pointer in first 8 bytes)
	config      []traceConfig
	contextKeys []ContextKey

	// Non-pointer fields
	maxTimeout time.Duration
	mu         sync.Mutex
}

// newTracesHandler creates a traces handler from config.
func newTracesHandler(s *Aperture) *tracesHandler {
	if len(s.config.Traces) == 0 {
		return nil
	}

	// Find maximum timeout from all trace configs
	var maxTimeout time.Duration
	for _, tc := range s.config.Traces {
		timeout := tc.SpanTimeout
		if timeout == 0 {
			timeout = 5 * time.Minute // Default per-config
		}
		if timeout > maxTimeout {
			maxTimeout = timeout
		}
	}

	// Extract context keys if configured
	var contextKeys []ContextKey
	if s.config.ContextExtraction != nil {
		contextKeys = s.config.ContextExtraction.Traces
	}

	th := &tracesHandler{
		tracer:        s.traceProvider.Tracer("capitan"),
		config:        s.config.Traces,
		pendingStarts: make(map[string]*pendingSpan),
		pendingEnds:   make(map[string]*pendingEnd),
		stopCleanup:   make(chan struct{}),
		maxTimeout:    maxTimeout,
		contextKeys:   contextKeys,
		internal:      s.internalObserver,
	}

	// Start cleanup goroutine
	th.startCleanup()

	return th
}

// startCleanup begins periodic cleanup of stale spans.
func (th *tracesHandler) startCleanup() {
	// Run cleanup every minute
	th.cleanupTicker = time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-th.cleanupTicker.C:
				th.cleanupStaleSpans()
			case <-th.stopCleanup:
				return
			}
		}
	}()
}

// cleanupStaleSpans removes pending starts and ends that have exceeded their timeout.
func (th *tracesHandler) cleanupStaleSpans() {
	th.mu.Lock()
	defer th.mu.Unlock()

	now := time.Now()

	// Clean up stale pending starts
	for id, pending := range th.pendingStarts {
		age := now.Sub(pending.receivedAt)
		if age > th.maxTimeout {
			th.internal.emit(pending.startCtx, SignalTraceExpired,
				internalCorrelationID.Field(pending.correlationID),
				internalSpanName.Field(pending.spanName),
				internalReason.Field("end event not received"),
			)
			delete(th.pendingStarts, id)
		}
	}

	// Clean up stale pending ends
	for id, pending := range th.pendingEnds {
		age := now.Sub(pending.receivedAt)
		if age > th.maxTimeout {
			th.internal.emit(pending.endCtx, SignalTraceExpired,
				internalCorrelationID.Field(pending.correlationID),
				internalSpanName.Field(pending.spanName),
				internalReason.Field("start event not received"),
			)
			delete(th.pendingEnds, id)
		}
	}
}

// Close stops the cleanup goroutine and discards pending starts and ends.
func (th *tracesHandler) Close() {
	if th == nil {
		return
	}

	if th.cleanupTicker != nil {
		th.cleanupTicker.Stop()
	}

	close(th.stopCleanup)

	// Discard all pending starts and ends
	th.mu.Lock()
	defer th.mu.Unlock()

	for id := range th.pendingStarts {
		delete(th.pendingStarts, id)
	}
	for id := range th.pendingEnds {
		delete(th.pendingEnds, id)
	}
}

// handleEvent checks if the event starts or ends a configured trace span.
func (th *tracesHandler) handleEvent(ctx context.Context, e *capitan.Event) {
	if th == nil {
		return
	}

	signalName := e.Signal().Name()

	// Check each trace configuration (match by signal name)
	for _, tc := range th.config {
		switch signalName {
		case tc.StartSignalName:
			th.handleStart(ctx, e, tc)
		case tc.EndSignalName:
			th.handleEnd(ctx, e, tc)
		}
	}
}

// handleStart stores the start event data or creates span if end already received.
func (th *tracesHandler) handleStart(ctx context.Context, e *capitan.Event, tc traceConfig) {
	// Determine span name for diagnostics
	spanName := tc.SpanName
	if spanName == "" {
		spanName = tc.StartSignalName
	}

	// Extract correlation ID from event (by key name)
	correlationID := extractStringFieldByName(e, tc.CorrelationKeyName)
	if correlationID == "" {
		// Emit diagnostic for missing correlation ID
		th.internal.emit(ctx, SignalTraceCorrelationMissing,
			internalSignal.Field(e.Signal().Name()),
			internalSpanName.Field(spanName),
			internalCorrelationKey.Field(tc.CorrelationKeyName),
		)
		return
	}

	// Create composite key to prevent collisions between different trace configs
	compositeKey := th.makeCompositeKey(correlationID, tc.StartSignalName, tc.EndSignalName)

	th.mu.Lock()
	defer th.mu.Unlock()

	// Check if end event already arrived
	if pendingEnd, ok := th.pendingEnds[compositeKey]; ok {
		// End arrived first - create span now with both timestamps
		// e is the start event, pendingEnd has the end event
		delete(th.pendingEnds, compositeKey)
		th.mu.Unlock()

		_, span := th.tracer.Start(ctx, spanName, trace.WithTimestamp(e.Timestamp()))

		// Add context attributes if configured
		if len(th.contextKeys) > 0 {
			contextAttrs := extractContextValuesForMetrics(ctx, th.contextKeys)
			span.SetAttributes(contextAttrs...)
		}

		span.End(trace.WithTimestamp(pendingEnd.endTime))

		th.mu.Lock()
		return
	}

	// No end yet - store start event data
	th.pendingStarts[compositeKey] = &pendingSpan{
		startTime:     e.Timestamp(),
		startCtx:      ctx,
		spanName:      spanName,
		correlationID: correlationID,
		receivedAt:    time.Now(),
	}
}

// handleEnd stores the end event data or creates span if start already received.
func (th *tracesHandler) handleEnd(ctx context.Context, e *capitan.Event, tc traceConfig) {
	// Determine span name for diagnostics
	spanName := tc.SpanName
	if spanName == "" {
		spanName = tc.StartSignalName
	}

	// Extract correlation ID from event (by key name)
	correlationID := extractStringFieldByName(e, tc.CorrelationKeyName)
	if correlationID == "" {
		// Emit diagnostic for missing correlation ID
		th.internal.emit(ctx, SignalTraceCorrelationMissing,
			internalSignal.Field(e.Signal().Name()),
			internalSpanName.Field(spanName),
			internalCorrelationKey.Field(tc.CorrelationKeyName),
		)
		return
	}

	// Create composite key to prevent collisions between different trace configs
	compositeKey := th.makeCompositeKey(correlationID, tc.StartSignalName, tc.EndSignalName)

	th.mu.Lock()
	defer th.mu.Unlock()

	// Check if start event already arrived
	if pendingStart, ok := th.pendingStarts[compositeKey]; ok {
		// Start arrived first - create span now with both timestamps
		delete(th.pendingStarts, compositeKey)
		th.mu.Unlock()

		_, span := th.tracer.Start(pendingStart.startCtx, pendingStart.spanName,
			trace.WithTimestamp(pendingStart.startTime))

		// Add context attributes if configured (use start context)
		if len(th.contextKeys) > 0 {
			contextAttrs := extractContextValuesForMetrics(pendingStart.startCtx, th.contextKeys)
			span.SetAttributes(contextAttrs...)
		}

		span.End(trace.WithTimestamp(e.Timestamp()))

		th.mu.Lock()
		return
	}

	// No start yet - store end event data
	th.pendingEnds[compositeKey] = &pendingEnd{
		endTime:       e.Timestamp(),
		endCtx:        ctx,
		correlationID: correlationID,
		spanName:      spanName,
		receivedAt:    time.Now(),
	}
}

// makeCompositeKey creates a unique key combining correlation ID and signal names.
// This prevents collisions when multiple trace configs share the same correlation ID.
func (*tracesHandler) makeCompositeKey(correlationID, startSignalName, endSignalName string) string {
	return correlationID + ":" + startSignalName + ":" + endSignalName
}

// extractStringFieldByName gets a string field value from the event fields by key name.
func extractStringFieldByName(e *capitan.Event, keyName string) string {
	if keyName == "" {
		return ""
	}

	for _, f := range e.Fields() {
		// Match by key name and string variant
		if f.Key().Name() == keyName && f.Variant() == capitan.VariantString {
			if gf, ok := f.(capitan.GenericField[string]); ok {
				return gf.Get()
			}
		}
	}

	return ""
}
