package aperture

import (
	"context"
	"testing"
	"time"

	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
)

func TestSchemaMetrics(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	_ = capitan.NewSignal("order.created", "Order created")
	_ = capitan.NewSignal("order.failed", "Order failed")

	// Configure metrics via schema
	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:      "order.created",
				Name:        "orders_created_total",
				Description: "Total number of orders created",
			},
			{
				Signal:      "order.failed",
				Name:        "orders_failed_total",
				Description: "Total number of failed orders",
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

	// Emit events
	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderFailed := capitan.NewSignal("order.failed", "Order failed")
	orderIDKey := capitan.NewStringKey("order_id")
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-123"))
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-124"))
	cap.Emit(ctx, orderFailed, orderIDKey.Field("ORDER-125"))

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Metrics should be incremented (validation would require OTLP capture)
}

func TestSchemaLogWhitelist(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderFailed := capitan.NewSignal("order.failed", "Order failed")
	orderCanceled := capitan.NewSignal("order.canceled", "Order canceled")

	// Configure log whitelist - only log created and failed
	schema := Schema{
		Logs: &LogSchema{
			Whitelist: []string{"order.created", "order.failed"},
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

	// Emit events
	orderIDKey := capitan.NewStringKey("order_id")
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-123"))  // Should be logged
	cap.Emit(ctx, orderFailed, orderIDKey.Field("ORDER-124"))   // Should be logged
	cap.Emit(ctx, orderCanceled, orderIDKey.Field("ORDER-125")) // Should NOT be logged

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Log filtering is applied (validation would require OTLP capture)
}

func TestSchemaTraces(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	requestStarted := capitan.NewSignal("request.started", "Request started")
	requestCompleted := capitan.NewSignal("request.completed", "Request completed")
	requestIDKey := capitan.NewStringKey("request_id")

	// Configure traces via schema
	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
				SpanName:       "http_request",
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

	// Emit correlated events
	cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-001"))
	time.Sleep(50 * time.Millisecond)
	cap.Emit(ctx, requestCompleted, requestIDKey.Field("REQ-001"))

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// Span should be created and completed (validation would require OTLP capture)
}

func TestSchemaCombined(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderCompleted := capitan.NewSignal("order.completed", "Order completed")
	orderIDKey := capitan.NewStringKey("order_id")

	// Configure all three: metrics, logs, traces
	schema := Schema{
		Metrics: []MetricSchema{
			{Signal: "order.created", Name: "orders_created_total"},
			{Signal: "order.completed", Name: "orders_completed_total"},
		},
		Logs: &LogSchema{
			Whitelist: []string{"order.created", "order.completed"},
		},
		Traces: []TraceSchema{
			{
				Start:          "order.created",
				End:            "order.completed",
				CorrelationKey: "order_id",
				SpanName:       "order_processing",
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

	// Emit events that should trigger all three signal types
	cap.Emit(ctx, orderCreated, orderIDKey.Field("ORD-001"))
	time.Sleep(50 * time.Millisecond)
	cap.Emit(ctx, orderCompleted, orderIDKey.Field("ORD-001"))

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	// All signals should be handled: metric incremented, logs emitted, span created
}

func TestEmptySchema(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Empty schema - should behave like default (all events logged)
	schema := Schema{}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit event
	testSig := capitan.NewSignal("test.event", "Test event")
	cap.Emit(ctx, testSig)

	time.Sleep(100 * time.Millisecond)

	// Should log all events (default behavior)
}

func TestParseMetricType(t *testing.T) {
	tests := []struct {
		input    string
		expected MetricType
	}{
		{"counter", MetricTypeCounter},
		{"", MetricTypeCounter},
		{"gauge", MetricTypeGauge},
		{"histogram", MetricTypeHistogram},
		{"updowncounter", MetricTypeUpDownCounter},
		{"unknown", MetricTypeCounter}, // defaults to counter
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMetricType(tt.input)
			if result != tt.expected {
				t.Errorf("parseMetricType(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"", 5 * time.Minute},
		{"invalid", 5 * time.Minute},
		{"30s", 30 * time.Second},
		{"10m", 10 * time.Minute},
		{"1h", 1 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTimeout(tt.input)
			if result != tt.expected {
				t.Errorf("parseTimeout(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
