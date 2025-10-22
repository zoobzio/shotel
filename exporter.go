package shotel

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Exporter manages OTLP exporters for metrics, traces, and logs.
type Exporter struct {
	resource       *resource.Resource
	metricExporter metric.Exporter
	traceExporter  sdktrace.SpanExporter
	logExporter    sdklog.Exporter
	metricReader   metric.Reader
	traceProvider  *sdktrace.TracerProvider
	logProvider    *sdklog.LoggerProvider
}

// NewExporter creates OTLP exporters for metrics, traces, and logs.
// If cfg.Endpoint is empty, creates in-memory exporters suitable for testing.
func NewExporter(ctx context.Context, cfg *Config) (*Exporter, error) {
	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// If endpoint is empty, create in-memory exporters for testing
	if cfg.Endpoint == "" {
		cfg.Logger.Info("creating in-memory exporters for testing")
		return newInMemoryExporter(res, cfg)
	}

	cfg.Logger.Debug("creating OTLP exporters",
		slog.String("endpoint", cfg.Endpoint),
		slog.Bool("insecure", cfg.Insecure))

	// Create metric exporter
	metricOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}

	metricExporter, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		cfg.Logger.Error("failed to create metric exporter", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}
	cfg.Logger.Debug("created metric exporter")

	// Create trace exporter
	traceOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}

	traceExporter, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		cfg.Logger.Error("failed to create trace exporter", slog.Any("error", err))
		_ = metricExporter.Shutdown(ctx) //nolint:errcheck // Cleanup on error path
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}
	cfg.Logger.Debug("created trace exporter")

	// Create log exporter
	logOpts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}

	logExporter, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		cfg.Logger.Error("failed to create log exporter", slog.Any("error", err))
		_ = metricExporter.Shutdown(ctx) //nolint:errcheck // Cleanup on error path
		_ = traceExporter.Shutdown(ctx)  //nolint:errcheck // Cleanup on error path
		return nil, fmt.Errorf("failed to create log exporter: %w", err)
	}
	cfg.Logger.Debug("created log exporter")

	// Create metric reader
	metricReader := metric.NewPeriodicReader(
		metricExporter,
		metric.WithInterval(cfg.MetricsInterval),
	)

	// Create trace provider
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)

	// Create log provider
	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)

	return &Exporter{
		resource:       res,
		metricExporter: metricExporter,
		traceExporter:  traceExporter,
		logExporter:    logExporter,
		metricReader:   metricReader,
		traceProvider:  traceProvider,
		logProvider:    logProvider,
	}, nil
}

// newInMemoryExporter creates in-memory exporters for testing.
func newInMemoryExporter(res *resource.Resource, _ *Config) (*Exporter, error) {
	// Use manual metric reader that doesn't poll automatically
	metricReader := metric.NewManualReader()

	// Use in-memory trace exporter
	traceExporter := &inMemoryTraceExporter{}
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSyncer(traceExporter),
	)

	// Use in-memory log exporter
	logExporter := &inMemoryLogExporter{}
	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(logExporter)),
	)

	return &Exporter{
		resource:       res,
		metricExporter: nil, // Not used in test mode
		traceExporter:  traceExporter,
		logExporter:    logExporter,
		metricReader:   metricReader,
		traceProvider:  traceProvider,
		logProvider:    logProvider,
	}, nil
}

// In-memory exporters for testing.
type inMemoryTraceExporter struct{}

func (e *inMemoryTraceExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *inMemoryTraceExporter) Shutdown(_ context.Context) error {
	return nil
}

type inMemoryLogExporter struct{}

func (e *inMemoryLogExporter) Export(_ context.Context, _ []sdklog.Record) error {
	return nil
}

func (e *inMemoryLogExporter) Shutdown(_ context.Context) error {
	return nil
}

func (e *inMemoryLogExporter) ForceFlush(_ context.Context) error {
	return nil
}

// MetricReader returns the metric reader for integration with MeterProvider.
func (e *Exporter) MetricReader() metric.Reader {
	return e.metricReader
}

// TraceProvider returns the trace provider for span creation.
func (e *Exporter) TraceProvider() *sdktrace.TracerProvider {
	return e.traceProvider
}

// LogProvider returns the log provider for log record creation.
func (e *Exporter) LogProvider() *sdklog.LoggerProvider {
	return e.logProvider
}

// Shutdown gracefully shuts down all exporters.
func (e *Exporter) Shutdown(ctx context.Context) error {
	var errs []error

	if err := e.traceProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
	}

	if err := e.logProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
	}

	if err := e.metricReader.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("metric reader shutdown: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// SetLogger updates the logger used by the exporter.
// Note: This is called internally by Shotel during construction.
func (e *Exporter) SetLogger(logger *slog.Logger) {
	_ = logger // Logger stored in Shotel, not Exporter
}
