// Package shotel bridges capitan event coordination with OpenTelemetry observability.
//
// Shotel observes all capitan events and transforms them into OTEL logs automatically,
// while exposing standard OTEL Logger, Meter, and Tracer interfaces for direct use.
//
// OTEL provider configuration is handled externally - use the providers package for
// common setups, or construct your own providers for full control.
package shotel

import (
	"fmt"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Shotel bridges capitan events to OTEL providers.
type Shotel struct {
	logProvider   log.LoggerProvider
	meterProvider metric.MeterProvider
	traceProvider trace.TracerProvider

	config          Config
	capitanObserver *capitanObserver
}

// New creates a Shotel instance that observes capitan events and forwards them to OTEL.
//
// Shotel automatically transforms capitan events based on the provided configuration.
// If no config is provided, all events are logged (backward compatible).
//
// Parameters:
//   - c: Capitan instance to observe (required)
//   - logProvider: OTEL LoggerProvider (required)
//   - meterProvider: OTEL MeterProvider (required)
//   - traceProvider: OTEL TracerProvider (required)
//   - config: Optional configuration (pass nil for defaults)
//
// Example with providers package:
//
//	providers, err := providers.Default(ctx, "my-service", "v1.0.0", "localhost:4318")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer providers.Shutdown(ctx)
//
//	sh := shotel.New(capitan.Default(), providers.Log, providers.Meter, providers.Trace, nil)
//	defer sh.Close()
//
// Example with configuration:
//
//	config := &shotel.Config{
//	    Metrics: []shotel.MetricConfig{
//	        {Signal: orderCreated, Name: "orders_created_total"},
//	    },
//	    Logs: &shotel.LogConfig{Whitelist: []capitan.Signal{orderCreated}},
//	}
//	sh := shotel.New(capitan.Default(), providers.Log, providers.Meter, providers.Trace, config)
func New(
	c *capitan.Capitan,
	logProvider log.LoggerProvider,
	meterProvider metric.MeterProvider,
	traceProvider trace.TracerProvider,
	config *Config,
) (*Shotel, error) {
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

	s := &Shotel{
		logProvider:   logProvider,
		meterProvider: meterProvider,
		traceProvider: traceProvider,
		config:        cfg,
	}

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
func (s *Shotel) Logger(name string) log.Logger {
	return s.logProvider.Logger(name)
}

// Meter returns an OTEL meter for the given scope name.
//
// The scope name typically represents the package or component emitting metrics.
func (s *Shotel) Meter(name string) metric.Meter {
	return s.meterProvider.Meter(name)
}

// Tracer returns an OTEL tracer for the given scope name.
//
// The scope name typically represents the package or component emitting traces.
func (s *Shotel) Tracer(name string) trace.Tracer {
	return s.traceProvider.Tracer(name)
}

// Close stops observing capitan events.
//
// Note: This does NOT shutdown the OTEL providers - that is the caller's responsibility.
// If using the providers package, call providers.Shutdown(ctx) separately.
func (s *Shotel) Close() {
	if s.capitanObserver != nil {
		s.capitanObserver.Close()
	}
}
