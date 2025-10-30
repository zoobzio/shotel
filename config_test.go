package shotel

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestConfigMetrics(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Define signals
	orderCreated := capitan.Signal("order.created")
	orderFailed := capitan.Signal("order.failed")

	// Configure metrics
	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:      orderCreated,
				Name:        "orders_created_total",
				Description: "Total number of orders created",
			},
			{
				Signal:      orderFailed,
				Name:        "orders_failed_total",
				Description: "Total number of failed orders",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit events
	orderIDKey := capitan.NewStringKey("order_id")
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-123"))
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-124"))
	cap.Emit(ctx, orderFailed, orderIDKey.Field("ORDER-125"))

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Metrics should be incremented (validation would require OTLP capture)
}

func TestConfigLogWhitelist(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Define signals
	orderCreated := capitan.Signal("order.created")
	orderFailed := capitan.Signal("order.failed")
	orderCanceled := capitan.Signal("order.canceled")

	// Configure log whitelist - only log created and failed
	config := &Config{
		Logs: &LogConfig{
			Whitelist: []capitan.Signal{orderCreated, orderFailed},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit events
	orderIDKey := capitan.NewStringKey("order_id")
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-123"))  // Should be logged
	cap.Emit(ctx, orderFailed, orderIDKey.Field("ORDER-124"))   // Should be logged
	cap.Emit(ctx, orderCanceled, orderIDKey.Field("ORDER-125")) // Should NOT be logged

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Log filtering is applied (validation would require OTLP capture)
}

func TestConfigTraces(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Define signals
	requestStarted := capitan.Signal("request.started")
	requestCompleted := capitan.Signal("request.completed")
	requestIDKey := capitan.NewStringKey("request_id")

	// Configure traces
	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          requestStarted,
				End:            requestCompleted,
				CorrelationKey: &requestIDKey,
				SpanName:       "http_request",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit correlated events
	cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-001"))
	time.Sleep(50 * time.Millisecond)
	cap.Emit(ctx, requestCompleted, requestIDKey.Field("REQ-001"))

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Span should be created and completed (validation would require OTLP capture)
}

func TestConfigCombined(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Define signals
	orderCreated := capitan.Signal("order.created")
	orderCompleted := capitan.Signal("order.completed")
	orderIDKey := capitan.NewStringKey("order_id")

	// Configure all three: metrics, logs, traces
	config := &Config{
		Metrics: []MetricConfig{
			{Signal: orderCreated, Name: "orders_created_total"},
			{Signal: orderCompleted, Name: "orders_completed_total"},
		},
		Logs: &LogConfig{
			Whitelist: []capitan.Signal{orderCreated, orderCompleted},
		},
		Traces: []TraceConfig{
			{
				Start:          orderCreated,
				End:            orderCompleted,
				CorrelationKey: &orderIDKey,
				SpanName:       "order_processing",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit events that should trigger all three signal types
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORD-001"))
	time.Sleep(50 * time.Millisecond)
	cap.Emit(ctx, orderCompleted, orderIDKey.Field("ORD-001"))

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// All signals should be handled: metric incremented, logs emitted, span created
}

func TestEmptyConfig(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Empty config - should behave like nil (all events logged)
	config := &Config{}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit event
	testSig := capitan.Signal("test.event")
	cap.Emit(ctx, testSig)

	time.Sleep(100 * time.Millisecond)

	// Should log all events (default behavior)
}
