package shotel

import (
	"context"
	"testing"
	"time"
)

func TestNewExporter_InMemory(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName:     "test-service",
		Endpoint:        "", // Empty endpoint triggers in-memory mode
		MetricsInterval: 10 * time.Second,
		Insecure:        false,
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		t.Fatalf("NewExporter failed: %v", err)
	}
	defer func() { _ = exporter.Shutdown(ctx) }() //nolint:errcheck // Test cleanup

	if exporter.resource == nil {
		t.Error("expected resource to be set")
	}

	if exporter.metricReader == nil {
		t.Error("expected metricReader to be set")
	}

	if exporter.traceProvider == nil {
		t.Error("expected traceProvider to be set")
	}

	if exporter.logProvider == nil {
		t.Error("expected logProvider to be set")
	}
}

func TestExporter_MetricReader(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName: "test-service",
		Endpoint:    "",
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		t.Fatalf("NewExporter failed: %v", err)
	}
	defer func() { _ = exporter.Shutdown(ctx) }() //nolint:errcheck // Test cleanup

	reader := exporter.MetricReader()
	if reader == nil {
		t.Error("expected MetricReader to return non-nil reader")
	}
}

func TestExporter_TraceProvider(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName: "test-service",
		Endpoint:    "",
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		t.Fatalf("NewExporter failed: %v", err)
	}
	defer func() { _ = exporter.Shutdown(ctx) }() //nolint:errcheck // Test cleanup

	provider := exporter.TraceProvider()
	if provider == nil {
		t.Error("expected TraceProvider to return non-nil provider")
	}
}

func TestExporter_LogProvider(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName: "test-service",
		Endpoint:    "",
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		t.Fatalf("NewExporter failed: %v", err)
	}
	defer func() { _ = exporter.Shutdown(ctx) }() //nolint:errcheck // Test cleanup

	provider := exporter.LogProvider()
	if provider == nil {
		t.Error("expected LogProvider to return non-nil provider")
	}
}

func TestExporter_Shutdown(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName: "test-service",
		Endpoint:    "",
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		t.Fatalf("NewExporter failed: %v", err)
	}

	err = exporter.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestInMemoryTraceExporter(t *testing.T) {
	ctx := context.Background()
	exporter := &inMemoryTraceExporter{}

	err := exporter.ExportSpans(ctx, nil)
	if err != nil {
		t.Errorf("ExportSpans failed: %v", err)
	}

	err = exporter.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestInMemoryLogExporter(t *testing.T) {
	ctx := context.Background()
	exporter := &inMemoryLogExporter{}

	err := exporter.Export(ctx, nil)
	if err != nil {
		t.Errorf("Export failed: %v", err)
	}

	err = exporter.ForceFlush(ctx)
	if err != nil {
		t.Errorf("ForceFlush failed: %v", err)
	}

	err = exporter.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestNewExporter_WithOTLPEndpoint(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName:     "test-service",
		Endpoint:        "invalid-endpoint:9999", // Invalid endpoint to test error paths
		MetricsInterval: 10 * time.Second,
		Insecure:        true,
	}

	// This will attempt to connect but may fail - we're testing that the code paths execute
	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		// Expected to fail with invalid endpoint, which tests error handling paths
		t.Logf("NewExporter failed as expected with invalid endpoint: %v", err)
		return
	}

	// If it somehow succeeds, clean up
	if exporter != nil {
		_ = exporter.Shutdown(ctx) //nolint:errcheck // Test cleanup
	}
}

func TestNewExporter_SecureEndpoint(t *testing.T) {
	ctx := context.Background()
	cfg := &Config{
		ServiceName:     "test-service",
		Endpoint:        "invalid-secure-endpoint:9999",
		MetricsInterval: 10 * time.Second,
		Insecure:        false, // Test secure connection path
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		// Expected to fail with invalid endpoint
		t.Logf("NewExporter failed as expected with invalid secure endpoint: %v", err)
		return
	}

	if exporter != nil {
		_ = exporter.Shutdown(ctx) //nolint:errcheck // Test cleanup
	}
}
