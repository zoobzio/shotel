package shotel

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

func TestRegisterTransformer(t *testing.T) {
	// Clean up after test
	defer func() {
		globalRegistry.mu.Lock()
		globalRegistry.transformers = make(map[capitan.Variant]func(capitan.Field) []log.KeyValue)
		globalRegistry.mu.Unlock()
	}()

	orderVariant := capitan.Variant("test.OrderInfo")
	orderKey := capitan.NewKey[OrderInfo]("order", orderVariant)

	// Register transformer
	RegisterTransformer(orderVariant, func(key string, order OrderInfo) []log.KeyValue {
		return []log.KeyValue{
			log.String(key+".id", order.ID),
			log.Float64(key+".total", order.Total),
			// Secret field omitted
		}
	})

	// Create field
	field := orderKey.Field(OrderInfo{
		ID:     "ORDER-123",
		Total:  99.99,
		Secret: "secret-data",
	})

	// Transform
	attrs := fieldsToAttributes([]capitan.Field{field})

	// Verify
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(attrs))
	}

	// Check attributes
	keys := make(map[string]bool)
	for _, attr := range attrs {
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

func TestUnregisterTransformer(t *testing.T) {
	// Clean up after test
	defer func() {
		globalRegistry.mu.Lock()
		globalRegistry.transformers = make(map[capitan.Variant]func(capitan.Field) []log.KeyValue)
		globalRegistry.mu.Unlock()
	}()

	variant := capitan.Variant("test.Custom")
	key := capitan.NewKey[string]("custom", variant)

	// Register
	RegisterTransformer(variant, func(k string, v string) []log.KeyValue {
		return []log.KeyValue{log.String(k, v)}
	})

	// Verify registered
	field := key.Field("test")
	attrs := fieldsToAttributes([]capitan.Field{field})
	if len(attrs) != 1 {
		t.Fatalf("expected 1 attribute after registration, got %d", len(attrs))
	}

	// Unregister
	UnregisterTransformer(variant)

	// Verify unregistered
	attrs = fieldsToAttributes([]capitan.Field{field})
	if len(attrs) != 0 {
		t.Fatalf("expected 0 attributes after unregistration, got %d", len(attrs))
	}
}

func TestCustomTransformerIntegration(t *testing.T) {
	// Clean up after test
	defer func() {
		globalRegistry.mu.Lock()
		globalRegistry.transformers = make(map[capitan.Variant]func(capitan.Field) []log.KeyValue)
		globalRegistry.mu.Unlock()
	}()

	ctx := context.Background()
	cap := capitan.New()

	// Register custom transformer before creating shotel
	orderVariant := capitan.Variant("test.OrderInfo")
	RegisterTransformer(orderVariant, func(key string, order OrderInfo) []log.KeyValue {
		return []log.KeyValue{
			log.String(key+".id", order.ID),
			log.Float64(key+".total", order.Total),
		}
	})

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
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
	// Clean up after test
	defer func() {
		globalRegistry.mu.Lock()
		globalRegistry.transformers = make(map[capitan.Variant]func(capitan.Field) []log.KeyValue)
		globalRegistry.mu.Unlock()
	}()

	variant := capitan.Variant("test.Skipped")
	key := capitan.NewKey[string]("skip", variant)

	// Register transformer that returns nil
	RegisterTransformer(variant, func(k string, v string) []log.KeyValue {
		return nil // Skip this field
	})

	field := key.Field("test")
	attrs := fieldsToAttributes([]capitan.Field{field})

	if len(attrs) != 0 {
		t.Errorf("expected 0 attributes when transformer returns nil, got %d", len(attrs))
	}
}

func TestMixedBuiltinAndCustomFields(t *testing.T) {
	// Clean up after test
	defer func() {
		globalRegistry.mu.Lock()
		globalRegistry.transformers = make(map[capitan.Variant]func(capitan.Field) []log.KeyValue)
		globalRegistry.mu.Unlock()
	}()

	variant := capitan.Variant("test.Custom")
	customKey := capitan.NewKey[int]("custom", variant)
	stringKey := capitan.NewStringKey("msg")

	// Register custom transformer
	RegisterTransformer(variant, func(k string, v int) []log.KeyValue {
		return []log.KeyValue{log.Int64(k+".custom", int64(v*2))}
	})

	fields := []capitan.Field{
		stringKey.Field("hello"),
		customKey.Field(21),
	}

	attrs := fieldsToAttributes(fields)

	// Should have 2 attributes: one from string, one from custom
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %d", len(attrs))
	}

	keys := make(map[string]bool)
	for _, attr := range attrs {
		keys[attr.Key] = true
	}

	if !keys["msg"] {
		t.Error("missing built-in string attribute")
	}
	if !keys["custom.custom"] {
		t.Error("missing custom attribute")
	}
}
