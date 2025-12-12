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

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal:      orderCreated,
				Name:        "orders_created_total",
				Type:        aperture.MetricTypeCounter,
				Description: "Total orders created",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal:   cpuUsage,
				Name:     "cpu_usage_percent",
				Type:     aperture.MetricTypeGauge,
				ValueKey: percentKey,
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal:   requestDone,
				Name:     "request_duration_ms",
				Type:     aperture.MetricTypeHistogram,
				ValueKey: durationKey,
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		Traces: []aperture.TraceConfig{
			{
				Start:          reqStarted,
				End:            reqCompleted,
				CorrelationKey: &requestID,
				SpanName:       "http_request",
				SpanTimeout:    time.Minute,
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		Traces: []aperture.TraceConfig{
			{
				Start:          reqStarted,
				End:            reqCompleted,
				CorrelationKey: &requestID,
				SpanName:       "http_request",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		Logs: &aperture.LogConfig{
			Whitelist: []capitan.Signal{
				orderCreated,
				orderFailed,
				// orderShipped intentionally excluded
			},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
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

	config := &aperture.Config{
		ContextExtraction: &aperture.ContextExtractionConfig{
			Logs: []aperture.ContextKey{
				{Key: userIDKey, Name: "user_id"},
			},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		StdoutLogging: true,
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

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

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal: orderCreated,
				Name:   "orders_created_total",
				Type:   aperture.MetricTypeCounter,
			},
			{
				Signal:   orderCompleted,
				Name:     "order_value",
				Type:     aperture.MetricTypeHistogram,
				ValueKey: totalKey,
			},
		},
		Traces: []aperture.TraceConfig{
			{
				Start:          orderCreated,
				End:            orderCompleted,
				CorrelationKey: &orderID,
				SpanName:       "order_processing",
			},
		},
		Logs: &aperture.LogConfig{
			Whitelist: []capitan.Signal{orderCreated, orderCompleted},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	// Emit a complete order flow
	cap.Emit(ctx, orderCreated, orderID.Field("ORD-001"))
	time.Sleep(10 * time.Millisecond)
	cap.Emit(ctx, orderCompleted, orderID.Field("ORD-001"), totalKey.Field(99.99))

	time.Sleep(50 * time.Millisecond)
}
