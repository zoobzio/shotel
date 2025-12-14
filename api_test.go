package aperture

import (
	"context"
	"strings"
	"testing"
	"time"

	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Valid configuration
	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sh == nil {
		t.Fatal("expected non-nil Aperture instance")
	}
	if sh.logProvider == nil {
		t.Error("logProvider not initialized")
	}
	if sh.capitanObserver == nil {
		t.Error("capitanObserver not initialized")
	}
	sh.Close()
}

func TestNew_NilCapitan(t *testing.T) {
	ctx := context.Background()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	_, err = New(nil, pvs.Log, pvs.Meter, pvs.Trace)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "capitan instance is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_NilLogProvider(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	_, err = New(cap, nil, pvs.Meter, pvs.Trace)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "log provider is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_NilMeterProvider(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	_, err = New(cap, pvs.Log, nil, pvs.Trace)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "meter provider is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_NilTraceProvider(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	_, err = New(cap, pvs.Log, pvs.Meter, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "trace provider is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestApertureInterfaces(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Test Logger
	logger := sh.Logger("test-scope")
	if logger == nil {
		t.Error("Logger returned nil")
	}

	// Test Meter
	meter := sh.Meter("test-scope")
	if meter == nil {
		t.Error("Meter returned nil")
	}

	// Test Tracer
	tracer := sh.Tracer("test-scope")
	if tracer == nil {
		t.Error("Tracer returned nil")
	}
}

func TestCapitanIntegration(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Define signal and keys
	testSig := capitan.NewSignal("test.event", "Test Event")
	msgKey := capitan.NewStringKey("message")
	countKey := capitan.NewIntKey("count")

	// Emit event through capitan
	cap.Emit(ctx, testSig,
		msgKey.Field("test message"),
		countKey.Field(42),
	)

	// Give some time for async processing
	time.Sleep(100 * time.Millisecond)

	// If we got here without panics, the integration is working
	// (Full validation would require capturing OTLP output)
}

func TestClose(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}

	// Close should complete without panic
	sh.Close()
}

func TestApply(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Start with no config
	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Define signals
	_ = capitan.NewSignal("test.one", "Test One")
	_ = capitan.NewSignal("test.two", "Test Two")

	// Apply a new schema with whitelist
	schema := Schema{
		Logs: &LogSchema{
			Whitelist: []string{"test.one"},
		},
	}

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify config was updated
	if sh.config.Logs == nil {
		t.Fatal("expected Logs config to be set")
	}
	if len(sh.config.Logs.WhitelistNames) != 1 {
		t.Errorf("expected 1 signal in whitelist, got %d", len(sh.config.Logs.WhitelistNames))
	}

	// Apply empty schema (reset to defaults)
	err = sh.Apply(Schema{})
	if err != nil {
		t.Fatalf("Apply(empty) failed: %v", err)
	}

	// Verify config was reset
	if sh.config.Logs != nil {
		t.Error("expected Logs config to be nil after reset")
	}
}

func TestApply_MultipleUpdates(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	sig := capitan.NewSignal("test.signal", "Test Signal")

	// Apply multiple schemas in sequence
	for i := 0; i < 5; i++ {
		schema := Schema{
			Logs: &LogSchema{
				Whitelist: []string{"test.signal"},
			},
			Stdout: i%2 == 0,
		}

		err = sh.Apply(schema)
		if err != nil {
			t.Fatalf("Apply iteration %d failed: %v", i, err)
		}

		// Emit an event to ensure observer is functional
		cap.Emit(ctx, sig)
	}

	time.Sleep(50 * time.Millisecond)
}

func TestRegisterContextKey(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Register a context key
	type ctxKey string
	const userIDKey ctxKey = "user_id"
	sh.RegisterContextKey("user_id", userIDKey)

	// Verify key was registered
	if _, ok := sh.contextKeys["user_id"]; !ok {
		t.Error("expected context key to be registered")
	}

	// Apply schema with context extraction
	schema := Schema{
		Context: &ContextSchema{
			Logs: []string{"user_id"},
		},
	}

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify context extraction was configured
	if sh.config.ContextExtraction == nil {
		t.Fatal("expected ContextExtraction to be set")
	}
	if len(sh.config.ContextExtraction.Logs) != 1 {
		t.Errorf("expected 1 log context key, got %d", len(sh.config.ContextExtraction.Logs))
	}
}

func TestApply_UnregisteredContextKey(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Apply schema referencing unregistered context key
	schema := Schema{
		Context: &ContextSchema{
			Logs: []string{"unregistered_key"},
		},
	}

	err = sh.Apply(schema)
	if err == nil {
		t.Fatal("expected error for unregistered context key")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("expected 'not registered' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unregistered_key") {
		t.Errorf("expected error to mention key name, got: %v", err)
	}
}
