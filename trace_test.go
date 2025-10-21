package shotel

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// mockTraceable implements Traceable for testing.
type mockTraceable struct {
	tracer *tracez.Tracer
}

func (m *mockTraceable) Tracer() *tracez.Tracer {
	return m.tracer
}

func TestShotelTraceObservation(t *testing.T) {
	// Create a test traceable with tracer
	tracer := tracez.New()
	traceable := &mockTraceable{tracer: tracer}

	// Create Shotel instance
	ctx := context.Background()
	cfg := DefaultConfig("trace-test")
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	// Observe traces
	shotel.ObserveTraces(traceable)

	// Verify observation was registered
	shotel.mu.Lock()
	if len(shotel.traceObservations) != 1 {
		t.Errorf("Expected 1 trace observation, got %d", len(shotel.traceObservations))
	}
	obs := shotel.traceObservations[0]
	if obs.traceable != traceable {
		t.Error("Traceable mismatch")
	}
	shotel.mu.Unlock()

	// Create a span
	spanCtx, span := tracer.StartSpan(ctx, "test.operation")
	span.SetTag("test.key", "test.value")
	span.SetIntTag("test.count", 42)
	span.Finish()

	// Give time for handler to execute
	time.Sleep(100 * time.Millisecond)

	// Verify tracer has handlers
	if !tracer.HasHandlers() {
		t.Error("Tracer should have handlers registered")
	}

	// Create child span
	_, childSpan := tracer.StartSpan(spanCtx, "test.child")
	childSpan.SetTag("child.key", "child.value")
	childSpan.Finish()

	time.Sleep(100 * time.Millisecond)
}

func TestShotelMultipleTraceables(t *testing.T) {
	// Create two traceables
	tracer1 := tracez.New()
	traceable1 := &mockTraceable{tracer: tracer1}

	tracer2 := tracez.New()
	traceable2 := &mockTraceable{tracer: tracer2}

	ctx := context.Background()
	cfg := DefaultConfig("multi-trace-test")
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	// Observe both
	shotel.ObserveTraces(traceable1)
	shotel.ObserveTraces(traceable2)

	shotel.mu.Lock()
	if len(shotel.traceObservations) != 2 {
		t.Errorf("Expected 2 trace observations, got %d", len(shotel.traceObservations))
	}
	shotel.mu.Unlock()

	// Create spans in both tracers
	_, span1 := tracer1.StartSpan(ctx, "tracer1.operation")
	span1.SetTag("source", "tracer1")
	span1.Finish()

	_, span2 := tracer2.StartSpan(ctx, "tracer2.operation")
	span2.SetTag("source", "tracer2")
	span2.Finish()

	time.Sleep(100 * time.Millisecond)

	if !tracer1.HasHandlers() {
		t.Error("Tracer1 should have handlers")
	}
	if !tracer2.HasHandlers() {
		t.Error("Tracer2 should have handlers")
	}
}

func TestShotelTraceWithMetrics(t *testing.T) {
	// Create observable that implements both Observable and Traceable
	type observableTraceable struct {
		*mockObservable
		*mockTraceable
	}

	registry := metricz.New()
	tracer := tracez.New()

	combined := &observableTraceable{
		mockObservable: &mockObservable{registry: registry},
		mockTraceable:  &mockTraceable{tracer: tracer},
	}

	ctx := context.Background()
	cfg := DefaultConfig("combined-test")
	cfg.MetricsInterval = 50 * time.Millisecond
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	// Observe both metrics and traces
	const CounterKey = metricz.Key("combined.counter")
	counter := registry.Counter(CounterKey)
	shotel.ObserveMetrics(combined.mockObservable, CounterKey)
	shotel.ObserveTraces(combined.mockTraceable)

	// Use both
	counter.Add(10)
	_, span := tracer.StartSpan(ctx, "combined.operation")
	span.SetTag("type", "combined")
	span.Finish()

	time.Sleep(100 * time.Millisecond)

	shotel.mu.Lock()
	if len(shotel.observations) != 1 {
		t.Errorf("Expected 1 metric observation, got %d", len(shotel.observations))
	}
	if len(shotel.traceObservations) != 1 {
		t.Errorf("Expected 1 trace observation, got %d", len(shotel.traceObservations))
	}
	shotel.mu.Unlock()

	if counter.Value() != 10.0 {
		t.Errorf("Counter value = %f, want 10.0", counter.Value())
	}
	if !tracer.HasHandlers() {
		t.Error("Tracer should have handlers")
	}
}

func TestShotelTraceShutdown(t *testing.T) {
	tracer := tracez.New()
	traceable := &mockTraceable{tracer: tracer}

	ctx := context.Background()
	cfg := DefaultConfig("shutdown-test")
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}

	shotel.ObserveTraces(traceable)

	if !tracer.HasHandlers() {
		t.Error("Tracer should have handlers before shutdown")
	}

	// Shutdown (allow more time for OTLP connection attempts to fail)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup

	// Handler should be removed
	if tracer.HasHandlers() {
		t.Error("Tracer should not have handlers after shutdown")
	}
}
