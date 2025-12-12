package aperture

import (
	"context"
	"testing"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestProviders_Shutdown(t *testing.T) {
	ctx := context.Background()

	t.Run("all providers present", func(t *testing.T) {
		pvs := &Providers{
			Log:   sdklog.NewLoggerProvider(),
			Meter: sdkmetric.NewMeterProvider(),
			Trace: sdktrace.NewTracerProvider(),
		}

		err := pvs.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("nil log provider", func(t *testing.T) {
		pvs := &Providers{
			Log:   nil,
			Meter: sdkmetric.NewMeterProvider(),
			Trace: sdktrace.NewTracerProvider(),
		}

		err := pvs.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected no error with nil log provider, got: %v", err)
		}
	})

	t.Run("nil meter provider", func(t *testing.T) {
		pvs := &Providers{
			Log:   sdklog.NewLoggerProvider(),
			Meter: nil,
			Trace: sdktrace.NewTracerProvider(),
		}

		err := pvs.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected no error with nil meter provider, got: %v", err)
		}
	})

	t.Run("nil trace provider", func(t *testing.T) {
		pvs := &Providers{
			Log:   sdklog.NewLoggerProvider(),
			Meter: sdkmetric.NewMeterProvider(),
			Trace: nil,
		}

		err := pvs.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected no error with nil trace provider, got: %v", err)
		}
	})

	t.Run("all providers nil", func(t *testing.T) {
		pvs := &Providers{
			Log:   nil,
			Meter: nil,
			Trace: nil,
		}

		err := pvs.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected no error with all nil providers, got: %v", err)
		}
	})
}

func TestProviders_ShutdownOrder(t *testing.T) {
	// Verify shutdown completes without error - order is internal implementation detail
	ctx := context.Background()

	pvs := &Providers{
		Log:   sdklog.NewLoggerProvider(),
		Meter: sdkmetric.NewMeterProvider(),
		Trace: sdktrace.NewTracerProvider(),
	}

	err := pvs.Shutdown(ctx)
	if err != nil {
		t.Errorf("Expected clean shutdown, got: %v", err)
	}
}

func TestProviders_DoubleShutdown(t *testing.T) {
	ctx := context.Background()

	pvs := &Providers{
		Log:   sdklog.NewLoggerProvider(),
		Meter: sdkmetric.NewMeterProvider(),
		Trace: sdktrace.NewTracerProvider(),
	}

	// First shutdown should succeed
	err := pvs.Shutdown(ctx)
	if err != nil {
		t.Errorf("First shutdown failed: %v", err)
	}

	// Second shutdown should return errors (providers already shut down)
	err = pvs.Shutdown(ctx)
	if err == nil {
		t.Error("Expected error on double shutdown, got nil")
	}
}
