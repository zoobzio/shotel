package aperture

import (
	"context"
	"testing"
	"time"

	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

func TestConfigMetrics(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderFailed := capitan.NewSignal("order.failed", "Order failed")

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
		t.Fatalf("failed to create Aperture: %v", err)
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

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderFailed := capitan.NewSignal("order.failed", "Order failed")
	orderCanceled := capitan.NewSignal("order.canceled", "Order canceled")

	// Configure log whitelist - only log created and failed
	config := &Config{
		Logs: &LogConfig{
			Whitelist: []capitan.Signal{orderCreated, orderFailed},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
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

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	requestStarted := capitan.NewSignal("request.started", "Request started")
	requestCompleted := capitan.NewSignal("request.completed", "Request completed")
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
		t.Fatalf("failed to create Aperture: %v", err)
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

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define signals
	orderCreated := capitan.NewSignal("order.created", "Order created")
	orderCompleted := capitan.NewSignal("order.completed", "Order completed")
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
		t.Fatalf("failed to create Aperture: %v", err)
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

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Empty config - should behave like nil (all events logged)
	config := &Config{}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit event
	testSig := capitan.NewSignal("test.event", "Test event")
	cap.Emit(ctx, testSig)

	time.Sleep(100 * time.Millisecond)

	// Should log all events (default behavior)
}

func TestMakeTransformer_Success(t *testing.T) {
	// Custom type for testing
	type OrderInfo struct {
		ID    string
		Total float64
	}

	// Create transformer
	transformer := MakeTransformer(func(key string, order OrderInfo) []log.KeyValue {
		return []log.KeyValue{
			log.String(key+".id", order.ID),
			log.Float64(key+".total", order.Total),
		}
	})

	// Create a field with the custom type
	orderKey := capitan.NewKey[OrderInfo]("order", "test.OrderInfo")
	field := orderKey.Field(OrderInfo{ID: "ORD-123", Total: 99.99})

	// Transform the field
	result := transformer(field)

	if len(result) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(result))
	}

	// Verify ID attribute
	if result[0].Key != "order.id" {
		t.Errorf("expected key 'order.id', got %q", result[0].Key)
	}
	if result[0].Value.AsString() != "ORD-123" {
		t.Errorf("expected value 'ORD-123', got %q", result[0].Value.AsString())
	}

	// Verify Total attribute
	if result[1].Key != "order.total" {
		t.Errorf("expected key 'order.total', got %q", result[1].Key)
	}
	if result[1].Value.AsFloat64() != 99.99 {
		t.Errorf("expected value 99.99, got %v", result[1].Value.AsFloat64())
	}
}

func TestMakeTransformer_WrongType(t *testing.T) {
	// Transformer expects OrderInfo
	type OrderInfo struct {
		ID string
	}

	transformer := MakeTransformer(func(key string, order OrderInfo) []log.KeyValue {
		return []log.KeyValue{
			log.String(key+".id", order.ID),
		}
	})

	// Pass a field with a different type (string)
	stringKey := capitan.NewStringKey("wrong_type")
	field := stringKey.Field("not an order")

	// Transform should return nil for wrong type
	result := transformer(field)

	if result != nil {
		t.Errorf("expected nil for wrong type, got %v", result)
	}
}

func TestMakeTransformer_EmptyResult(t *testing.T) {
	// Transformer that returns empty slice
	type EmptyType struct{}

	transformer := MakeTransformer(func(key string, _ EmptyType) []log.KeyValue {
		return []log.KeyValue{}
	})

	emptyKey := capitan.NewKey[EmptyType]("empty", "test.EmptyType")
	field := emptyKey.Field(EmptyType{})

	result := transformer(field)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d attributes", len(result))
	}
}

func TestMakeTransformer_NilResult(t *testing.T) {
	// Transformer that returns nil
	type NilType struct{}

	transformer := MakeTransformer(func(key string, _ NilType) []log.KeyValue {
		return nil
	})

	nilKey := capitan.NewKey[NilType]("nil", "test.NilType")
	field := nilKey.Field(NilType{})

	result := transformer(field)

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}
