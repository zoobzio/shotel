package shotel

import (
	"context"
	"testing"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

func TestSeverityToOTEL(t *testing.T) {
	tests := []struct {
		name            string
		capitanSev      capitan.Severity
		expectedOTELSev log.Severity
	}{
		{
			name:            "debug maps to debug",
			capitanSev:      capitan.SeverityDebug,
			expectedOTELSev: log.SeverityDebug,
		},
		{
			name:            "info maps to info",
			capitanSev:      capitan.SeverityInfo,
			expectedOTELSev: log.SeverityInfo,
		},
		{
			name:            "warn maps to warn",
			capitanSev:      capitan.SeverityWarn,
			expectedOTELSev: log.SeverityWarn,
		},
		{
			name:            "error maps to error",
			capitanSev:      capitan.SeverityError,
			expectedOTELSev: log.SeverityError,
		},
		{
			name:            "unknown maps to info (default)",
			capitanSev:      capitan.Severity("unknown"),
			expectedOTELSev: log.SeverityInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := severityToOTEL(tt.capitanSev)
			if result != tt.expectedOTELSev {
				t.Errorf("severityToOTEL(%v) = %v, want %v", tt.capitanSev, result, tt.expectedOTELSev)
			}
		})
	}
}

func TestCapitanObserver_LogWhitelist(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	allowedSignal := capitan.NewSignal("allowed", "Allowed signal")
	blockedSignal := capitan.NewSignal("blocked", "Blocked signal")

	config := &Config{
		Logs: &LogConfig{
			Whitelist: []capitan.Signal{allowedSignal},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit both signals - only allowedSignal should be logged
	cap.Emit(ctx, allowedSignal)
	cap.Emit(ctx, blockedSignal)

	// Wait for processing
	// (Full validation would require capturing OTLP output)
}

func TestCapitanObserver_NoWhitelist(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// No whitelist - all events should be logged
	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	sig1 := capitan.NewSignal("event.one", "Event one")
	sig2 := capitan.NewSignal("event.two", "Event two")

	cap.Emit(ctx, sig1)
	cap.Emit(ctx, sig2)

	// Both should be logged
}

func TestCapitanObserver_EmptyWhitelist(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	// Empty whitelist - should behave like no whitelist (log all)
	config := &Config{
		Logs: &LogConfig{
			Whitelist: []capitan.Signal{},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	sig := capitan.NewSignal("test.event", "Test event")
	cap.Emit(ctx, sig)

	// Should be logged (empty whitelist = log all)
}

func TestCapitanObserver_Close(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}

	// Close should not panic
	sh.Close()

	// Multiple closes should be safe
	sh.Close()
}

func TestCapitanObserver_MetricsAndTracesIntegration(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	orderCreated := capitan.NewSignal("order.created", "Order created")
	requestStarted := capitan.NewSignal("request.started", "Request started")
	requestCompleted := capitan.NewSignal("request.completed", "Request completed")
	requestID := capitan.NewStringKey("request_id")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal: orderCreated,
				Name:   "orders_total",
				Type:   MetricTypeCounter,
			},
		},
		Traces: []TraceConfig{
			{
				Start:          requestStarted,
				End:            requestCompleted,
				CorrelationKey: &requestID,
				SpanName:       "request",
			},
		},
	}

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}
	defer sh.Close()

	// Emit events - should trigger metrics, traces, and logs
	cap.Emit(ctx, orderCreated)
	cap.Emit(ctx, requestStarted, requestID.Field("REQ-123"))
	cap.Emit(ctx, requestCompleted, requestID.Field("REQ-123"))

	// All handlers should process without panic
}

func TestCapitanObserver_SeverityPropagation(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

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

	testSignal := capitan.NewSignal("test.signal", "Test signal")

	// Emit events - capitan will set severity based on context
	// Testing that severity mapping works without panicking
	cap.Emit(ctx, testSignal)

	// Severity mapping is tested directly in TestSeverityToOTEL
}
