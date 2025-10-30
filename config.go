package shotel

import (
	"time"

	"github.com/zoobzio/capitan"
)

// Config configures how capitan events are transformed to OTEL signals.
type Config struct {
	// Metrics specifies which signals should be auto-converted to OTEL counters.
	Metrics []MetricConfig

	// Logs configures which signals should be logged.
	// If nil or empty, all signals are logged (default behavior).
	Logs *LogConfig

	// Traces configures signal pairs that should be correlated into spans.
	Traces []TraceConfig
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

// MetricConfig defines a signal-to-metric conversion.
type MetricConfig struct {
	// Signal is the capitan signal to observe.
	Signal capitan.Signal

	// Name is the OTEL metric name.
	// Required - must be a valid OTEL metric name.
	Name string

	// Type is the metric instrument type.
	// Defaults to MetricTypeCounter if not specified.
	Type MetricType

	// ValueKey is the field key to extract metric value from.
	// Required for Gauge, Histogram, and UpDownCounter.
	// Not used for Counter (counts signal occurrences).
	// Must have a numeric variant (int, int64, float64, etc.).
	ValueKey capitan.Key

	// Description is optional metric description.
	Description string
}

// LogConfig configures log filtering.
type LogConfig struct {
	// Whitelist specifies which signals should be logged.
	// If empty, all signals are logged.
	Whitelist []capitan.Signal
}

// TraceConfig defines a signal pair that forms a trace span.
type TraceConfig struct {
	// Start is the signal that begins the span.
	Start capitan.Signal

	// End is the signal that completes the span.
	End capitan.Signal

	// CorrelationKey is the field key used to correlate start/end events.
	// Both start and end events must have this field with matching values.
	CorrelationKey *capitan.StringKey

	// SpanName is the name of the generated span.
	// If empty, uses the Start signal as the span name.
	SpanName string

	// SpanTimeout is the maximum duration to wait for an end event.
	// If the end event doesn't arrive within this timeout, the span is
	// automatically ended and cleaned up to prevent memory leaks.
	// Defaults to 5 minutes if not specified or zero.
	SpanTimeout time.Duration
}
