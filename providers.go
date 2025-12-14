package aperture

import (
	"context"
	"fmt"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Providers holds OTEL SDK providers for logs, metrics, and traces.
//
// This is a convenience struct for managing OTEL provider lifecycle.
// The providers can be passed individually to [New], and this struct
// provides a unified [Shutdown] method.
//
// Example:
//
//	pvs := &aperture.Providers{Log: logProvider, Meter: meterProvider, Trace: traceProvider}
//	ap, _ := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
//	defer pvs.Shutdown(ctx)
//	defer ap.Close()
type Providers struct {
	// Log provides OTEL loggers.
	Log *sdklog.LoggerProvider

	// Meter provides OTEL meters.
	Meter *sdkmetric.MeterProvider

	// Trace provides OTEL tracers.
	Trace *sdktrace.TracerProvider
}

// Shutdown gracefully shuts down all providers, flushing any pending telemetry.
//
// Providers are shut down in reverse order (trace, meter, log) to ensure
// all telemetry is flushed before the log provider closes.
//
// Returns an error if any provider fails to shutdown cleanly.
func (p *Providers) Shutdown(ctx context.Context) error {
	var errs []error

	if p.Trace != nil {
		if err := p.Trace.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trace provider: %w", err))
		}
	}

	if p.Meter != nil {
		if err := p.Meter.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider: %w", err))
		}
	}

	if p.Log != nil {
		if err := p.Log.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("log provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}
