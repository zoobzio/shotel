package aperture

import (
	"fmt"
	"time"

	"github.com/zoobzio/capitan"
)

// Registry holds registered capitan components for schema resolution.
// Components are registered by their intrinsic names and resolved during Build().
type Registry struct {
	signals      map[string]capitan.Signal
	keys         map[string]capitan.Key
	contextKeys  map[string]ContextKey
	transformers map[capitan.Variant]FieldTransformer
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		signals:      make(map[string]capitan.Signal),
		keys:         make(map[string]capitan.Key),
		contextKeys:  make(map[string]ContextKey),
		transformers: make(map[capitan.Variant]FieldTransformer),
	}
}

// Register adds signals to the registry using their intrinsic names.
func (r *Registry) Register(signals ...capitan.Signal) {
	for _, s := range signals {
		r.signals[s.Name()] = s
	}
}

// RegisterKey adds keys to the registry using their intrinsic names.
func (r *Registry) RegisterKey(keys ...capitan.Key) {
	for _, k := range keys {
		r.keys[k.Name()] = k
	}
}

// RegisterContextKey adds a context key with a name for schema reference.
// The key is the value used with context.Value(), name is used in schema.
func (r *Registry) RegisterContextKey(name string, key any) {
	r.contextKeys[name] = ContextKey{Key: key, Name: name}
}

// RegisterTransformer adds a field transformer for a custom variant.
func (r *Registry) RegisterTransformer(v capitan.Variant, t FieldTransformer) {
	r.transformers[v] = t
}

// Build converts a serializable Schema to a runtime Config.
// Returns an error if any referenced signal, key, or context key is not registered.
func (r *Registry) Build(schema Schema) (*Config, error) {
	config := &Config{
		StdoutLogging: schema.Stdout,
		Transformers:  r.transformers,
	}

	// Build metrics
	for i, m := range schema.Metrics {
		mc, err := r.buildMetric(m)
		if err != nil {
			return nil, fmt.Errorf("metrics[%d]: %w", i, err)
		}
		config.Metrics = append(config.Metrics, mc)
	}

	// Build traces
	for i, t := range schema.Traces {
		tc, err := r.buildTrace(t)
		if err != nil {
			return nil, fmt.Errorf("traces[%d]: %w", i, err)
		}
		config.Traces = append(config.Traces, tc)
	}

	// Build logs
	if schema.Logs != nil {
		lc, err := r.buildLogs(schema.Logs)
		if err != nil {
			return nil, fmt.Errorf("logs: %w", err)
		}
		config.Logs = lc
	}

	// Build context extraction
	if schema.Context != nil {
		cc, err := r.buildContext(schema.Context)
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}
		config.ContextExtraction = cc
	}

	return config, nil
}

func (r *Registry) buildMetric(m MetricSchema) (MetricConfig, error) {
	signal, ok := r.signals[m.Signal]
	if !ok {
		return MetricConfig{}, fmt.Errorf("signal %q not registered", m.Signal)
	}

	mc := MetricConfig{
		Signal:      signal,
		Name:        m.Name,
		Type:        parseMetricType(m.Type),
		Description: m.Description,
	}

	if m.ValueKey != "" {
		key, ok := r.keys[m.ValueKey]
		if !ok {
			return MetricConfig{}, fmt.Errorf("value_key %q not registered", m.ValueKey)
		}
		mc.ValueKey = key
	}

	return mc, nil
}

func (r *Registry) buildTrace(t TraceSchema) (TraceConfig, error) {
	start, ok := r.signals[t.Start]
	if !ok {
		return TraceConfig{}, fmt.Errorf("start signal %q not registered", t.Start)
	}

	end, ok := r.signals[t.End]
	if !ok {
		return TraceConfig{}, fmt.Errorf("end signal %q not registered", t.End)
	}

	key, ok := r.keys[t.CorrelationKey]
	if !ok {
		return TraceConfig{}, fmt.Errorf("correlation_key %q not registered", t.CorrelationKey)
	}

	stringKey, ok := key.(*capitan.StringKey)
	if !ok {
		// Try direct type assertion for value type
		if sk, ok := key.(capitan.StringKey); ok {
			stringKey = &sk
		} else {
			return TraceConfig{}, fmt.Errorf("correlation_key %q must be a StringKey", t.CorrelationKey)
		}
	}

	tc := TraceConfig{
		Start:          start,
		End:            end,
		CorrelationKey: stringKey,
		SpanName:       t.SpanName,
	}

	if t.SpanTimeout != "" {
		d, err := time.ParseDuration(t.SpanTimeout)
		if err != nil {
			return TraceConfig{}, fmt.Errorf("span_timeout %q: %w", t.SpanTimeout, err)
		}
		tc.SpanTimeout = d
	}

	return tc, nil
}

func (r *Registry) buildLogs(l *LogSchema) (*LogConfig, error) {
	lc := &LogConfig{}

	for _, name := range l.Whitelist {
		signal, ok := r.signals[name]
		if !ok {
			return nil, fmt.Errorf("whitelist signal %q not registered", name)
		}
		lc.Whitelist = append(lc.Whitelist, signal)
	}

	return lc, nil
}

func (r *Registry) buildContext(c *ContextSchema) (*ContextExtractionConfig, error) {
	cc := &ContextExtractionConfig{}

	for _, name := range c.Logs {
		ck, ok := r.contextKeys[name]
		if !ok {
			return nil, fmt.Errorf("logs context key %q not registered", name)
		}
		cc.Logs = append(cc.Logs, ck)
	}

	for _, name := range c.Metrics {
		ck, ok := r.contextKeys[name]
		if !ok {
			return nil, fmt.Errorf("metrics context key %q not registered", name)
		}
		cc.Metrics = append(cc.Metrics, ck)
	}

	for _, name := range c.Traces {
		ck, ok := r.contextKeys[name]
		if !ok {
			return nil, fmt.Errorf("traces context key %q not registered", name)
		}
		cc.Traces = append(cc.Traces, ck)
	}

	return cc, nil
}

func parseMetricType(s string) MetricType {
	switch s {
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
