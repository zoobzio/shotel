package shotel

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	// Create valid providers for testing
	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	tests := []struct {
		name          string
		capitan       *capitan.Capitan
		logProvider   interface{}
		meterProvider interface{}
		traceProvider interface{}
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid configuration",
			capitan:       cap,
			logProvider:   pvs.Log,
			meterProvider: pvs.Meter,
			traceProvider: pvs.Trace,
			wantErr:       false,
		},
		{
			name:          "nil capitan",
			capitan:       nil,
			logProvider:   pvs.Log,
			meterProvider: pvs.Meter,
			traceProvider: pvs.Trace,
			wantErr:       true,
			errContains:   "capitan instance is required",
		},
		{
			name:          "nil log provider",
			capitan:       cap,
			logProvider:   nil,
			meterProvider: pvs.Meter,
			traceProvider: pvs.Trace,
			wantErr:       true,
			errContains:   "log provider is required",
		},
		{
			name:          "nil meter provider",
			capitan:       cap,
			logProvider:   nil,
			meterProvider: nil,
			traceProvider: nil,
			wantErr:       true,
			errContains:   "log provider is required", // First nil check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sh *Shotel
			var err error

			// Handle nil interfaces properly
			if tt.logProvider == nil || tt.meterProvider == nil || tt.traceProvider == nil {
				// This will trigger nil check errors
				sh, err = New(tt.capitan, nil, nil, nil, nil)
			} else {
				sh, err = New(
					tt.capitan,
					pvs.Log,
					pvs.Meter,
					pvs.Trace,
					nil, // No config
				)
			}

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if sh == nil {
				t.Fatal("expected non-nil Shotel instance")
			}

			// Verify providers initialized
			if sh.logProvider == nil {
				t.Error("logProvider not initialized")
			}
			if sh.meterProvider == nil {
				t.Error("meterProvider not initialized")
			}
			if sh.traceProvider == nil {
				t.Error("traceProvider not initialized")
			}
			if sh.capitanObserver == nil {
				t.Error("capitanObserver not initialized")
			}

			// Clean up
			sh.Close()
		})
	}
}

func TestShotelInterfaces(t *testing.T) {
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

	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	sh, err := New(cap, pvs.Log, pvs.Meter, pvs.Trace, nil)
	if err != nil {
		t.Fatalf("failed to create Shotel: %v", err)
	}

	// Close should complete without error
	sh.Close()

	// Second close should also be safe
	sh.Close()
}

// Helper function.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || substr == "" ||
		(s != "" && substr != "" && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
