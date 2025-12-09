package aperture

import (
	"testing"

	"github.com/zoobzio/capitan"

	"go.opentelemetry.io/otel/log"
)

func TestRegistrySpec_Empty(t *testing.T) {
	r := NewRegistry()
	spec := r.Spec()

	if len(spec.Signals) != 0 {
		t.Errorf("expected 0 signals, got %d", len(spec.Signals))
	}
	if len(spec.Keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(spec.Keys))
	}
	if len(spec.ContextKeys) != 0 {
		t.Errorf("expected 0 context keys, got %d", len(spec.ContextKeys))
	}
	if len(spec.Transformers) != 0 {
		t.Errorf("expected 0 transformers, got %d", len(spec.Transformers))
	}
}

func TestRegistrySpec_Signals(t *testing.T) {
	r := NewRegistry()
	sig1 := capitan.NewSignal("test.signal.one", "First test signal")
	sig2 := capitan.NewSignal("test.signal.two", "Second test signal")
	r.Register(sig1, sig2)

	spec := r.Spec()

	if len(spec.Signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(spec.Signals))
	}

	names := make(map[string]string)
	for _, s := range spec.Signals {
		names[s.Name] = s.Description
	}

	if desc, ok := names["test.signal.one"]; !ok || desc != "First test signal" {
		t.Errorf("signal one mismatch: got %q", desc)
	}
	if desc, ok := names["test.signal.two"]; !ok || desc != "Second test signal" {
		t.Errorf("signal two mismatch: got %q", desc)
	}
}

func TestRegistrySpec_Keys(t *testing.T) {
	r := NewRegistry()
	strKey := capitan.NewStringKey("str")
	intKey := capitan.NewIntKey("num")
	durKey := capitan.NewDurationKey("dur")
	r.RegisterKey(strKey, intKey, durKey)

	spec := r.Spec()

	if len(spec.Keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(spec.Keys))
	}

	keys := make(map[string]string)
	for _, k := range spec.Keys {
		keys[k.Name] = k.Variant
	}

	if v := keys["str"]; v != "string" {
		t.Errorf("expected string variant, got %q", v)
	}
	if v := keys["num"]; v != "int" {
		t.Errorf("expected int variant, got %q", v)
	}
	if v := keys["dur"]; v != "duration" {
		t.Errorf("expected duration variant, got %q", v)
	}
}

func TestRegistrySpec_ContextKeys(t *testing.T) {
	r := NewRegistry()
	type ctxKey1 struct{}
	type ctxKey2 struct{}
	r.RegisterContextKey("request_id", ctxKey1{})
	r.RegisterContextKey("user_id", ctxKey2{})

	spec := r.Spec()

	if len(spec.ContextKeys) != 2 {
		t.Fatalf("expected 2 context keys, got %d", len(spec.ContextKeys))
	}

	keys := make(map[string]bool)
	for _, k := range spec.ContextKeys {
		keys[k] = true
	}

	if !keys["request_id"] {
		t.Error("missing request_id context key")
	}
	if !keys["user_id"] {
		t.Error("missing user_id context key")
	}
}

func TestRegistrySpec_Transformers(t *testing.T) {
	r := NewRegistry()
	r.RegisterTransformer(capitan.VariantString, func(f capitan.Field) []log.KeyValue {
		return nil
	})

	spec := r.Spec()

	if len(spec.Transformers) != 1 {
		t.Fatalf("expected 1 transformer, got %d", len(spec.Transformers))
	}

	if spec.Transformers[0].Variant != "string" {
		t.Errorf("expected string variant, got %q", spec.Transformers[0].Variant)
	}
}

func TestRegistrySpec_Full(t *testing.T) {
	r := NewRegistry()

	sig := capitan.NewSignal("test.signal", "Test signal")
	r.Register(sig)

	strKey := capitan.NewStringKey("id")
	intKey := capitan.NewIntKey("count")
	r.RegisterKey(strKey, intKey)

	type ctxKey struct{}
	r.RegisterContextKey("request_id", ctxKey{})

	customVariant := capitan.Variant("test.Custom")
	r.RegisterTransformer(customVariant, func(f capitan.Field) []log.KeyValue {
		return nil
	})

	spec := r.Spec()

	if len(spec.Signals) != 1 {
		t.Errorf("expected 1 signal, got %d", len(spec.Signals))
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

func TestVariantName_BuiltinTypes(t *testing.T) {
	tests := []struct {
		variant capitan.Variant
		want    string
	}{
		{capitan.VariantString, "string"},
		{capitan.VariantInt, "int"},
		{capitan.VariantInt32, "int32"},
		{capitan.VariantInt64, "int64"},
		{capitan.VariantUint, "uint"},
		{capitan.VariantUint32, "uint32"},
		{capitan.VariantUint64, "uint64"},
		{capitan.VariantFloat32, "float32"},
		{capitan.VariantFloat64, "float64"},
		{capitan.VariantBool, "bool"},
		{capitan.VariantTime, "time"},
		{capitan.VariantDuration, "duration"},
		{capitan.VariantBytes, "bytes"},
		{capitan.VariantError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := variantName(tt.variant)
			if got != tt.want {
				t.Errorf("variantName(%v) = %q, want %q", tt.variant, got, tt.want)
			}
		})
	}
}

func TestVariantName_CustomType(t *testing.T) {
	custom := capitan.Variant("mypackage.CustomType")
	got := variantName(custom)
	if got != "mypackage.CustomType" {
		t.Errorf("variantName(%v) = %q, want %q", custom, got, "mypackage.CustomType")
	}
}
