package shotel

import (
	"context"
	"testing"
	"time"
)

func TestDefaultProviders(t *testing.T) {
	tests := []struct {
		name           string
		serviceName    string
		serviceVersion string
		otlpEndpoint   string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid configuration",
			serviceName:    "test-service",
			serviceVersion: "v1.0.0",
			otlpEndpoint:   "localhost:4318",
			wantErr:        false,
		},
		{
			name:           "missing service name",
			serviceName:    "",
			serviceVersion: "v1.0.0",
			otlpEndpoint:   "localhost:4318",
			wantErr:        true,
			errContains:    "service name is required",
		},
		{
			name:           "missing service version",
			serviceName:    "test-service",
			serviceVersion: "",
			otlpEndpoint:   "localhost:4318",
			wantErr:        true,
			errContains:    "service version is required",
		},
		{
			name:           "missing OTLP endpoint",
			serviceName:    "test-service",
			serviceVersion: "v1.0.0",
			otlpEndpoint:   "",
			wantErr:        true,
			errContains:    "OTLP endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			pvs, err := DefaultProviders(ctx, tt.serviceName, tt.serviceVersion, tt.otlpEndpoint)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && err.Error() != tt.errContains {
					t.Errorf("error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pvs == nil {
				t.Fatal("expected providers, got nil")
			}

			// Verify all providers are initialized
			if pvs.Log == nil {
				t.Error("log provider is nil")
			}
			if pvs.Meter == nil {
				t.Error("meter provider is nil")
			}
			if pvs.Trace == nil {
				t.Error("trace provider is nil")
			}

			// Clean shutdown
			if err := pvs.Shutdown(ctx); err != nil {
				t.Errorf("shutdown error: %v", err)
			}
		})
	}
}

func TestProviders_Shutdown(t *testing.T) {
	t.Run("successful shutdown", func(t *testing.T) {
		ctx := context.Background()

		pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
		if err != nil {
			t.Fatalf("failed to create providers: %v", err)
		}

		// Shutdown should succeed
		if err := pvs.Shutdown(ctx); err != nil {
			t.Errorf("shutdown failed: %v", err)
		}
	})

	t.Run("shutdown with nil providers", func(t *testing.T) {
		ctx := context.Background()

		pvs := &Providers{
			Log:   nil,
			Meter: nil,
			Trace: nil,
		}

		// Should not panic or error with nil providers
		if err := pvs.Shutdown(ctx); err != nil {
			t.Errorf("unexpected error with nil providers: %v", err)
		}
	})

	t.Run("shutdown with canceled context", func(t *testing.T) {
		ctx := context.Background()

		pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
		if err != nil {
			t.Fatalf("failed to create providers: %v", err)
		}

		// Create already-canceled context
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		// Shutdown with canceled context should complete (may return error)
		_ = pvs.Shutdown(canceledCtx)
	})

	t.Run("multiple shutdowns are safe", func(t *testing.T) {
		ctx := context.Background()

		pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
		if err != nil {
			t.Fatalf("failed to create providers: %v", err)
		}

		// First shutdown
		if err := pvs.Shutdown(ctx); err != nil {
			t.Errorf("first shutdown failed: %v", err)
		}

		// Second shutdown should not panic (may return error)
		_ = pvs.Shutdown(ctx)
	})
}

func TestProviders_Integration(t *testing.T) {
	t.Run("providers produce OTEL primitives", func(t *testing.T) {
		ctx := context.Background()

		pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
		if err != nil {
			t.Fatalf("failed to create providers: %v", err)
		}
		defer pvs.Shutdown(ctx)

		// Test that we can obtain loggers, meters, and tracers
		logger := pvs.Log.Logger("test")
		if logger == nil {
			t.Error("failed to obtain logger")
		}

		meter := pvs.Meter.Meter("test")
		if meter == nil {
			t.Error("failed to obtain meter")
		}

		tracer := pvs.Trace.Tracer("test")
		if tracer == nil {
			t.Error("failed to obtain tracer")
		}
	})

	t.Run("providers can create instruments", func(t *testing.T) {
		ctx := context.Background()

		pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
		if err != nil {
			t.Fatalf("failed to create providers: %v", err)
		}
		defer pvs.Shutdown(ctx)

		meter := pvs.Meter.Meter("test")

		// Create various metric instruments
		counter, err := meter.Int64Counter("test_counter")
		if err != nil {
			t.Errorf("failed to create counter: %v", err)
		}
		if counter == nil {
			t.Error("counter is nil")
		}

		gauge, err := meter.Int64Gauge("test_gauge")
		if err != nil {
			t.Errorf("failed to create gauge: %v", err)
		}
		if gauge == nil {
			t.Error("gauge is nil")
		}

		histogram, err := meter.Float64Histogram("test_histogram")
		if err != nil {
			t.Errorf("failed to create histogram: %v", err)
		}
		if histogram == nil {
			t.Error("histogram is nil")
		}
	})
}
