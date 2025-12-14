package aperture

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/metric"
)

// metricInstrument holds a configured OTEL metric instrument.
type metricInstrument struct {
	config metricConfig

	// Only one of these will be set based on Type
	// For counters, always int64
	// For others, both int64 and float64 are created to handle any numeric type
	int64Counter         metric.Int64Counter
	int64UpDownCounter   metric.Int64UpDownCounter
	float64UpDownCounter metric.Float64UpDownCounter
	int64Gauge           metric.Int64Gauge
	float64Gauge         metric.Float64Gauge
	int64Histogram       metric.Int64Histogram
	float64Histogram     metric.Float64Histogram
}

// metricsHandler manages auto-conversion of signals to OTEL metrics.
type metricsHandler struct {
	meter       metric.Meter
	instruments map[string]*metricInstrument // signal name → instrument
	contextKeys []ContextKey
}

// newMetricsHandler creates a metrics handler from config.
func newMetricsHandler(s *Aperture) (*metricsHandler, error) {
	if len(s.config.Metrics) == 0 {
		return nil, nil
	}

	// Extract context keys if configured
	var contextKeys []ContextKey
	if s.config.ContextExtraction != nil {
		contextKeys = s.config.ContextExtraction.Metrics
	}

	mh := &metricsHandler{
		meter:       s.meterProvider.Meter("capitan"),
		instruments: make(map[string]*metricInstrument),
		contextKeys: contextKeys,
	}

	// Pre-create all configured instruments
	for _, mc := range s.config.Metrics {
		// Default to counter if not specified
		if mc.Type == "" {
			mc.Type = MetricTypeCounter
		}

		// Validate configuration
		if err := validateMetricConfig(mc); err != nil {
			return nil, fmt.Errorf("invalid metric config for signal %q: %w", mc.SignalName, err)
		}

		inst := &metricInstrument{config: mc}

		// Create appropriate instrument based on type
		var err error
		switch mc.Type {
		case MetricTypeCounter:
			err = mh.createCounter(inst)
		case MetricTypeUpDownCounter:
			err = mh.createUpDownCounter(inst)
		case MetricTypeGauge:
			err = mh.createGauge(inst)
		case MetricTypeHistogram:
			err = mh.createHistogram(inst)
		default:
			return nil, fmt.Errorf("unknown metric type: %s", mc.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("creating %s for signal %q: %w", mc.Type, mc.SignalName, err)
		}

		mh.instruments[mc.SignalName] = inst
	}

	return mh, nil
}

// validateMetricConfig checks if the metric configuration is valid.
func validateMetricConfig(mc metricConfig) error {
	if mc.SignalName == "" {
		return fmt.Errorf("signal name is required")
	}
	if mc.Name == "" {
		return fmt.Errorf("metric name is required")
	}

	// Counter doesn't need ValueKey, others do
	if mc.Type != MetricTypeCounter && mc.Type != "" {
		if mc.ValueKeyName == "" {
			return fmt.Errorf("%s requires value_key", mc.Type)
		}
	}

	return nil
}

// createCounter creates a counter instrument (always int64, counts signals).
func (mh *metricsHandler) createCounter(inst *metricInstrument) error {
	counter, err := mh.meter.Int64Counter(
		inst.config.Name,
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.int64Counter = counter
	return nil
}

// createUpDownCounter creates up/down counter instruments (both int64 and float64).
func (mh *metricsHandler) createUpDownCounter(inst *metricInstrument) error {
	// Create both int64 and float64 counters - we'll use the appropriate one at runtime
	int64Counter, err := mh.meter.Int64UpDownCounter(
		inst.config.Name,
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.int64UpDownCounter = int64Counter

	float64Counter, err := mh.meter.Float64UpDownCounter(
		inst.config.Name+"_f64",
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.float64UpDownCounter = float64Counter

	return nil
}

// createGauge creates gauge instruments (both int64 and float64).
func (mh *metricsHandler) createGauge(inst *metricInstrument) error {
	int64Gauge, err := mh.meter.Int64Gauge(
		inst.config.Name,
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.int64Gauge = int64Gauge

	float64Gauge, err := mh.meter.Float64Gauge(
		inst.config.Name+"_f64",
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.float64Gauge = float64Gauge

	return nil
}

// createHistogram creates histogram instruments (both int64 and float64).
func (mh *metricsHandler) createHistogram(inst *metricInstrument) error {
	int64Histogram, err := mh.meter.Int64Histogram(
		inst.config.Name,
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.int64Histogram = int64Histogram

	float64Histogram, err := mh.meter.Float64Histogram(
		inst.config.Name+"_f64",
		metric.WithDescription(inst.config.Description),
	)
	if err != nil {
		return err
	}
	inst.float64Histogram = float64Histogram

	return nil
}

// handleEvent processes a capitan event and records metrics.
func (mh *metricsHandler) handleEvent(ctx context.Context, e *capitan.Event, internal *internalObserver) {
	if mh == nil {
		return
	}

	// Match signal by name
	inst, ok := mh.instruments[e.Signal().Name()]
	if !ok {
		return
	}

	// Convert fields to metric attributes
	attrs := fieldsToMetricAttributes(e.Fields())

	// Extract and add context values if configured
	if len(mh.contextKeys) > 0 {
		contextAttrs := extractContextValuesForMetrics(ctx, mh.contextKeys)
		attrs = append(attrs, contextAttrs...)
	}

	opts := metric.WithAttributes(attrs...)

	// Handle based on metric type
	switch inst.config.Type {
	case MetricTypeCounter:
		// Counter just counts signal occurrences
		inst.int64Counter.Add(ctx, 1, opts)

	case MetricTypeUpDownCounter:
		mh.recordUpDownCounter(ctx, inst, e, opts, internal)

	case MetricTypeGauge:
		mh.recordGauge(ctx, inst, e, opts, internal)

	case MetricTypeHistogram:
		mh.recordHistogram(ctx, inst, e, opts, internal)
	}
}

// recordUpDownCounter extracts value from event and records it.
func (*metricsHandler) recordUpDownCounter(ctx context.Context, inst *metricInstrument, e *capitan.Event, opts metric.AddOption, internal *internalObserver) {
	value := extractNumericValueByName(e, inst.config.ValueKeyName)
	if value == nil {
		internal.emit(ctx, SignalMetricValueMissing,
			internalSignal.Field(e.Signal().Name()),
			internalMetricName.Field(inst.config.Name),
			internalValueKey.Field(inst.config.ValueKeyName),
		)
		return
	}

	if value.isFloat {
		inst.float64UpDownCounter.Add(ctx, value.asFloat64(), opts)
	} else {
		inst.int64UpDownCounter.Add(ctx, value.asInt64(), opts)
	}
}

// recordGauge extracts value from event and records it.
func (*metricsHandler) recordGauge(ctx context.Context, inst *metricInstrument, e *capitan.Event, opts metric.RecordOption, internal *internalObserver) {
	value := extractNumericValueByName(e, inst.config.ValueKeyName)
	if value == nil {
		internal.emit(ctx, SignalMetricValueMissing,
			internalSignal.Field(e.Signal().Name()),
			internalMetricName.Field(inst.config.Name),
			internalValueKey.Field(inst.config.ValueKeyName),
		)
		return
	}

	if value.isFloat {
		inst.float64Gauge.Record(ctx, value.asFloat64(), opts)
	} else {
		inst.int64Gauge.Record(ctx, value.asInt64(), opts)
	}
}

// recordHistogram extracts value from event and records it.
func (*metricsHandler) recordHistogram(ctx context.Context, inst *metricInstrument, e *capitan.Event, opts metric.RecordOption, internal *internalObserver) {
	value := extractNumericValueByName(e, inst.config.ValueKeyName)
	if value == nil {
		internal.emit(ctx, SignalMetricValueMissing,
			internalSignal.Field(e.Signal().Name()),
			internalMetricName.Field(inst.config.Name),
			internalValueKey.Field(inst.config.ValueKeyName),
		)
		return
	}

	if value.isFloat {
		inst.float64Histogram.Record(ctx, value.asFloat64(), opts)
	} else {
		inst.int64Histogram.Record(ctx, value.asInt64(), opts)
	}
}

// numericValue holds a numeric value that can be converted to int64 or float64.
type numericValue struct {
	intValue   int64
	floatValue float64
	isFloat    bool
}

func (n *numericValue) asInt64() int64 {
	if n.isFloat {
		return int64(n.floatValue)
	}
	return n.intValue
}

func (n *numericValue) asFloat64() float64 {
	if n.isFloat {
		return n.floatValue
	}
	return float64(n.intValue)
}

// extractNumericValueByName extracts a numeric value from event fields by key name.
func extractNumericValueByName(e *capitan.Event, keyName string) *numericValue {
	if keyName == "" {
		return nil
	}

	for _, f := range e.Fields() {
		// Match by key name
		if f.Key().Name() != keyName {
			continue
		}

		// Extract value based on variant
		switch f.Variant() {
		case capitan.VariantInt:
			if gf, ok := f.(capitan.GenericField[int]); ok {
				return &numericValue{intValue: int64(gf.Get())}
			}
		case capitan.VariantInt32:
			if gf, ok := f.(capitan.GenericField[int32]); ok {
				return &numericValue{intValue: int64(gf.Get())}
			}
		case capitan.VariantInt64:
			if gf, ok := f.(capitan.GenericField[int64]); ok {
				return &numericValue{intValue: gf.Get()}
			}
		case capitan.VariantUint:
			if gf, ok := f.(capitan.GenericField[uint]); ok {
				return &numericValue{intValue: safeUintToInt64(gf.Get())}
			}
		case capitan.VariantUint32:
			if gf, ok := f.(capitan.GenericField[uint32]); ok {
				return &numericValue{intValue: int64(gf.Get())}
			}
		case capitan.VariantUint64:
			if gf, ok := f.(capitan.GenericField[uint64]); ok {
				return &numericValue{intValue: safeUint64ToInt64(gf.Get())}
			}
		case capitan.VariantFloat32:
			if gf, ok := f.(capitan.GenericField[float32]); ok {
				return &numericValue{floatValue: float64(gf.Get()), isFloat: true}
			}
		case capitan.VariantFloat64:
			if gf, ok := f.(capitan.GenericField[float64]); ok {
				return &numericValue{floatValue: gf.Get(), isFloat: true}
			}
		case capitan.VariantDuration:
			if gf, ok := f.(capitan.GenericField[time.Duration]); ok {
				// Convert duration to milliseconds
				return &numericValue{floatValue: float64(gf.Get()) / float64(time.Millisecond), isFloat: true}
			}
		}
	}

	return nil
}
