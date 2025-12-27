package aperture

import (
	"time"
)

// config is the internal runtime configuration for aperture.
// Users configure via Schema (YAML/JSON), which is converted to config via buildConfig().
type config struct {
	// Pointers first for optimal GC pointer bitmap
	// Logs configures which signals should be logged.
	// If nil or empty, all signals are logged (default behavior).
	Logs *logConfig

	// ContextExtraction specifies context keys to extract and add to OTEL signals.
	// If nil, no context extraction is performed.
	ContextExtraction *contextExtractionConfig

	// Slices (pointer in first 8 bytes)
	// Metrics specifies which signals should be auto-converted to OTEL counters.
	Metrics []metricConfig

	// Traces configures signal pairs that should be correlated into spans.
	Traces []traceConfig

	// StdoutLogging enables duplication of OTEL output to stdout.
	// When true, all OTEL signals are logged to stdout in human-readable format using slog.
	StdoutLogging bool
}

// MetricType specifies the type of OTEL metric instrument.
type MetricType string

const (
	// MetricTypeCounter increments on each signal occurrence.
	// Does not use ValueKey - counts signals.
	MetricTypeCounter MetricType = "counter"

	// MetricTypeUpDownCounter increments or decrements based on ValueKey.
	// Requires ValueKey with numeric variant (int64 or float64).
	MetricTypeUpDownCounter MetricType = "updowncounter"

	// MetricTypeGauge records instantaneous value from ValueKey.
	// Requires ValueKey with numeric variant (int64 or float64).
	MetricTypeGauge MetricType = "gauge"

	// MetricTypeHistogram records value distribution from ValueKey.
	// Requires ValueKey with numeric variant (int64 or float64).
	MetricTypeHistogram MetricType = "histogram"
)

// metricConfig defines a signal-to-metric conversion (internal).
type metricConfig struct {
	// SignalName is the name of the capitan signal to observe.
	SignalName string

	// Name is the OTEL metric name.
	Name string

	// Type is the metric instrument type.
	// Defaults to MetricTypeCounter if not specified.
	Type MetricType

	// ValueKeyName is the name of the field key to extract metric value from.
	// Required for Gauge, Histogram, and UpDownCounter.
	// Not used for Counter (counts signal occurrences).
	ValueKeyName string

	// Description is optional metric description.
	Description string
}

// logConfig configures log filtering (internal).
type logConfig struct {
	// WhitelistNames specifies signal names to log.
	// If empty, all signals are logged.
	WhitelistNames []string
}

// traceConfig defines a signal pair that forms a trace span (internal).
type traceConfig struct {
	// StartSignalName is the name of the signal that begins the span.
	StartSignalName string

	// EndSignalName is the name of the signal that completes the span.
	EndSignalName string

	// CorrelationKeyName is the name of the field key used to correlate start/end events.
	// Both start and end events must have this field with matching values.
	CorrelationKeyName string

	// SpanName is the name of the generated span.
	// If empty, uses the start signal name.
	SpanName string

	// SpanTimeout is the maximum duration to wait for an end event.
	// If the end event doesn't arrive within this timeout, the span is
	// automatically ended and cleaned up to prevent memory leaks.
	// Defaults to 5 minutes if not specified or zero.
	SpanTimeout time.Duration
}

// ContextKey defines a key-name pair for extracting values from context.Context.
type ContextKey struct {
	// Key is the context key used with context.Value().
	// Typically an unexported type to avoid collisions.
	Key any

	// Name is the attribute name to use in OTEL signals.
	Name string
}

// contextExtractionConfig defines context values to extract for each signal type (internal).
type contextExtractionConfig struct {
	// Logs specifies context keys to extract and add to log attributes.
	Logs []ContextKey

	// Metrics specifies context keys to extract and add to metric dimensions.
	// WARNING: High-cardinality values (like unique request IDs) can significantly
	// increase metric storage costs. Use only low-cardinality values.
	Metrics []ContextKey

	// Traces specifies context keys to extract and add to span attributes.
	Traces []ContextKey
}
