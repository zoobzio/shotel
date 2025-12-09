package aperture

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
)

// Providers holds configured OTEL SDK providers for logs, metrics, and traces.
//
// Use [DefaultProviders] for a pre-configured setup, or construct manually
// for full control over exporters, batching, and sampling.
//
// Always call [Providers.Shutdown] before application exit to flush pending
// telemetry data.
type Providers struct {
	// Log provides OTEL loggers. Pass to [New] as the logProvider parameter.
	Log *log.LoggerProvider

	// Meter provides OTEL meters. Pass to [New] as the meterProvider parameter.
	Meter *metric.MeterProvider

	// Trace provides OTEL tracers. Pass to [New] as the traceProvider parameter.
	Trace *trace.TracerProvider
}

// Shutdown gracefully shuts down all providers, flushing any pending telemetry.
//
// Call this before application exit, typically via defer:
//
//	providers, _ := aperture.DefaultProviders(ctx, "my-service", "v1.0.0", "localhost:4318")
//	defer providers.Shutdown(ctx)
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

// DefaultProviders creates OTLP providers with opinionated defaults.
//
// Configuration:
//   - OTLP HTTP exporters for all signals
//   - Insecure connection (for local development)
//   - Batch processing for logs and traces
//   - Periodic reader (60s) for metrics
//   - Always-sample strategy for traces
//
// Example:
//
//	providers, err := aperture.DefaultProviders(ctx, "my-service", "v1.0.0", "localhost:4318")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer providers.Shutdown(ctx)
//
//	sh, err := aperture.New(capitan.Default(), providers.Log, providers.Meter, providers.Trace, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sh.Close()
func DefaultProviders(
	ctx context.Context,
	serviceName string,
	serviceVersion string,
	otlpEndpoint string,
) (*Providers, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if serviceVersion == "" {
		return nil, fmt.Errorf("service version is required")
	}
	if otlpEndpoint == "" {
		return nil, fmt.Errorf("OTLP endpoint is required")
	}

	// Create resource with service identity
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	// Create log provider
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(otlpEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating log exporter: %w", err)
	}

	logProvider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)

	// Create meter provider
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(otlpEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		_ = logProvider.Shutdown(ctx) //nolint:errcheck // Best effort cleanup
		return nil, fmt.Errorf("creating metric exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(60*time.Second),
		)),
	)

	// Create trace provider
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		_ = logProvider.Shutdown(ctx)   //nolint:errcheck // Best effort cleanup
		_ = meterProvider.Shutdown(ctx) //nolint:errcheck // Best effort cleanup
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(traceExporter)),
		trace.WithSampler(trace.AlwaysSample()),
	)

	return &Providers{
		Log:   logProvider,
		Meter: meterProvider,
		Trace: traceProvider,
	}, nil
}
