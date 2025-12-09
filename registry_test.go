package aperture

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/capitan"

	"go.opentelemetry.io/otel/log"
)

type OrderInfo struct {
	ID     string
	Total  float64
	Secret string // Should not be logged
}

func TestMakeTransformer(t *testing.T) {
	orderVariant := capitan.Variant("test.OrderInfo")
	orderKey := capitan.NewKey[OrderInfo]("order", orderVariant)

	// Create transformer using MakeTransformer
	transformers := map[capitan.Variant]FieldTransformer{
		orderVariant: MakeTransformer(func(key string, order OrderInfo) []log.KeyValue {
			return []log.KeyValue{
				log.String(key+".id", order.ID),
				log.Float64(key+".total", order.Total),
				// Secret field omitted
			}
		}),
	}

	// Create field
	field := orderKey.Field(OrderInfo{
		ID:     "ORDER-123",
		Total:  99.99,
		Secret: "secret-data",
	})

	// Transform
	result := fieldsToAttributes([]capitan.Field{field}, transformers)

	// Verify
	if len(result.attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(result.attrs))
	}

	// Check attributes
	keys := make(map[string]bool)
	for _, attr := range result.attrs {
		keys[attr.Key] = true
	}

	if !keys["order.id"] {
		t.Error("missing order.id attribute")
	}
	if !keys["order.total"] {
		t.Error("missing order.total attribute")
	}
	if keys["order.secret"] {
		t.Error("secret field should not be present")
	}
}

func TestTransformer_NotRegistered(t *testing.T) {
	variant := capitan.Variant("test.Custom")
	key := capitan.NewKey[string]("custom", variant)

	// No transformers registered
	transformers := map[capitan.Variant]FieldTransformer{}

	field := key.Field("test")
	result := fieldsToAttributes([]capitan.Field{field}, transformers)

	// Should be skipped
	if len(result.attrs) != 0 {
		t.Fatalf("expected 0 attributes for unregistered type, got %d", len(result.attrs))
	}

	// Should be recorded as skipped
	if len(result.skipped) != 1 {
		t.Fatalf("expected 1 skipped field, got %d", len(result.skipped))
	}
}

func TestCustomTransformerIntegration(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	orderVariant := capitan.Variant("test.OrderInfo")

	// Configure transformers in config
	config := &Config{
		Transformers: map[capitan.Variant]FieldTransformer{
			orderVariant: MakeTransformer(func(key string, order OrderInfo) []log.KeyValue {
				return []log.KeyValue{
					log.String(key+".id", order.ID),
					log.Float64(key+".total", order.Total),
				}
			}),
		},
	}

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Define signal and custom key
	testSig := capitan.NewSignal("test.order.created", "Test Order Created")
	orderKey := capitan.NewKey[OrderInfo]("order", orderVariant)

	// Emit event with custom type
	cap.Emit(ctx, testSig, orderKey.Field(OrderInfo{
		ID:     "ORDER-456",
		Total:  199.99,
		Secret: "should-not-appear",
	}))

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// If we got here without panics, the integration is working
}

func TestTransformerReturnsNil(t *testing.T) {
	variant := capitan.Variant("test.Skipped")
	key := capitan.NewKey[string]("skip", variant)

	// Transformer that returns nil
	transformers := map[capitan.Variant]FieldTransformer{
		variant: MakeTransformer(func(k string, v string) []log.KeyValue {
			return nil // Skip this field
		}),
	}

	field := key.Field("test")
	result := fieldsToAttributes([]capitan.Field{field}, transformers)

	if len(result.attrs) != 0 {
		t.Errorf("expected 0 attributes when transformer returns nil, got %d", len(result.attrs))
	}
}

func TestMixedBuiltinAndCustomFields(t *testing.T) {
	variant := capitan.Variant("test.Custom")
	customKey := capitan.NewKey[int]("custom", variant)
	stringKey := capitan.NewStringKey("msg")

	// Custom transformer
	transformers := map[capitan.Variant]FieldTransformer{
		variant: MakeTransformer(func(k string, v int) []log.KeyValue {
			return []log.KeyValue{log.Int64(k+".custom", int64(v*2))}
		}),
	}

	fields := []capitan.Field{
		stringKey.Field("hello"),
		customKey.Field(21),
	}

	result := fieldsToAttributes(fields, transformers)

	// Should have 2 attributes: one from string, one from custom
	if len(result.attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(result.attrs))
	}

	keys := make(map[string]bool)
	for _, attr := range result.attrs {
		keys[attr.Key] = true
	}

	if !keys["msg"] {
		t.Error("missing built-in string attribute")
	}
	if !keys["custom.custom"] {
		t.Error("missing custom attribute")
	}
}

func TestNilTransformersMap(t *testing.T) {
	variant := capitan.Variant("test.Custom")
	key := capitan.NewKey[string]("custom", variant)

	field := key.Field("test")

	// Nil transformers map should not panic
	result := fieldsToAttributes([]capitan.Field{field}, nil)

	if len(result.attrs) != 0 {
		t.Fatalf("expected 0 attributes with nil transformers, got %d", len(result.attrs))
	}
}

// Registry and Schema tests

var (
	testSignalStart = capitan.NewSignal("TestStart", "test start signal")
	testSignalEnd   = capitan.NewSignal("TestEnd", "test end signal")
	testKeyID       = capitan.NewStringKey("id")
	testKeyDuration = capitan.NewDurationKey("duration")
	testKeyCount    = capitan.NewIntKey("count")
)

type testContextKey struct{}

func TestRegistryRegisterSignals(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)

	spec := r.Spec()
	if len(spec.Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(spec.Signals))
	}
}

func TestRegistryRegisterKeys(t *testing.T) {
	r := NewRegistry()
	r.RegisterKey(testKeyID, testKeyDuration, testKeyCount)

	spec := r.Spec()
	if len(spec.Keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(spec.Keys))
	}
}

func TestRegistryRegisterContextKey(t *testing.T) {
	r := NewRegistry()
	r.RegisterContextKey("request_id", testContextKey{})

	spec := r.Spec()
	if len(spec.ContextKeys) != 1 {
		t.Errorf("expected 1 context key, got %d", len(spec.ContextKeys))
	}
}

func TestRegistryBuildMetricCounter(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart)

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:      "TestStart",
				Name:        "test.started",
				Type:        "counter",
				Description: "test counter",
			},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(config.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(config.Metrics))
	}

	m := config.Metrics[0]
	if m.Signal != testSignalStart {
		t.Errorf("signal mismatch")
	}
	if m.Name != "test.started" {
		t.Errorf("name mismatch: got %s", m.Name)
	}
	if m.Type != MetricTypeCounter {
		t.Errorf("type mismatch: got %s", m.Type)
	}
}

func TestRegistryBuildMetricGauge(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart)
	r.RegisterKey(testKeyCount)

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "TestStart",
				Name:     "test.count",
				Type:     "gauge",
				ValueKey: "count",
			},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	m := config.Metrics[0]
	if m.Type != MetricTypeGauge {
		t.Errorf("type mismatch: got %s", m.Type)
	}
	if m.ValueKey != testKeyCount {
		t.Errorf("value key mismatch")
	}
}

func TestRegistryBuildMetricHistogram(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalEnd)
	r.RegisterKey(testKeyDuration)

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "TestEnd",
				Name:     "test.duration",
				Type:     "histogram",
				ValueKey: "duration",
			},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if config.Metrics[0].Type != MetricTypeHistogram {
		t.Errorf("type mismatch: got %s", config.Metrics[0].Type)
	}
}

func TestRegistryBuildMetricUpDownCounter(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart)
	r.RegisterKey(testKeyCount)

	schema := Schema{
		Metrics: []MetricSchema{
			{
				Signal:   "TestStart",
				Name:     "test.active",
				Type:     "updowncounter",
				ValueKey: "count",
			},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if config.Metrics[0].Type != MetricTypeUpDownCounter {
		t.Errorf("type mismatch: got %s", config.Metrics[0].Type)
	}
}

func TestRegistryBuildTrace(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)
	r.RegisterKey(testKeyID)

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "TestStart",
				End:            "TestEnd",
				CorrelationKey: "id",
				SpanName:       "test-operation",
				SpanTimeout:    "10m",
			},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(config.Traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(config.Traces))
	}

	tr := config.Traces[0]
	if tr.Start != testSignalStart {
		t.Errorf("start signal mismatch")
	}
	if tr.End != testSignalEnd {
		t.Errorf("end signal mismatch")
	}
	if tr.SpanName != "test-operation" {
		t.Errorf("span name mismatch: got %s", tr.SpanName)
	}
	if tr.SpanTimeout != 10*time.Minute {
		t.Errorf("span timeout mismatch: got %v", tr.SpanTimeout)
	}
}

func TestRegistryBuildLogs(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)

	schema := Schema{
		Logs: &LogSchema{
			Whitelist: []string{"TestStart"},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if config.Logs == nil {
		t.Fatal("logs config is nil")
	}
	if len(config.Logs.Whitelist) != 1 {
		t.Errorf("expected 1 whitelist entry, got %d", len(config.Logs.Whitelist))
	}
	if config.Logs.Whitelist[0] != testSignalStart {
		t.Errorf("whitelist signal mismatch")
	}
}

func TestRegistryBuildContext(t *testing.T) {
	r := NewRegistry()
	r.RegisterContextKey("request_id", testContextKey{})

	schema := Schema{
		Context: &ContextSchema{
			Logs:    []string{"request_id"},
			Metrics: []string{"request_id"},
			Traces:  []string{"request_id"},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if config.ContextExtraction == nil {
		t.Fatal("context extraction is nil")
	}
	if len(config.ContextExtraction.Logs) != 1 {
		t.Errorf("expected 1 log context key, got %d", len(config.ContextExtraction.Logs))
	}
	if len(config.ContextExtraction.Metrics) != 1 {
		t.Errorf("expected 1 metric context key, got %d", len(config.ContextExtraction.Metrics))
	}
	if len(config.ContextExtraction.Traces) != 1 {
		t.Errorf("expected 1 trace context key, got %d", len(config.ContextExtraction.Traces))
	}
}

func TestRegistryBuildStdout(t *testing.T) {
	r := NewRegistry()

	schema := Schema{
		Stdout: true,
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !config.StdoutLogging {
		t.Error("stdout logging should be enabled")
	}
}

func TestRegistryBuildErrorUnknownSignal(t *testing.T) {
	r := NewRegistry()

	schema := Schema{
		Metrics: []MetricSchema{
			{Signal: "Unknown", Name: "test"},
		},
	}

	_, err := r.Build(schema)
	if err == nil {
		t.Fatal("expected error for unknown signal")
	}
}

func TestRegistryBuildErrorUnknownKey(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart)

	schema := Schema{
		Metrics: []MetricSchema{
			{Signal: "TestStart", Name: "test", Type: "gauge", ValueKey: "unknown"},
		},
	}

	_, err := r.Build(schema)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestRegistryBuildErrorUnknownContextKey(t *testing.T) {
	r := NewRegistry()

	schema := Schema{
		Context: &ContextSchema{
			Logs: []string{"unknown"},
		},
	}

	_, err := r.Build(schema)
	if err == nil {
		t.Fatal("expected error for unknown context key")
	}
}

func TestRegistryBuildErrorInvalidTimeout(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)
	r.RegisterKey(testKeyID)

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "TestStart",
				End:            "TestEnd",
				CorrelationKey: "id",
				SpanTimeout:    "invalid",
			},
		},
	}

	_, err := r.Build(schema)
	if err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestRegistryBuildErrorNonStringCorrelationKey(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)
	r.RegisterKey(testKeyCount) // int key, not string

	schema := Schema{
		Traces: []TraceSchema{
			{
				Start:          "TestStart",
				End:            "TestEnd",
				CorrelationKey: "count",
			},
		},
	}

	_, err := r.Build(schema)
	if err == nil {
		t.Fatal("expected error for non-string correlation key")
	}
}

func TestRegistrySpecFull(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)
	r.RegisterKey(testKeyID, testKeyCount)
	r.RegisterContextKey("request_id", testContextKey{})
	r.RegisterTransformer("custom", func(f capitan.Field) []log.KeyValue { return nil })

	spec := r.Spec()

	if len(spec.Signals) != 2 {
		t.Errorf("expected 2 signals, got %d", len(spec.Signals))
	}
	if len(spec.Keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(spec.Keys))
	}
	if len(spec.ContextKeys) != 1 {
		t.Errorf("expected 1 context key, got %d", len(spec.ContextKeys))
	}
	if len(spec.Transformers) != 1 {
		t.Errorf("expected 1 transformer, got %d", len(spec.Transformers))
	}
}

func TestRegistryDefaultMetricType(t *testing.T) {
	r := NewRegistry()
	r.Register(testSignalStart)

	schema := Schema{
		Metrics: []MetricSchema{
			{Signal: "TestStart", Name: "test"},
		},
	}

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if config.Metrics[0].Type != MetricTypeCounter {
		t.Errorf("expected default counter type, got %s", config.Metrics[0].Type)
	}
}

func TestYAMLToConfigIntegration(t *testing.T) {
	yaml := `
metrics:
  - signal: TestStart
    name: test.started
    type: counter

traces:
  - start: TestStart
    end: TestEnd
    correlation_key: id
    span_timeout: 10m
`

	// Parse YAML
	schema, err := LoadSchemaFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadSchemaFromYAML failed: %v", err)
	}

	// Validate
	if err := schema.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	// Build config
	r := NewRegistry()
	r.Register(testSignalStart, testSignalEnd)
	r.RegisterKey(testKeyID)

	config, err := r.Build(schema)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify
	if len(config.Metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(config.Metrics))
	}
	if config.Metrics[0].Signal != testSignalStart {
		t.Error("metric signal mismatch")
	}

	if len(config.Traces) != 1 {
		t.Errorf("expected 1 trace, got %d", len(config.Traces))
	}
	if config.Traces[0].SpanTimeout != 10*time.Minute {
		t.Errorf("expected 10m timeout, got %v", config.Traces[0].SpanTimeout)
	}
}
