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
	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
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

	_, err = New(nil, pvs.Log, pvs.Meter, pvs.Trace, nil)
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

	_, err = New(cap, nil, pvs.Meter, pvs.Trace, nil)
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

	_, err = New(cap, pvs.Log, nil, pvs.Trace, nil)
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

	_, err = New(cap, pvs.Log, pvs.Meter, nil, nil)
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

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
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

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
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

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}

	// Close should complete without panic
	sh.Close()
}
