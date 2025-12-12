package testing

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

func TestTestProviders(t *testing.T) {
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

			pvs, err := TestProviders(ctx, tt.serviceName, tt.serviceVersion, tt.otlpEndpoint)

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

			if pvs.Log == nil {
				t.Error("log provider is nil")
			}
			if pvs.Meter == nil {
				t.Error("meter provider is nil")
			}
			if pvs.Trace == nil {
				t.Error("trace provider is nil")
			}

			_ = pvs.Shutdown(ctx)
		})
	}
}

func TestLogCapture(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		capture := NewLogCapture()

		if capture.Count() != 0 {
			t.Errorf("expected 0 records, got %d", capture.Count())
		}

		records := capture.Records()
		if len(records) != 0 {
			t.Errorf("expected empty records, got %d", len(records))
		}
	})

	t.Run("reset clears records", func(t *testing.T) {
		capture := NewLogCapture()
		capture.Reset()

		if capture.Count() != 0 {
			t.Errorf("expected 0 after reset, got %d", capture.Count())
		}
	})

	t.Run("wait for count timeout", func(t *testing.T) {
		capture := NewLogCapture()

		start := time.Now()
		result := capture.WaitForCount(1, 10*time.Millisecond)
		elapsed := time.Since(start)

		if result {
			t.Error("expected timeout, got success")
		}
		if elapsed < 10*time.Millisecond {
			t.Errorf("returned too early: %v", elapsed)
		}
	})
}

func TestMockLogger(t *testing.T) {
	t.Run("enabled returns true", func(t *testing.T) {
		logger := NewMockLogger()
		if !logger.Enabled(context.Background(), log.EnabledParameters{}) {
			t.Error("expected Enabled to return true")
		}
	})

	t.Run("capture accessible", func(t *testing.T) {
		logger := NewMockLogger()
		capture := logger.Capture()
		if capture == nil {
			t.Error("expected capture to be non-nil")
		}
	})
}

func TestMockLoggerProvider(t *testing.T) {
	t.Run("returns logger", func(t *testing.T) {
		provider := NewMockLoggerProvider()
		logger := provider.Logger("test")
		if logger == nil {
			t.Error("expected logger to be non-nil")
		}
	})

	t.Run("capture accessible", func(t *testing.T) {
		provider := NewMockLoggerProvider()
		capture := provider.Capture()
		if capture == nil {
			t.Error("expected capture to be non-nil")
		}
	})
}

func TestEventCapture(t *testing.T) {
	t.Run("captures events", func(t *testing.T) {
		capture := NewEventCapture()

		if capture.Count() != 0 {
			t.Errorf("expected 0 events, got %d", capture.Count())
		}

		// Create a capitan instance and emit an event
		cap := capitan.New()
		sig := capitan.NewSignal("test.signal", "Test signal")
		key := capitan.NewStringKey("key")

		cap.Hook(sig, capture.Handler())
		cap.Emit(context.Background(), sig, key.Field("value"))
		cap.Shutdown()

		if capture.Count() != 1 {
			t.Errorf("expected 1 event, got %d", capture.Count())
		}

		events := capture.Events()
		if len(events) != 1 {
			t.Errorf("expected 1 event in slice, got %d", len(events))
		}

		if events[0].Signal != sig {
			t.Error("captured event has wrong signal")
		}
	})

	t.Run("reset clears events", func(t *testing.T) {
		capture := NewEventCapture()
		capture.Reset()

		if capture.Count() != 0 {
			t.Errorf("expected 0 after reset, got %d", capture.Count())
		}
	})

	t.Run("wait for count", func(t *testing.T) {
		capture := NewEventCapture()

		cap := capitan.New()
		sig := capitan.NewSignal("test.wait", "Test wait signal")
		cap.Hook(sig, capture.Handler())

		// Emit in background
		go func() {
			time.Sleep(5 * time.Millisecond)
			cap.Emit(context.Background(), sig)
		}()

		result := capture.WaitForCount(1, 100*time.Millisecond)
		cap.Shutdown()

		if !result {
			t.Error("expected success, got timeout")
		}
	})

	t.Run("wait for count timeout", func(t *testing.T) {
		capture := NewEventCapture()

		start := time.Now()
		result := capture.WaitForCount(1, 10*time.Millisecond)
		elapsed := time.Since(start)

		if result {
			t.Error("expected timeout, got success")
		}
		if elapsed < 10*time.Millisecond {
			t.Errorf("returned too early: %v", elapsed)
		}
	})

	t.Run("returns defensive copy", func(t *testing.T) {
		capture := NewEventCapture()

		cap := capitan.New()
		sig := capitan.NewSignal("test.copy", "Test copy signal")
		cap.Hook(sig, capture.Handler())
		cap.Emit(context.Background(), sig)
		cap.Shutdown()

		events1 := capture.Events()
		events2 := capture.Events()

		// Modifying one should not affect the other
		if len(events1) == 0 || len(events2) == 0 {
			t.Skip("no events captured")
		}

		// They should be equal but not the same slice
		if &events1[0] == &events2[0] {
			t.Error("expected defensive copy, got same pointer")
		}
	})
}
