package shotel

import (
	"context"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/trace"
)

// pendingSpan holds start event data waiting for the corresponding end event.
type pendingSpan struct {
	startTime  time.Time
	startCtx   context.Context
	spanName   string
	receivedAt time.Time // For cleanup timeout
}

// pendingEnd holds end event data waiting for the corresponding start event.
type pendingEnd struct {
	endTime    time.Time
	endCtx     context.Context
	receivedAt time.Time // For cleanup timeout
}

// tracesHandler manages trace correlation from signal pairs.
type tracesHandler struct {
	tracer trace.Tracer
	config []TraceConfig

	// Track pending starts and ends by correlation ID
	pendingStarts map[string]*pendingSpan
	pendingEnds   map[string]*pendingEnd
	mu            sync.Mutex

	// Cleanup management
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	maxTimeout    time.Duration
}

// newTracesHandler creates a traces handler from config.
func newTracesHandler(s *Shotel) *tracesHandler {
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

	th := &tracesHandler{
		tracer:        s.traceProvider.Tracer("capitan"),
		config:        s.config.Traces,
		pendingStarts: make(map[string]*pendingSpan),
		pendingEnds:   make(map[string]*pendingEnd),
		stopCleanup:   make(chan struct{}),
		maxTimeout:    maxTimeout,
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
			delete(th.pendingStarts, id)
		}
	}

	// Clean up stale pending ends
	for id, pending := range th.pendingEnds {
		age := now.Sub(pending.receivedAt)
		if age > th.maxTimeout {
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

	signal := e.Signal()

	// Check each trace configuration
	for _, tc := range th.config {
		if signal == tc.Start {
			th.handleStart(ctx, e, tc)
		} else if signal == tc.End {
			th.handleEnd(ctx, e, tc)
		}
	}
}

// handleStart stores the start event data or creates span if end already received.
func (th *tracesHandler) handleStart(ctx context.Context, e *capitan.Event, tc TraceConfig) {
	// Extract correlation ID from event
	correlationID := th.extractCorrelationID(e, tc.CorrelationKey)
	if correlationID == "" {
		return // No correlation ID, cannot track
	}

	// Create composite key to prevent collisions between different trace configs
	compositeKey := th.makeCompositeKey(correlationID, tc.Start, tc.End)

	// Determine span name
	spanName := tc.SpanName
	if spanName == "" {
		spanName = tc.Start.Name()
	}

	th.mu.Lock()
	defer th.mu.Unlock()

	// Check if end event already arrived
	if pendingEnd, ok := th.pendingEnds[compositeKey]; ok {
		// End arrived first - create span now with both timestamps
		// e is the start event, pendingEnd has the end event
		delete(th.pendingEnds, compositeKey)
		th.mu.Unlock()

		_, span := th.tracer.Start(ctx, spanName, trace.WithTimestamp(e.Timestamp()))
		span.End(trace.WithTimestamp(pendingEnd.endTime))

		th.mu.Lock()
		return
	}

	// No end yet - store start event data
	th.pendingStarts[compositeKey] = &pendingSpan{
		startTime:  e.Timestamp(),
		startCtx:   ctx,
		spanName:   spanName,
		receivedAt: time.Now(),
	}
}

// handleEnd stores the end event data or creates span if start already received.
func (th *tracesHandler) handleEnd(ctx context.Context, e *capitan.Event, tc TraceConfig) {
	// Extract correlation ID from event
	correlationID := th.extractCorrelationID(e, tc.CorrelationKey)
	if correlationID == "" {
		return
	}

	// Create composite key to prevent collisions between different trace configs
	compositeKey := th.makeCompositeKey(correlationID, tc.Start, tc.End)

	th.mu.Lock()
	defer th.mu.Unlock()

	// Check if start event already arrived
	if pendingStart, ok := th.pendingStarts[compositeKey]; ok {
		// Start arrived first - create span now with both timestamps
		delete(th.pendingStarts, compositeKey)
		th.mu.Unlock()

		_, span := th.tracer.Start(pendingStart.startCtx, pendingStart.spanName,
			trace.WithTimestamp(pendingStart.startTime))
		span.End(trace.WithTimestamp(e.Timestamp()))

		th.mu.Lock()
		return
	}

	// No start yet - store end event data
	th.pendingEnds[compositeKey] = &pendingEnd{
		endTime:    e.Timestamp(),
		endCtx:     ctx,
		receivedAt: time.Now(),
	}
}

// makeCompositeKey creates a unique key combining correlation ID and signal names.
// This prevents collisions when multiple trace configs share the same correlation ID.
func (*tracesHandler) makeCompositeKey(correlationID string, start, end capitan.Signal) string {
	return correlationID + ":" + start.Name() + ":" + end.Name()
}

// extractCorrelationID gets the correlation ID from the event fields.
func (*tracesHandler) extractCorrelationID(e *capitan.Event, key *capitan.StringKey) string {
	if key == nil {
		return ""
	}

	// Compare keys by their Name() since Key interface doesn't define equality
	keyName := key.Name()
	for _, f := range e.Fields() {
		fKey := f.Key()
		// Match by name and variant
		if fKey.Name() == keyName && fKey.Variant() == key.Variant() && f.Variant() == capitan.VariantString {
			if gf, ok := f.(capitan.GenericField[string]); ok {
				return gf.Get()
			}
		}
	}

	return ""
}
