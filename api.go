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
//	ap, _ := aperture.New(capitan.Default(), logProvider, meterProvider, traceProvider)
//	defer ap.Close()
//
//	// Load and apply configuration
//	schema, _ := aperture.LoadSchemaFromYAML(configBytes)
//	ap.Apply(schema)
//
// # Configuration
//
// Aperture is configured via Schema (YAML or JSON). See [LoadSchemaFromYAML] and [LoadSchemaFromJSON].
//
//	schema, _ := aperture.LoadSchemaFromYAML([]byte(`
//	metrics:
//	  - signal: order.created
//	    name: orders_total
//	    type: counter
//	traces:
//	  - start: request.started
//	    end: request.completed
//	    correlation_key: request_id
//	logs:
//	  whitelist:
//	    - order.created
//	`))
//	ap.Apply(schema)
//
// # Context Extraction
//
// To extract values from context.Context and add them to OTEL signals, register context keys:
//
//	type ctxKey string
//	const userIDKey ctxKey = "user_id"
//
//	ap.RegisterContextKey("user_id", userIDKey)
//
// Then reference "user_id" in your schema's context section.
//
// # Hot Reload
//
// Use [Aperture.Apply] for dynamic configuration updates:
//
//	capacitor := flux.New[aperture.Schema](
//	    file.New("config.yaml"),
//	    func(_, schema aperture.Schema) error {
//	        return ap.Apply(schema)
//	    },
//	)
//	capacitor.Start(ctx)
//
// # Diagnostics
//
// Aperture emits diagnostic signals for operational visibility:
//   - [SignalMetricValueMissing]: Metric event lacks required value field
//   - [SignalTraceExpired]: Span start/end never matched within timeout
//   - [SignalTraceCorrelationMissing]: Trace event lacks correlation ID field
//
// These appear as DEBUG-level logs with "aperture.signal" attribute.
package aperture

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Aperture bridges capitan events to OTEL providers.
type Aperture struct {
	capitan       *capitan.Capitan
	logProvider   log.LoggerProvider
	meterProvider metric.MeterProvider
	traceProvider trace.TracerProvider

	config           config
	contextKeys      map[string]any // name → context key for ctx.Value()
	capitanObserver  *capitanObserver
	internalObserver *internalObserver

	mu sync.RWMutex
}

// New creates an Aperture instance that observes capitan events and forwards them to OTEL.
//
// Aperture starts with no configuration (logs all events). Use [Aperture.Apply] to set configuration.
//
// Parameters:
//   - c: Capitan instance to observe (required)
//   - logProvider: OTEL LoggerProvider (required)
//   - meterProvider: OTEL MeterProvider (required)
//   - traceProvider: OTEL TracerProvider (required)
//
// Example:
//
//	ap, err := aperture.New(capitan.Default(), logProvider, meterProvider, traceProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ap.Close()
//
//	schema, _ := aperture.LoadSchemaFromYAML(configBytes)
//	ap.Apply(schema)
func New(
	c *capitan.Capitan,
	logProvider log.LoggerProvider,
	meterProvider metric.MeterProvider,
	traceProvider trace.TracerProvider,
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

	s := &Aperture{
		capitan:       c,
		logProvider:   logProvider,
		meterProvider: meterProvider,
		traceProvider: traceProvider,
		config:        config{},
		contextKeys:   make(map[string]any),
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

// RegisterContextKey registers a context key for extraction.
//
// Context keys must be registered before they can be used in schema configuration.
// The name should match the name used in the schema's context section.
//
// Example:
//
//	type ctxKey string
//	const userIDKey ctxKey = "user_id"
//
//	ap.RegisterContextKey("user_id", userIDKey)
//
// Then in schema:
//
//	context:
//	  logs:
//	    - user_id
func (s *Aperture) RegisterContextKey(name string, key any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextKeys[name] = key
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

// Apply updates the aperture configuration atomically.
//
// This drains the current observer (waiting for queued events to complete),
// then creates a new one with the updated config. No events are lost during
// the transition.
//
// Example with flux for hot-reload:
//
//	capacitor := flux.New[aperture.Schema](
//	    file.New("config.yaml"),
//	    func(_, schema aperture.Schema) error {
//	        return ap.Apply(schema)
//	    },
//	)
//	capacitor.Start(ctx)
func (s *Aperture) Apply(schema Schema) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate schema first
	if err := schema.Validate(); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	// Build internal config from schema
	cfg, err := s.buildConfig(schema)
	if err != nil {
		return fmt.Errorf("building config: %w", err)
	}

	// Drain and close old observer
	if s.capitanObserver != nil {
		// Drain waits for all queued events to be processed
		if drainErr := s.capitanObserver.Drain(context.Background()); drainErr != nil {
			return fmt.Errorf("draining observer: %w", drainErr)
		}
		s.capitanObserver.Close()
	}

	// Update config
	s.config = *cfg

	// Create new observer with updated config
	observer, err := newCapitanObserver(s, s.capitan)
	if err != nil {
		return fmt.Errorf("creating observer: %w", err)
	}
	s.capitanObserver = observer

	return nil
}

// buildConfig converts a Schema to internal config.
func (s *Aperture) buildConfig(schema Schema) (*config, error) {
	cfg := &config{
		StdoutLogging: schema.Stdout,
	}

	// Convert metrics
	for _, m := range schema.Metrics {
		mc := metricConfig{
			SignalName:   m.Signal,
			Name:         m.Name,
			Type:         parseMetricType(m.Type),
			ValueKeyName: m.ValueKey,
			Description:  m.Description,
		}
		cfg.Metrics = append(cfg.Metrics, mc)
	}

	// Convert traces
	for _, t := range schema.Traces {
		tc := traceConfig{
			StartSignalName:    t.Start,
			EndSignalName:      t.End,
			CorrelationKeyName: t.CorrelationKey,
			SpanName:           t.SpanName,
			SpanTimeout:        parseTimeout(t.SpanTimeout),
		}
		cfg.Traces = append(cfg.Traces, tc)
	}

	// Convert logs
	if schema.Logs != nil && len(schema.Logs.Whitelist) > 0 {
		cfg.Logs = &logConfig{
			WhitelistNames: schema.Logs.Whitelist,
		}
	}

	// Convert context extraction
	if schema.Context != nil {
		ctxCfg := &contextExtractionConfig{}

		// Build log context keys
		for _, name := range schema.Context.Logs {
			key, ok := s.contextKeys[name]
			if !ok {
				return nil, fmt.Errorf("context key %q not registered (referenced in context.logs)", name)
			}
			ctxCfg.Logs = append(ctxCfg.Logs, ContextKey{Key: key, Name: name})
		}

		// Build metric context keys
		for _, name := range schema.Context.Metrics {
			key, ok := s.contextKeys[name]
			if !ok {
				return nil, fmt.Errorf("context key %q not registered (referenced in context.metrics)", name)
			}
			ctxCfg.Metrics = append(ctxCfg.Metrics, ContextKey{Key: key, Name: name})
		}

		// Build trace context keys
		for _, name := range schema.Context.Traces {
			key, ok := s.contextKeys[name]
			if !ok {
				return nil, fmt.Errorf("context key %q not registered (referenced in context.traces)", name)
			}
			ctxCfg.Traces = append(ctxCfg.Traces, ContextKey{Key: key, Name: name})
		}

		if len(ctxCfg.Logs) > 0 || len(ctxCfg.Metrics) > 0 || len(ctxCfg.Traces) > 0 {
			cfg.ContextExtraction = ctxCfg
		}
	}

	return cfg, nil
}

// parseMetricType converts a string to MetricType.
func parseMetricType(s string) MetricType {
	switch s {
	case "counter", "":
		return MetricTypeCounter
	case "gauge":
		return MetricTypeGauge
	case "histogram":
		return MetricTypeHistogram
	case "updowncounter":
		return MetricTypeUpDownCounter
	default:
		return MetricTypeCounter
	}
}

// parseTimeout parses a duration string, returning 5 minutes as default.
func parseTimeout(s string) time.Duration {
	if s == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}

// Close stops observing capitan events.
//
// Note: This does NOT shutdown the OTEL providers - that is the caller's responsibility.
// If using the providers package, call providers.Shutdown(ctx) separately.
func (s *Aperture) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.capitanObserver != nil {
		s.capitanObserver.Close()
	}
	if s.internalObserver != nil {
		s.internalObserver.Close()
	}
}
