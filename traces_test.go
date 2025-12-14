package aperture

import (
	"context"
	"sync"
	"testing"
	"time"

	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
)

func TestTraceSpanCleanup(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	_ = capitan.NewSignal("request.started", "Request Started")
	_ = capitan.NewSignal("request.completed", "Request Completed")
	_ = capitan.NewStringKey("request_id")

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
				SpanName:       "http_request",
				SpanTimeout:    "5s",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	th := sh.capitanObserver.tracesHandler
	if th == nil {
		t.Fatal("traces handler is nil")
	}

	// Manually insert old pending events to test cleanup logic
	th.mu.Lock()
	th.pendingStarts["old-start"] = &pendingSpan{
		startTime:  time.Now(),
		startCtx:   ctx,
		spanName:   "old_span",
		receivedAt: time.Now().Add(-10 * time.Second), // 10 seconds ago
	}
	th.pendingEnds["old-end"] = &pendingEnd{
		endTime:    time.Now(),
		endCtx:     ctx,
		receivedAt: time.Now().Add(-10 * time.Second), // 10 seconds ago
	}
	th.pendingStarts["recent-start"] = &pendingSpan{
		startTime:  time.Now(),
		startCtx:   ctx,
		spanName:   "recent_span",
		receivedAt: time.Now().Add(-1 * time.Second), // 1 second ago
	}
	th.mu.Unlock()

	// Verify we have 3 pending events
	th.mu.Lock()
	totalBefore := len(th.pendingStarts) + len(th.pendingEnds)
	th.mu.Unlock()
	if totalBefore != 3 {
		t.Errorf("expected 3 pending events before cleanup, got %d", totalBefore)
	}

	// Run cleanup - should remove events older than 5 seconds
	t.Logf("maxTimeout: %v", th.maxTimeout)
	th.cleanupStaleSpans()

	// Verify old events removed, recent kept
	th.mu.Lock()
	startsAfter := len(th.pendingStarts)
	endsAfter := len(th.pendingEnds)
	totalAfter := startsAfter + endsAfter
	th.mu.Unlock()

	if totalAfter != 1 {
		t.Errorf("expected 1 pending event after cleanup, got %d (starts: %d, ends: %d)",
			totalAfter, startsAfter, endsAfter)
	}

	// Verify the recent one is still there
	th.mu.Lock()
	if _, ok := th.pendingStarts["recent-start"]; !ok {
		t.Error("expected recent-start to still be present")
	}
	if _, ok := th.pendingStarts["old-start"]; ok {
		t.Error("expected old-start to be cleaned up")
	}
	if _, ok := th.pendingEnds["old-end"]; ok {
		t.Error("expected old-end to be cleaned up")
	}
	th.mu.Unlock()
}

func TestTraceSpanCompletesBeforeTimeout(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	requestStarted := capitan.NewSignal("request.started", "Request Started")
	requestCompleted := capitan.NewSignal("request.completed", "Request Completed")
	requestIDKey := capitan.NewStringKey("request_id")

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
				SpanName:       "http_request",
				SpanTimeout:    "5s",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Add listeners for both events
	var wg sync.WaitGroup
	wg.Add(2)
	listener := cap.Observe(func(ctx context.Context, e *capitan.Event) {
		if e.Signal() == requestStarted || e.Signal() == requestCompleted {
			// Give aperture time to process before we signal done
			time.Sleep(10 * time.Millisecond)
			wg.Done()
		}
	})

	// Emit matched start/end events
	cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-123"))
	cap.Emit(ctx, requestCompleted, requestIDKey.Field("REQ-123"))
	wg.Wait()
	listener.Close()

	// Verify span was completed (both pending maps should be empty)
	th := sh.capitanObserver.tracesHandler
	th.mu.Lock()
	totalPending := len(th.pendingStarts) + len(th.pendingEnds)
	if totalPending != 0 {
		t.Errorf("expected 0 pending events after completion, got %d (starts: %d, ends: %d)",
			totalPending, len(th.pendingStarts), len(th.pendingEnds))
	}
	th.mu.Unlock()
}

func TestTraceDefaultTimeout(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	_ = capitan.NewSignal("request.started", "Request Started")
	_ = capitan.NewSignal("request.completed", "Request Completed")
	_ = capitan.NewStringKey("request_id")

	// No timeout specified - should default to 5 minutes
	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	th := sh.capitanObserver.tracesHandler
	if th.maxTimeout != 5*time.Minute {
		t.Errorf("expected default timeout of 5 minutes, got %v", th.maxTimeout)
	}
}

func TestTraceCloseEndsAllSpans(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	requestStarted := capitan.NewSignal("request.started", "Request Started")
	_ = capitan.NewSignal("request.completed", "Request Completed")
	requestIDKey := capitan.NewStringKey("request_id")

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
				SpanTimeout:    "10s",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Add listener AFTER aperture to ensure aperture processes first
	var wg sync.WaitGroup
	wg.Add(3)
	listener := cap.Observe(func(ctx context.Context, e *capitan.Event) {
		if e.Signal() == requestStarted {
			// Give aperture time to process before we signal done
			time.Sleep(10 * time.Millisecond)
			wg.Done()
		}
	})

	// Create multiple orphaned spans
	cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-1"))
	cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-2"))
	cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-3"))
	wg.Wait()
	listener.Close()

	th := sh.capitanObserver.tracesHandler
	th.mu.Lock()
	totalPending := len(th.pendingStarts) + len(th.pendingEnds)
	th.mu.Unlock()

	if totalPending != 3 {
		t.Errorf("expected 3 pending events, got %d", totalPending)
	}

	// Close should discard all pending events
	sh.Close()

	th.mu.Lock()
	remainingPending := len(th.pendingStarts) + len(th.pendingEnds)
	th.mu.Unlock()

	if remainingPending != 0 {
		t.Errorf("expected 0 pending events after shutdown, got %d", remainingPending)
	}
}

func TestTraceCompositeKeyPreventsCollisions(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define two different trace configs that will use the same correlation ID
	dbQueryStarted := capitan.NewSignal("db.query.started", "Db Query Started")
	dbQueryCompleted := capitan.NewSignal("db.query.completed", "Db Query Completed")
	httpRequestStarted := capitan.NewSignal("http.request.started", "Http Request Started")
	httpRequestCompleted := capitan.NewSignal("http.request.completed", "Http Request Completed")
	correlationIDKey := capitan.NewStringKey("correlation_id")

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "db.query.started",
				End:            "db.query.completed",
				CorrelationKey: "correlation_id",
				SpanName:       "database_query",
				SpanTimeout:    "5s",
			},
			{
				Start:          "http.request.started",
				End:            "http.request.completed",
				CorrelationKey: "correlation_id",
				SpanName:       "http_request",
				SpanTimeout:    "5s",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(4)
	listener := cap.Observe(func(ctx context.Context, e *capitan.Event) {
		signal := e.Signal()
		if signal == dbQueryStarted || signal == dbQueryCompleted ||
			signal == httpRequestStarted || signal == httpRequestCompleted {
			time.Sleep(10 * time.Millisecond)
			wg.Done()
		}
	})

	// Emit events with the same correlation ID for both trace configs
	sharedID := "shared-123"
	cap.Emit(ctx, dbQueryStarted, correlationIDKey.Field(sharedID))
	cap.Emit(ctx, httpRequestStarted, correlationIDKey.Field(sharedID))
	cap.Emit(ctx, dbQueryCompleted, correlationIDKey.Field(sharedID))
	cap.Emit(ctx, httpRequestCompleted, correlationIDKey.Field(sharedID))
	wg.Wait()
	listener.Close()

	// Both spans should complete without collision
	th := sh.capitanObserver.tracesHandler
	th.mu.Lock()
	totalPending := len(th.pendingStarts) + len(th.pendingEnds)
	th.mu.Unlock()

	if totalPending != 0 {
		t.Errorf("expected 0 pending events (both spans completed), got %d (starts: %d, ends: %d)",
			totalPending, len(th.pendingStarts), len(th.pendingEnds))
	}
}
