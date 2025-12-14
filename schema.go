package aperture

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// LoadSchemaFromYAML parses a YAML byte slice into a Schema.
func LoadSchemaFromYAML(data []byte) (Schema, error) {
	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Schema{}, fmt.Errorf("yaml unmarshal: %w", err)
	}
	return s, nil
}

// LoadSchemaFromJSON parses a JSON byte slice into a Schema.
func LoadSchemaFromJSON(data []byte) (Schema, error) {
	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return Schema{}, fmt.Errorf("json unmarshal: %w", err)
	}
	return s, nil
}

// Schema is the serializable configuration for aperture.
// Load from YAML or JSON via [LoadSchemaFromYAML] or [LoadSchemaFromJSON], then apply via [Aperture.Apply].
type Schema struct {
	// Metrics specifies which signals should be converted to OTEL metrics.
	Metrics []MetricSchema `json:"metrics,omitempty" yaml:"metrics,omitempty"`

	// Traces specifies signal pairs that should be correlated into spans.
	Traces []TraceSchema `json:"traces,omitempty" yaml:"traces,omitempty"`

	// Logs configures which signals should be logged.
	// If nil or empty whitelist, all signals are logged.
	Logs *LogSchema `json:"logs,omitempty" yaml:"logs,omitempty"`

	// Context specifies context keys to extract for each signal type.
	Context *ContextSchema `json:"context,omitempty" yaml:"context,omitempty"`

	// Stdout enables duplication of OTEL output to stdout.
	Stdout bool `json:"stdout,omitempty" yaml:"stdout,omitempty"`
}

// MetricSchema defines a signal-to-metric conversion in serializable form.
type MetricSchema struct {
	// Signal is the name of the capitan signal to observe.
	Signal string `json:"signal" yaml:"signal"`

	// Name is the OTEL metric name.
	Name string `json:"name" yaml:"name"`

	// Type is the metric instrument type: counter, gauge, histogram, updowncounter.
	// Defaults to "counter" if not specified.
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	// ValueKey is the name of the field key to extract metric value from.
	// Required for gauge, histogram, and updowncounter.
	ValueKey string `json:"value_key,omitempty" yaml:"value_key,omitempty"`

	// Description is optional metric description.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// TraceSchema defines a signal pair that forms a trace span in serializable form.
type TraceSchema struct {
	// Start is the name of the signal that begins the span.
	Start string `json:"start" yaml:"start"`

	// End is the name of the signal that completes the span.
	End string `json:"end" yaml:"end"`

	// CorrelationKey is the name of the field key used to correlate start/end events.
	CorrelationKey string `json:"correlation_key" yaml:"correlation_key"`

	// SpanName is the name of the generated span.
	// If empty, uses the start signal name.
	SpanName string `json:"span_name,omitempty" yaml:"span_name,omitempty"`

	// SpanTimeout is the maximum duration to wait for an end event (e.g., "5m", "30s").
	// Defaults to 5 minutes if not specified.
	SpanTimeout string `json:"span_timeout,omitempty" yaml:"span_timeout,omitempty"`
}

// LogSchema configures log filtering in serializable form.
type LogSchema struct {
	// Whitelist specifies signal names to log.
	// If empty, all signals are logged.
	Whitelist []string `json:"whitelist,omitempty" yaml:"whitelist,omitempty"`
}

// ContextSchema defines context values to extract for each signal type.
type ContextSchema struct {
	// Logs specifies context key names to extract for log attributes.
	Logs []string `json:"logs,omitempty" yaml:"logs,omitempty"`

	// Metrics specifies context key names to extract for metric dimensions.
	// WARNING: High-cardinality values can significantly increase storage costs.
	Metrics []string `json:"metrics,omitempty" yaml:"metrics,omitempty"`

	// Traces specifies context key names to extract for span attributes.
	Traces []string `json:"traces,omitempty" yaml:"traces,omitempty"`
}

// Validate checks that required fields are present in the schema.
func (s Schema) Validate() error {
	for i, m := range s.Metrics {
		if m.Signal == "" {
			return fmt.Errorf("metrics[%d]: signal is required", i)
		}
		if m.Name == "" {
			return fmt.Errorf("metrics[%d]: name is required", i)
		}
		// ValueKey required for non-counter types
		if m.Type != "" && m.Type != "counter" && m.ValueKey == "" {
			return fmt.Errorf("metrics[%d]: value_key is required for type %q", i, m.Type)
		}
	}

	for i, t := range s.Traces {
		if t.Start == "" {
			return fmt.Errorf("traces[%d]: start is required", i)
		}
		if t.End == "" {
			return fmt.Errorf("traces[%d]: end is required", i)
		}
		if t.CorrelationKey == "" {
			return fmt.Errorf("traces[%d]: correlation_key is required", i)
		}
	}

	return nil
}
