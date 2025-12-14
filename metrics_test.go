package aperture

import (
	"context"
	"sync"
	"testing"
	"time"

	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
)

func TestMetricTypeCounter(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	orderCreated := capitan.NewSignal("order.created", "Order Created")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal: "order.created",
				Name:   "orders_created_total",
				Type:   "counter",
				// No ValueKey needed for counter
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

	// Emit events - counter should increment
	cap.Emit(ctx, orderCreated)
	cap.Emit(ctx, orderCreated)
	cap.Emit(ctx, orderCreated)

	time.Sleep(100 * time.Millisecond)
	// Counter should be 3 (validation would require OTLP capture)
}

func TestMetricTypeGaugeInt64(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	cpuUsage := capitan.NewSignal("system.cpu.usage", "System Cpu Usage")
	usageKey := capitan.NewInt64Key("percent")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "system.cpu.usage",
				Name:     "cpu_usage_percent",
				Type:     "gauge",
				ValueKey: "percent",
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

	// Emit gauge values
	cap.Emit(ctx, cpuUsage, usageKey.Field(45))
	cap.Emit(ctx, cpuUsage, usageKey.Field(52))
	cap.Emit(ctx, cpuUsage, usageKey.Field(38))

	time.Sleep(100 * time.Millisecond)
	// Gauge should be set to 38 (last value)
}

func TestMetricTypeGaugeFloat64(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	temperature := capitan.NewSignal("system.temperature", "System Temperature")
	tempKey := capitan.NewFloat64Key("celsius")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "system.temperature",
				Name:     "temperature_celsius",
				Type:     "gauge",
				ValueKey: "celsius",
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

	// Emit float gauge values
	cap.Emit(ctx, temperature, tempKey.Field(22.5))
	cap.Emit(ctx, temperature, tempKey.Field(23.8))

	time.Sleep(100 * time.Millisecond)
}

func TestMetricTypeHistogramDuration(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	requestCompleted := capitan.NewSignal("request.completed", "Request Completed")
	durationKey := capitan.NewDurationKey("duration")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:      "request.completed",
				Name:        "request_duration_ms",
				Type:        "histogram",
				ValueKey:    "duration",
				Description: "Request duration in milliseconds",
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

	// Emit duration measurements
	cap.Emit(ctx, requestCompleted, durationKey.Field(150*time.Millisecond))
	cap.Emit(ctx, requestCompleted, durationKey.Field(200*time.Millisecond))
	cap.Emit(ctx, requestCompleted, durationKey.Field(180*time.Millisecond))

	time.Sleep(100 * time.Millisecond)
	// Histogram should contain distribution of durations
}

func TestMetricTypeHistogramInt64(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	messageReceived := capitan.NewSignal("message.received", "Message Received")
	sizeKey := capitan.NewInt64Key("size_bytes")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "message.received",
				Name:     "message_size_bytes",
				Type:     "histogram",
				ValueKey: "size_bytes",
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

	// Emit size measurements
	cap.Emit(ctx, messageReceived, sizeKey.Field(1024))
	cap.Emit(ctx, messageReceived, sizeKey.Field(2048))
	cap.Emit(ctx, messageReceived, sizeKey.Field(512))

	time.Sleep(100 * time.Millisecond)
}

func TestMetricTypeUpDownCounterInt64(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	queueDepth := capitan.NewSignal("queue.depth.changed", "Queue Depth Changed")
	deltaKey := capitan.NewInt64Key("delta")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "queue.depth.changed",
				Name:     "queue_depth",
				Type:     "updowncounter",
				ValueKey: "delta",
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

	// Emit deltas (positive and negative)
	cap.Emit(ctx, queueDepth, deltaKey.Field(5))  // +5
	cap.Emit(ctx, queueDepth, deltaKey.Field(3))  // +3
	cap.Emit(ctx, queueDepth, deltaKey.Field(-2)) // -2

	time.Sleep(100 * time.Millisecond)
	// UpDownCounter should be 6
}

func TestMetricTypeUpDownCounterFloat64(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	balanceChanged := capitan.NewSignal("balance.changed", "Balance Changed")
	amountKey := capitan.NewFloat64Key("amount")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "balance.changed",
				Name:     "account_balance",
				Type:     "updowncounter",
				ValueKey: "amount",
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

	// Emit balance changes
	cap.Emit(ctx, balanceChanged, amountKey.Field(100.50))
	cap.Emit(ctx, balanceChanged, amountKey.Field(-25.75))
	cap.Emit(ctx, balanceChanged, amountKey.Field(50.00))

	time.Sleep(100 * time.Millisecond)
}

func TestMetricSchemaValidation(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	tests := []struct {
		name    string
		schema  MetricSchema
		wantErr bool
	}{
		{
			name: "counter without ValueKey (valid)",
			schema: MetricSchema{
				Signal: "test.signal",
				Name:   "test_counter",
				Type:   "counter",
			},
			wantErr: false,
		},
		{
			name: "gauge without ValueKey (invalid)",
			schema: MetricSchema{
				Signal: "test.signal",
				Name:   "test_gauge",
				Type:   "gauge",
			},
			wantErr: true,
		},
		{
			name: "gauge with ValueKey (valid)",
			schema: MetricSchema{
				Signal:   "test.signal",
				Name:     "test_gauge",
				Type:     "gauge",
				ValueKey: "value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
			if err != nil {
				t.Fatalf("failed to create Aperture: %v", err)
			}
			defer sh.Close()

			schema := Schema{
				Metrics: []MetricSchema{tt.schema},
			}

			err = sh.Apply(schema)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMixedMetricTypes(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Define multiple signals with different metric types
	orderCreated := capitan.NewSignal("order.created", "Order Created")
	cpuUsage := capitan.NewSignal("cpu.usage", "Cpu Usage")
	requestCompleted := capitan.NewSignal("request.completed", "Request Completed")
	queueDepth := capitan.NewSignal("queue.depth", "Queue Depth")

	usageKey := capitan.NewFloat64Key("percent")
	durationKey := capitan.NewDurationKey("duration")
	deltaKey := capitan.NewInt64Key("delta")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal: "order.created",
				Name:   "orders_total",
				Type:   "counter",
			},
			{
				Signal:   "cpu.usage",
				Name:     "cpu_usage",
				Type:     "gauge",
				ValueKey: "percent",
			},
			{
				Signal:   "request.completed",
				Name:     "request_duration_ms",
				Type:     "histogram",
				ValueKey: "duration",
			},
			{
				Signal:   "queue.depth",
				Name:     "queue_depth",
				Type:     "updowncounter",
				ValueKey: "delta",
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

	// Emit various events
	cap.Emit(ctx, orderCreated)
	cap.Emit(ctx, cpuUsage, usageKey.Field(45.5))
	cap.Emit(ctx, requestCompleted, durationKey.Field(250*time.Millisecond))
	cap.Emit(ctx, queueDepth, deltaKey.Field(10))

	time.Sleep(100 * time.Millisecond)
	// All metric types should be recorded
}

func TestDefaultMetricTypeIsCounter(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	testSignal := capitan.NewSignal("test.signal", "Test Signal")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal: "test.signal",
				Name:   "test_metric",
				// Type not specified - should default to Counter
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

	cap.Emit(ctx, testSignal)
	time.Sleep(100 * time.Millisecond)
}

func TestExtractNumericValue_AllIntegerTypes(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	testSignal := capitan.NewSignal("numeric.test", "Numeric Test")

	// Test int
	intKey := capitan.NewIntKey("int_value")
	// Test int32
	int32Key := capitan.NewInt32Key("int32_value")
	// Test uint
	uintKey := capitan.NewUintKey("uint_value")
	// Test uint32
	uint32Key := capitan.NewUint32Key("uint32_value")
	// Test uint64
	uint64Key := capitan.NewUint64Key("uint64_value")

	schema := Schema{
		Metrics: []MetricSchema{
			{Signal: "numeric.test", Name: "int_gauge", Type: "gauge", ValueKey: "int_value"},
			{Signal: "numeric.test", Name: "int32_gauge", Type: "gauge", ValueKey: "int32_value"},
			{Signal: "numeric.test", Name: "uint_gauge", Type: "gauge", ValueKey: "uint_value"},
			{Signal: "numeric.test", Name: "uint32_gauge", Type: "gauge", ValueKey: "uint32_value"},
			{Signal: "numeric.test", Name: "uint64_gauge", Type: "gauge", ValueKey: "uint64_value"},
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

	// Emit with all integer types
	cap.Emit(ctx, testSignal,
		intKey.Field(42),
		int32Key.Field(int32(100)),
		uintKey.Field(uint(200)),
		uint32Key.Field(uint32(300)),
		uint64Key.Field(uint64(400)),
	)

	time.Sleep(100 * time.Millisecond)
	// All integer variants should be extracted and recorded
}

func TestExtractNumericValue_AllFloatTypes(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	testSignal := capitan.NewSignal("float.test", "Float Test")

	float32Key := capitan.NewFloat32Key("float32_value")

	schema := Schema{
		Metrics: []MetricSchema{
			{Signal: "float.test", Name: "float32_gauge", Type: "gauge", ValueKey: "float32_value"},
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

	cap.Emit(ctx, testSignal, float32Key.Field(float32(3.14)))

	time.Sleep(100 * time.Millisecond)
	// Float32 should be extracted
}

func TestRecordHistogram_FloatVariant(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	testSignal := capitan.NewSignal("histogram.float.test", "Histogram Float Test")
	valueKey := capitan.NewFloat64Key("value")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "histogram.float.test",
				Name:     "float_histogram",
				Type:     "histogram",
				ValueKey: "value",
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

	// Emit multiple values to exercise histogram recording
	cap.Emit(ctx, testSignal, valueKey.Field(1.5))
	cap.Emit(ctx, testSignal, valueKey.Field(2.5))
	cap.Emit(ctx, testSignal, valueKey.Field(3.5))

	time.Sleep(100 * time.Millisecond)
	// Float64 histogram should record all values
}

func TestExtractNumericValue_MissingKey(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	testSignal := capitan.NewSignal("missing.key.test", "Missing Key Test")

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "missing.key.test",
				Name:     "missing_key_gauge",
				Type:     "gauge",
				ValueKey: "value",
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

	// Emit without the expected key - should not panic
	cap.Emit(ctx, testSignal)

	time.Sleep(100 * time.Millisecond)
	// Missing key should be handled gracefully
}

func TestMetricsHandler_NilHandler(t *testing.T) {
	ctx := context.Background()

	// Create nil handler
	var mh *metricsHandler

	// Should not panic
	mh.handleEvent(ctx, nil, nil)
}

func TestValidateMetricConfig_EmptyName(t *testing.T) {
	config := metricConfig{
		SignalName: "test.signal",
		Name:       "", // Empty name
		Type:       MetricTypeCounter,
	}

	err := validateMetricConfig(config)
	if err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestNumericValue_Conversions(t *testing.T) {
	// Test int to float conversion
	intVal := &numericValue{intValue: 42, isFloat: false}
	if intVal.asInt64() != 42 {
		t.Errorf("asInt64() = %v, want 42", intVal.asInt64())
	}
	if intVal.asFloat64() != 42.0 {
		t.Errorf("asFloat64() = %v, want 42.0", intVal.asFloat64())
	}

	// Test float to int conversion
	floatVal := &numericValue{floatValue: 3.14, isFloat: true}
	if floatVal.asInt64() != 3 {
		t.Errorf("asInt64() = %v, want 3", floatVal.asInt64())
	}
	if floatVal.asFloat64() != 3.14 {
		t.Errorf("asFloat64() = %v, want 3.14", floatVal.asFloat64())
	}
}

func TestExtractNumericValue_NilKeyName(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	testSignal := capitan.NewSignal("test.signal", "Test Signal")
	intKey := capitan.NewInt64Key("value")

	// Capture event via a gauge metric handler
	var capturedEvent *capitan.Event
	var mu sync.Mutex
	cap.Hook(testSignal, func(ctx context.Context, e *capitan.Event) {
		mu.Lock()
		capturedEvent = e
		mu.Unlock()
	})

	// Create aperture to ensure hooks work
	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit event
	cap.Emit(ctx, testSignal, intKey.Field(42))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	evt := capturedEvent
	mu.Unlock()

	if evt == nil {
		t.Fatal("event was not captured")
	}

	// Extract with empty key name
	result := extractNumericValueByName(evt, "")
	if result != nil {
		t.Error("expected nil for empty key name")
	}
}

func TestInvalidMetricType(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	// Test with unknown metric type - schema with string type gets parsed
	// and unknown types default to counter, so no error
	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "test.signal",
				Name:     "test_metric",
				Type:     "unknown_type",
				ValueKey: "value",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Unknown type defaults to counter, which doesn't require ValueKey validation
	err = sh.Apply(schema)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
