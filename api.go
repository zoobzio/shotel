// Package aperture bridges capitan event coordination with OpenTelemetry observability.
//
// Aperture observes all capitan events and transforms them into OTEL signals:
//   - Logs: All events are logged by default (configurable via whitelist)
//   - Metrics: Configured signals become counters, gauges, or histograms
//   - Traces: Signal pairs are correlated into spans via a correlation key
//
// # Basic Usage
//
//	// Configure OTEL providers (see go.opentelemetry.io/otel/sdk)
//	logProvider := log.NewLoggerProvider(...)
//	meterProvider := metric.NewMeterProvider(...)
//	traceProvider := trace.NewTracerProvider(...)
//
//	sh, _ := aperture.New(capitan.Default(), logProvider, meterProvider, traceProvider, nil)
//	defer sh.Close()
//
// # Configuration
//
// Pass a [Config] to control signal transformation:
//
//	config := &aperture.Config{
//	    Metrics: []aperture.MetricConfig{
//	        {Signal: orderCreated, Name: "orders_total", Type: aperture.MetricTypeCounter},
//	    },
//	    Traces: []aperture.TraceConfig{
//	        {Start: reqStart, End: reqEnd, CorrelationKey: &reqID, SpanName: "http.request"},
//	    },
//	    Logs: &aperture.LogConfig{Whitelist: []capitan.Signal{orderCreated}},
//	}
//
// # Custom Field Types
//
// Register transformers for user-defined types via [Config.Transformers]:
//
//	config := &aperture.Config{
//	    Transformers: map[capitan.Variant]aperture.FieldTransformer{
//	        orderVariant: aperture.MakeTransformer(func(key string, o Order) []log.KeyValue {
//	            return []log.KeyValue{log.String(key+".id", o.ID)}
//	        }),
//	    },
//	}
//
// # Diagnostics
//
// Aperture emits diagnostic signals for operational visibility:
//   - [SignalTransformSkipped]: Field type has no registered transformer
//   - [SignalMetricValueMissing]: Metric event lacks required value field
//   - [SignalTraceExpired]: Span start/end never matched within timeout
//   - [SignalTraceCorrelationMissing]: Trace event lacks correlation ID field
//
// These appear as DEBUG-level logs with "aperture.signal" attribute.
package aperture

import (
	"fmt"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Aperture bridges capitan events to OTEL providers.
type Aperture struct {
	logProvider   log.LoggerProvider
	meterProvider metric.MeterProvider
	traceProvider trace.TracerProvider

	config           Config
	capitanObserver  *capitanObserver
	internalObserver *internalObserver
}

// New creates a Aperture instance that observes capitan events and forwards them to OTEL.
//
// Aperture automatically transforms capitan events based on the provided configuration.
// If no config is provided, all events are logged (backward compatible).
//
// Parameters:
//   - c: Capitan instance to observe (required)
//   - logProvider: OTEL LoggerProvider (required)
//   - meterProvider: OTEL MeterProvider (required)
//   - traceProvider: OTEL TracerProvider (required)
//   - config: Optional configuration (pass nil for defaults)
//
// Example with configuration:
//
//	config := &aperture.Config{
//	    Metrics: []aperture.MetricConfig{
//	        {Signal: orderCreated, Name: "orders_created_total"},
//	    },
//	    Logs: &aperture.LogConfig{Whitelist: []capitan.Signal{orderCreated}},
//	}
//	sh := aperture.New(capitan.Default(), providers.Log, providers.Meter, providers.Trace, config)
func New(
	c *capitan.Capitan,
	logProvider log.LoggerProvider,
	meterProvider metric.MeterProvider,
	traceProvider trace.TracerProvider,
	config *Config,
) (*Aperture, error) {
	if c == nil {
		return nil, fmt.Errorf("capitan instance is required")
	}
	if logProvider == nil {
		return nil, fmt.Errorf("log provider is required")
	}
	if meterProvider == nil {
		return nil, fmt.Errorf("meter provider is required")
	}
	if traceProvider == nil {
		return nil, fmt.Errorf("trace provider is required")
	}

	// Use empty config if nil
	cfg := Config{}
	if config != nil {
		cfg = *config
	}

	s := &Aperture{
		logProvider:   logProvider,
		meterProvider: meterProvider,
		traceProvider: traceProvider,
		config:        cfg,
	}

	// Create internal diagnostic observer
	s.internalObserver = newInternalObserver(s.logProvider.Logger("aperture.internal"))

	// Attach capitan observer
	observer, err := newCapitanObserver(s, c)
	if err != nil {
		return nil, fmt.Errorf("creating observer: %w", err)
	}
	s.capitanObserver = observer

	return s, nil
}

// Logger returns an OTEL logger for the given scope name.
//
// The scope name typically represents the package or component emitting logs.
func (s *Aperture) Logger(name string) log.Logger {
	return s.logProvider.Logger(name)
}

// Meter returns an OTEL meter for the given scope name.
//
// The scope name typically represents the package or component emitting metrics.
func (s *Aperture) Meter(name string) metric.Meter {
	return s.meterProvider.Meter(name)
}

// Tracer returns an OTEL tracer for the given scope name.
//
// The scope name typically represents the package or component emitting traces.
func (s *Aperture) Tracer(name string) trace.Tracer {
	return s.traceProvider.Tracer(name)
}

// Close stops observing capitan events.
//
// Note: This does NOT shutdown the OTEL providers - that is the caller's responsibility.
// If using the providers package, call providers.Shutdown(ctx) separately.
func (s *Aperture) Close() {
	if s.capitanObserver != nil {
		s.capitanObserver.Close()
	}
	if s.internalObserver != nil {
		s.internalObserver.Close()
	}
}
