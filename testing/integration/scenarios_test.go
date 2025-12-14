package integration

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/aperture"
	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestScenario_MetricsCounter(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderID := capitan.NewStringKey("order_id")

	schema := aperture.Schema{
		Metrics: []aperture.MetricSchema{
			{
				Signal:      "order.created",
				Name:        "orders_created_total",
				Type:        "counter",
				Description: "Total orders created",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit multiple events
	for i := 0; i < 5; i++ {
		cap.Emit(ctx, orderCreated, orderID.Field("ORDER-"+string(rune('A'+i))))
	}

	// Allow async processing
	time.Sleep(50 * time.Millisecond)

	// Test passes if no panics - actual metric values would need OTEL collector
}

func TestScenario_MetricsGauge(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	cpuUsage := capitan.NewSignal("system.cpu", "CPU usage measurement")
	percentKey := capitan.NewFloat64Key("percent")

	schema := aperture.Schema{
		Metrics: []aperture.MetricSchema{
			{
				Signal:   "system.cpu",
				Name:     "cpu_usage_percent",
				Type:     "gauge",
				ValueKey: "percent",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit gauge readings
	cap.Emit(ctx, cpuUsage, percentKey.Field(45.5))
	cap.Emit(ctx, cpuUsage, percentKey.Field(67.2))
	cap.Emit(ctx, cpuUsage, percentKey.Field(23.1))

	time.Sleep(50 * time.Millisecond)
}

func TestScenario_MetricsHistogram(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	requestDone := capitan.NewSignal("request.done", "Request completed")
	durationKey := capitan.NewDurationKey("duration")

	schema := aperture.Schema{
		Metrics: []aperture.MetricSchema{
			{
				Signal:   "request.done",
				Name:     "request_duration_ms",
				Type:     "histogram",
				ValueKey: "duration",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit duration measurements
	durations := []time.Duration{
		10 * time.Millisecond,
		25 * time.Millisecond,
		100 * time.Millisecond,
		5 * time.Millisecond,
		50 * time.Millisecond,
	}

	for _, d := range durations {
		cap.Emit(ctx, requestDone, durationKey.Field(d))
	}

	time.Sleep(50 * time.Millisecond)
}

func TestScenario_TraceCorrelation(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	reqStarted := capitan.NewSignal("request.started", "Request started")
	reqCompleted := capitan.NewSignal("request.completed", "Request completed")
	requestID := capitan.NewStringKey("request_id")

	schema := aperture.Schema{
		Traces: []aperture.TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
				SpanName:       "http_request",
				SpanTimeout:    "1m",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit correlated events
	cap.Emit(ctx, reqStarted, requestID.Field("REQ-001"))
	time.Sleep(10 * time.Millisecond)
	cap.Emit(ctx, reqCompleted, requestID.Field("REQ-001"))

	// Emit another pair
	cap.Emit(ctx, reqStarted, requestID.Field("REQ-002"))
	time.Sleep(5 * time.Millisecond)
	cap.Emit(ctx, reqCompleted, requestID.Field("REQ-002"))

	time.Sleep(50 * time.Millisecond)
}

func TestScenario_TraceOutOfOrder(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	reqStarted := capitan.NewSignal("request.started", "Request started")
	reqCompleted := capitan.NewSignal("request.completed", "Request completed")
	requestID := capitan.NewStringKey("request_id")

	schema := aperture.Schema{
		Traces: []aperture.TraceSchema{
			{
				Start:          "request.started",
				End:            "request.completed",
				CorrelationKey: "request_id",
				SpanName:       "http_request",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// End arrives before start (out of order)
	cap.Emit(ctx, reqCompleted, requestID.Field("REQ-OOO"))
	time.Sleep(10 * time.Millisecond)
	cap.Emit(ctx, reqStarted, requestID.Field("REQ-OOO"))

	time.Sleep(50 * time.Millisecond)
}

func TestScenario_LogWhitelist(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderFailed := capitan.NewSignal("order.failed", "Order failed")
	orderShipped := capitan.NewSignal("order.shipped", "Order shipped")

	schema := aperture.Schema{
		Logs: &aperture.LogSchema{
			Whitelist: []string{
				"order.created",
				"order.failed",
				// orderShipped intentionally excluded
			},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit all three signals
	cap.Emit(ctx, orderCreated)
	cap.Emit(ctx, orderFailed)
	cap.Emit(ctx, orderShipped) // Should be filtered

	cap.Shutdown()

	// Only whitelisted signals should be logged
	records := mockLog.Capture().Records()
	if len(records) != 2 {
		t.Errorf("expected 2 log records (whitelist filtering), got %d", len(records))
	}
}

func TestScenario_LogAllEvents(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig1 := capitan.NewSignal("event.one", "Event one")
	sig2 := capitan.NewSignal("event.two", "Event two")
	sig3 := capitan.NewSignal("event.three", "Event three")

	// No log config = log all events
	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	cap.Emit(ctx, sig1)
	cap.Emit(ctx, sig2)
	cap.Emit(ctx, sig3)

	cap.Shutdown()

	records := mockLog.Capture().Records()
	if len(records) != 3 {
		t.Errorf("expected 3 log records (no whitelist), got %d", len(records))
	}
}

func TestScenario_ContextExtraction(t *testing.T) {
	ctx := context.Background()

	type ctxKey string
	const userIDKey ctxKey = "user_id"

	cap := capitan.New()
	defer cap.Shutdown()

	orderCreated := capitan.NewSignal("order.created", "Order created")

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	// Register context key first
	ap.RegisterContextKey("user_id", userIDKey)

	schema := aperture.Schema{
		Context: &aperture.ContextSchema{
			Logs: []string{"user_id"},
		},
	}

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Add user ID to context
	ctx = context.WithValue(ctx, userIDKey, "user-123")

	cap.Emit(ctx, orderCreated)
	cap.Shutdown()

	records := mockLog.Capture().Records()
	if len(records) != 1 {
		t.Errorf("expected 1 log record, got %d", len(records))
	}

	// Context extraction adds attributes to the log record
	// The actual verification would check record attributes
}

func TestScenario_StdoutLogging(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("test.stdout", "Test stdout logging")

	schema := aperture.Schema{
		Stdout: true,
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit event - should also log to stdout
	cap.Emit(ctx, sig)
	cap.Shutdown()

	// Test passes if no panic - stdout output is visible in test output
}

func TestScenario_CombinedConfiguration(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderCompleted := capitan.NewSignal("order.completed", "Order completed")
	orderID := capitan.NewStringKey("order_id")
	totalKey := capitan.NewFloat64Key("total")

	schema := aperture.Schema{
		Metrics: []aperture.MetricSchema{
			{
				Signal: "order.created",
				Name:   "orders_created_total",
				Type:   "counter",
			},
			{
				Signal:   "order.completed",
				Name:     "order_value",
				Type:     "histogram",
				ValueKey: "total",
			},
		},
		Traces: []aperture.TraceSchema{
			{
				Start:          "order.created",
				End:            "order.completed",
				CorrelationKey: "order_id",
				SpanName:       "order_processing",
			},
		},
		Logs: &aperture.LogSchema{
			Whitelist: []string{"order.created", "order.completed"},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	err = ap.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Emit a complete order flow
	cap.Emit(ctx, orderCreated, orderID.Field("ORD-001"))
	time.Sleep(10 * time.Millisecond)
	cap.Emit(ctx, orderCompleted, orderID.Field("ORD-001"), totalKey.Field(99.99))

	time.Sleep(50 * time.Millisecond)
}
