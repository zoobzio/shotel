package shotel

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestMetricTypeCounter(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	orderCreated := capitan.NewSignal("order.created", "Order Created")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal: orderCreated,
				Name:   "orders_created_total",
				Type:   MetricTypeCounter,
				// No ValueKey needed for counter
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	cpuUsage := capitan.NewSignal("system.cpu.usage", "System Cpu Usage")
	usageKey := capitan.NewInt64Key("percent")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   cpuUsage,
				Name:     "cpu_usage_percent",
				Type:     MetricTypeGauge,
				ValueKey: usageKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	temperature := capitan.NewSignal("system.temperature", "System Temperature")
	tempKey := capitan.NewFloat64Key("celsius")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   temperature,
				Name:     "temperature_celsius",
				Type:     MetricTypeGauge,
				ValueKey: tempKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit float gauge values
	cap.Emit(ctx, temperature, tempKey.Field(22.5))
	cap.Emit(ctx, temperature, tempKey.Field(23.8))

	time.Sleep(100 * time.Millisecond)
}

func TestMetricTypeHistogramDuration(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	requestCompleted := capitan.NewSignal("request.completed", "Request Completed")
	durationKey := capitan.NewDurationKey("duration")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:      requestCompleted,
				Name:        "request_duration_ms",
				Type:        MetricTypeHistogram,
				ValueKey:    durationKey,
				Description: "Request duration in milliseconds",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	messageReceived := capitan.NewSignal("message.received", "Message Received")
	sizeKey := capitan.NewInt64Key("size_bytes")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   messageReceived,
				Name:     "message_size_bytes",
				Type:     MetricTypeHistogram,
				ValueKey: sizeKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit size measurements
	cap.Emit(ctx, messageReceived, sizeKey.Field(1024))
	cap.Emit(ctx, messageReceived, sizeKey.Field(2048))
	cap.Emit(ctx, messageReceived, sizeKey.Field(512))

	time.Sleep(100 * time.Millisecond)
}

func TestMetricTypeUpDownCounterInt64(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	queueDepth := capitan.NewSignal("queue.depth.changed", "Queue Depth Changed")
	deltaKey := capitan.NewInt64Key("delta")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   queueDepth,
				Name:     "queue_depth",
				Type:     MetricTypeUpDownCounter,
				ValueKey: deltaKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	balanceChanged := capitan.NewSignal("balance.changed", "Balance Changed")
	amountKey := capitan.NewFloat64Key("amount")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   balanceChanged,
				Name:     "account_balance",
				Type:     MetricTypeUpDownCounter,
				ValueKey: amountKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit balance changes
	cap.Emit(ctx, balanceChanged, amountKey.Field(100.50))
	cap.Emit(ctx, balanceChanged, amountKey.Field(-25.75))
	cap.Emit(ctx, balanceChanged, amountKey.Field(50.00))

	time.Sleep(100 * time.Millisecond)
}

func TestMetricConfigValidation(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	tests := []struct {
		name    string
		config  MetricConfig
		wantErr bool
	}{
		{
			name: "counter without ValueKey (valid)",
			config: MetricConfig{
				Signal: capitan.NewSignal("test.signal", "Test Signal"),
				Name:   "test_counter",
				Type:   MetricTypeCounter,
			},
			wantErr: false,
		},
		{
			name: "gauge without ValueKey (invalid)",
			config: MetricConfig{
				Signal: capitan.NewSignal("test.signal", "Test Signal"),
				Name:   "test_gauge",
				Type:   MetricTypeGauge,
			},
			wantErr: true,
		},
		{
			name: "histogram with non-numeric ValueKey (invalid)",
			config: MetricConfig{
				Signal:   capitan.NewSignal("test.signal", "Test Signal"),
				Name:     "test_histogram",
				Type:     MetricTypeHistogram,
				ValueKey: capitan.NewStringKey("value"),
			},
			wantErr: true,
		},
		{
			name: "gauge with int64 ValueKey (valid)",
			config: MetricConfig{
				Signal:   capitan.NewSignal("test.signal", "Test Signal"),
				Name:     "test_gauge",
				Type:     MetricTypeGauge,
				ValueKey: capitan.NewInt64Key("value"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Metrics: []MetricConfig{tt.config},
			}

			_, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Define multiple signals with different metric types
	orderCreated := capitan.NewSignal("order.created", "Order Created")
	cpuUsage := capitan.NewSignal("cpu.usage", "Cpu Usage")
	requestCompleted := capitan.NewSignal("request.completed", "Request Completed")
	queueDepth := capitan.NewSignal("queue.depth", "Queue Depth")

	usageKey := capitan.NewFloat64Key("percent")
	durationKey := capitan.NewDurationKey("duration")
	deltaKey := capitan.NewInt64Key("delta")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal: orderCreated,
				Name:   "orders_total",
				Type:   MetricTypeCounter,
			},
			{
				Signal:   cpuUsage,
				Name:     "cpu_usage",
				Type:     MetricTypeGauge,
				ValueKey: usageKey,
			},
			{
				Signal:   requestCompleted,
				Name:     "request_duration_ms",
				Type:     MetricTypeHistogram,
				ValueKey: durationKey,
			},
			{
				Signal:   queueDepth,
				Name:     "queue_depth",
				Type:     MetricTypeUpDownCounter,
				ValueKey: deltaKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	testSignal := capitan.NewSignal("test.signal", "Test Signal")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal: testSignal,
				Name:   "test_metric",
				// Type not specified - should default to Counter
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	cap.Emit(ctx, testSignal)
	time.Sleep(100 * time.Millisecond)
}

func TestExtractNumericValue_AllIntegerTypes(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

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

	config := &Config{
		Metrics: []MetricConfig{
			{Signal: testSignal, Name: "int_gauge", Type: MetricTypeGauge, ValueKey: intKey},
			{Signal: testSignal, Name: "int32_gauge", Type: MetricTypeGauge, ValueKey: int32Key},
			{Signal: testSignal, Name: "uint_gauge", Type: MetricTypeGauge, ValueKey: uintKey},
			{Signal: testSignal, Name: "uint32_gauge", Type: MetricTypeGauge, ValueKey: uint32Key},
			{Signal: testSignal, Name: "uint64_gauge", Type: MetricTypeGauge, ValueKey: uint64Key},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	testSignal := capitan.NewSignal("float.test", "Float Test")

	float32Key := capitan.NewFloat32Key("float32_value")

	config := &Config{
		Metrics: []MetricConfig{
			{Signal: testSignal, Name: "float32_gauge", Type: MetricTypeGauge, ValueKey: float32Key},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	cap.Emit(ctx, testSignal, float32Key.Field(float32(3.14)))

	time.Sleep(100 * time.Millisecond)
	// Float32 should be extracted
}

func TestRecordHistogram_FloatVariant(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	testSignal := capitan.NewSignal("histogram.float.test", "Histogram Float Test")
	valueKey := capitan.NewFloat64Key("value")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   testSignal,
				Name:     "float_histogram",
				Type:     MetricTypeHistogram,
				ValueKey: valueKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	testSignal := capitan.NewSignal("missing.key.test", "Missing Key Test")
	valueKey := capitan.NewInt64Key("value")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   testSignal,
				Name:     "missing_key_gauge",
				Type:     MetricTypeGauge,
				ValueKey: valueKey,
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit without the expected key - should not panic
	cap.Emit(ctx, testSignal)

	time.Sleep(100 * time.Millisecond)
	// Missing key should be handled gracefully
}
