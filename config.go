package aperture

import (
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
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

	// ContextExtraction specifies context keys to extract and add to OTEL signals.
	// If nil, no context extraction is performed.
	ContextExtraction *ContextExtractionConfig

	// StdoutLogging enables duplication of OTEL output to stdout.
	// When true, all OTEL signals are logged to stdout in human-readable format using slog.
	StdoutLogging bool

	// Transformers registers custom field transformers for user-defined types.
	// Keys are capitan variants, values are transformer functions.
	// Use RegisterTransformer helper to build this map with type safety.
	Transformers map[capitan.Variant]FieldTransformer
}

// FieldTransformer converts a capitan field to OTEL log attributes.
// Return nil or empty slice to skip the field.
type FieldTransformer func(f capitan.Field) []log.KeyValue

// TransformerFunc is a type-safe transformer for a specific field type.
// Use with MakeTransformer to create a FieldTransformer.
type TransformerFunc[T any] func(key string, value T) []log.KeyValue

// MakeTransformer creates a FieldTransformer from a type-safe TransformerFunc.
//
// Example:
//
//	type OrderInfo struct {
//	    ID    string
//	    Total float64
//	}
//
//	config := &aperture.Config{
//	    Transformers: map[capitan.Variant]aperture.FieldTransformer{
//	        orderVariant: aperture.MakeTransformer(func(key string, order OrderInfo) []log.KeyValue {
//	            return []log.KeyValue{
//	                log.String(key+".id", order.ID),
//	                log.Float64(key+".total", order.Total),
//	            }
//	        }),
//	    },
//	}
func MakeTransformer[T any](fn TransformerFunc[T]) FieldTransformer {
	return func(f capitan.Field) []log.KeyValue {
		if gf, ok := f.(capitan.GenericField[T]); ok {
			return fn(f.Key().Name(), gf.Get())
		}
		return nil
	}
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

// ContextKey defines a key-name pair for extracting values from context.Context.
type ContextKey struct {
	// Key is the context key used with context.Value().
	// Typically an unexported type to avoid collisions.
	Key any

	// Name is the attribute name to use in OTEL signals.
	Name string
}

// ContextExtractionConfig defines context values to extract for each signal type.
type ContextExtractionConfig struct {
	// Logs specifies context keys to extract and add to log attributes.
	Logs []ContextKey

	// Metrics specifies context keys to extract and add to metric dimensions.
	// WARNING: High-cardinality values (like unique request IDs) can significantly
	// increase metric storage costs. Use only low-cardinality values.
	Metrics []ContextKey

	// Traces specifies context keys to extract and add to span attributes.
	Traces []ContextKey
}
