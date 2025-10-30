package shotel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/metric"
)

// metricInstrument holds a configured OTEL metric instrument.
type metricInstrument struct {
	config MetricConfig

	// Only one of these will be set based on Type and variant
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
	instruments map[capitan.Signal]*metricInstrument
	mu          sync.RWMutex
}

// newMetricsHandler creates a metrics handler from config.
func newMetricsHandler(s *Shotel) (*metricsHandler, error) {
	if len(s.config.Metrics) == 0 {
		return nil, nil
	}

	mh := &metricsHandler{
		meter:       s.meterProvider.Meter("capitan"),
		instruments: make(map[capitan.Signal]*metricInstrument),
	}

	// Pre-create all configured instruments
	for _, mc := range s.config.Metrics {
		// Default to counter if not specified
		if mc.Type == "" {
			mc.Type = MetricTypeCounter
		}

		// Validate configuration
		if err := validateMetricConfig(mc); err != nil {
			return nil, fmt.Errorf("invalid metric config for %s: %w", mc.Signal, err)
		}

		inst := &metricInstrument{config: mc}

		// Create appropriate instrument based on type and variant
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
			return nil, fmt.Errorf("creating %s for signal %s: %w", mc.Type, mc.Signal, err)
		}

		mh.instruments[mc.Signal] = inst
	}

	return mh, nil
}

// validateMetricConfig checks if the metric configuration is valid.
func validateMetricConfig(mc MetricConfig) error {
	if mc.Signal == "" {
		return fmt.Errorf("signal is required")
	}
	if mc.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Counter doesn't need ValueKey, others do
	if mc.Type != MetricTypeCounter && mc.Type != "" {
		if mc.ValueKey == nil {
			return fmt.Errorf("%s requires ValueKey", mc.Type)
		}
		// Check that ValueKey has numeric variant
		if !isNumericVariant(mc.ValueKey.Variant()) {
			return fmt.Errorf("ValueKey must have numeric variant, got %s", mc.ValueKey.Variant())
		}
	}

	return nil
}

// isNumericVariant checks if a variant represents a numeric type.
func isNumericVariant(v capitan.Variant) bool {
	switch v {
	case capitan.VariantInt, capitan.VariantInt32, capitan.VariantInt64,
		capitan.VariantUint, capitan.VariantUint32, capitan.VariantUint64,
		capitan.VariantFloat32, capitan.VariantFloat64,
		capitan.VariantDuration:
		return true
	default:
		return false
	}
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

// createUpDownCounter creates an up/down counter based on ValueKey variant.
func (mh *metricsHandler) createUpDownCounter(inst *metricInstrument) error {
	variant := inst.config.ValueKey.Variant()

	if isFloatVariant(variant) {
		counter, err := mh.meter.Float64UpDownCounter(
			inst.config.Name,
			metric.WithDescription(inst.config.Description),
		)
		if err != nil {
			return err
		}
		inst.float64UpDownCounter = counter
	} else {
		counter, err := mh.meter.Int64UpDownCounter(
			inst.config.Name,
			metric.WithDescription(inst.config.Description),
		)
		if err != nil {
			return err
		}
		inst.int64UpDownCounter = counter
	}

	return nil
}

// createGauge creates a gauge based on ValueKey variant.
func (mh *metricsHandler) createGauge(inst *metricInstrument) error {
	variant := inst.config.ValueKey.Variant()

	if isFloatVariant(variant) {
		gauge, err := mh.meter.Float64Gauge(
			inst.config.Name,
			metric.WithDescription(inst.config.Description),
		)
		if err != nil {
			return err
		}
		inst.float64Gauge = gauge
	} else {
		gauge, err := mh.meter.Int64Gauge(
			inst.config.Name,
			metric.WithDescription(inst.config.Description),
		)
		if err != nil {
			return err
		}
		inst.int64Gauge = gauge
	}

	return nil
}

// createHistogram creates a histogram based on ValueKey variant.
func (mh *metricsHandler) createHistogram(inst *metricInstrument) error {
	variant := inst.config.ValueKey.Variant()

	if isFloatVariant(variant) {
		histogram, err := mh.meter.Float64Histogram(
			inst.config.Name,
			metric.WithDescription(inst.config.Description),
		)
		if err != nil {
			return err
		}
		inst.float64Histogram = histogram
	} else {
		histogram, err := mh.meter.Int64Histogram(
			inst.config.Name,
			metric.WithDescription(inst.config.Description),
		)
		if err != nil {
			return err
		}
		inst.int64Histogram = histogram
	}

	return nil
}

// isFloatVariant checks if a variant is a floating-point type.
func isFloatVariant(v capitan.Variant) bool {
	return v == capitan.VariantFloat32 || v == capitan.VariantFloat64
}

// handleEvent processes a capitan event and records metrics.
func (mh *metricsHandler) handleEvent(ctx context.Context, e *capitan.Event) {
	if mh == nil {
		return
	}

	mh.mu.RLock()
	inst, ok := mh.instruments[e.Signal()]
	mh.mu.RUnlock()

	if !ok {
		return
	}

	// Convert fields to metric attributes
	attrs := fieldsToMetricAttributes(e.Fields())
	opts := metric.WithAttributes(attrs...)

	// Handle based on metric type
	switch inst.config.Type {
	case MetricTypeCounter:
		// Counter just counts signal occurrences
		inst.int64Counter.Add(ctx, 1, opts)

	case MetricTypeUpDownCounter:
		mh.recordUpDownCounter(ctx, inst, e, opts)

	case MetricTypeGauge:
		mh.recordGauge(ctx, inst, e, opts)

	case MetricTypeHistogram:
		mh.recordHistogram(ctx, inst, e, opts)
	}
}

// recordUpDownCounter extracts value from event and records it.
func (*metricsHandler) recordUpDownCounter(ctx context.Context, inst *metricInstrument, e *capitan.Event, opts metric.AddOption) {
	value := extractNumericValue(e, inst.config.ValueKey)
	if value == nil {
		return // Value not found or wrong type
	}

	if inst.int64UpDownCounter != nil {
		inst.int64UpDownCounter.Add(ctx, value.asInt64(), opts)
	} else if inst.float64UpDownCounter != nil {
		inst.float64UpDownCounter.Add(ctx, value.asFloat64(), opts)
	}
}

// recordGauge extracts value from event and records it.
func (*metricsHandler) recordGauge(ctx context.Context, inst *metricInstrument, e *capitan.Event, opts metric.RecordOption) {
	value := extractNumericValue(e, inst.config.ValueKey)
	if value == nil {
		return
	}

	if inst.int64Gauge != nil {
		inst.int64Gauge.Record(ctx, value.asInt64(), opts)
	} else if inst.float64Gauge != nil {
		inst.float64Gauge.Record(ctx, value.asFloat64(), opts)
	}
}

// recordHistogram extracts value from event and records it.
func (*metricsHandler) recordHistogram(ctx context.Context, inst *metricInstrument, e *capitan.Event, opts metric.RecordOption) {
	value := extractNumericValue(e, inst.config.ValueKey)
	if value == nil {
		return
	}

	if inst.int64Histogram != nil {
		inst.int64Histogram.Record(ctx, value.asInt64(), opts)
	} else if inst.float64Histogram != nil {
		inst.float64Histogram.Record(ctx, value.asFloat64(), opts)
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

// extractNumericValue extracts a numeric value from event fields.
func extractNumericValue(e *capitan.Event, key capitan.Key) *numericValue {
	if key == nil {
		return nil
	}

	for _, f := range e.Fields() {
		// Check if this is the field we're looking for
		if f.Key() != key {
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
				return &numericValue{intValue: int64(gf.Get())} //nolint:gosec // Intentional uint to int64 conversion for OTEL
			}
		case capitan.VariantUint32:
			if gf, ok := f.(capitan.GenericField[uint32]); ok {
				return &numericValue{intValue: int64(gf.Get())}
			}
		case capitan.VariantUint64:
			if gf, ok := f.(capitan.GenericField[uint64]); ok {
				return &numericValue{intValue: int64(gf.Get())} //nolint:gosec // Intentional uint64 to int64 conversion for OTEL
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
