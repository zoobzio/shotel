package shotel

import (
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// fieldsToAttributes transforms capitan fields to OTEL log attributes.
//
// Only built-in capitan field variants are supported. Custom field types
// are skipped (to be handled by future transformer registration system).
func fieldsToAttributes(fields []capitan.Field) []log.KeyValue {
	attrs := make([]log.KeyValue, 0, len(fields))

	for _, f := range fields {
		key := f.Key().Name()

		switch f.Variant() {
		case capitan.VariantString:
			if gf, ok := f.(capitan.GenericField[string]); ok {
				attrs = append(attrs, log.String(key, gf.Get()))
			}

		case capitan.VariantInt:
			if gf, ok := f.(capitan.GenericField[int]); ok {
				attrs = append(attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantInt32:
			if gf, ok := f.(capitan.GenericField[int32]); ok {
				attrs = append(attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantInt64:
			if gf, ok := f.(capitan.GenericField[int64]); ok {
				attrs = append(attrs, log.Int64(key, gf.Get()))
			}

		case capitan.VariantUint:
			if gf, ok := f.(capitan.GenericField[uint]); ok {
				attrs = append(attrs, log.Int64(key, int64(gf.Get()))) //nolint:gosec // Intentional uint to int64 conversion for OTEL
			}

		case capitan.VariantUint32:
			if gf, ok := f.(capitan.GenericField[uint32]); ok {
				attrs = append(attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantUint64:
			if gf, ok := f.(capitan.GenericField[uint64]); ok {
				attrs = append(attrs, log.Int64(key, int64(gf.Get()))) //nolint:gosec // Intentional uint64 to int64 conversion for OTEL
			}

		case capitan.VariantFloat32:
			if gf, ok := f.(capitan.GenericField[float32]); ok {
				attrs = append(attrs, log.Float64(key, float64(gf.Get())))
			}

		case capitan.VariantFloat64:
			if gf, ok := f.(capitan.GenericField[float64]); ok {
				attrs = append(attrs, log.Float64(key, gf.Get()))
			}

		case capitan.VariantBool:
			if gf, ok := f.(capitan.GenericField[bool]); ok {
				attrs = append(attrs, log.Bool(key, gf.Get()))
			}

		case capitan.VariantTime:
			if gf, ok := f.(capitan.GenericField[time.Time]); ok {
				// Store as Unix timestamp in seconds
				attrs = append(attrs, log.Int64(key, gf.Get().Unix()))
			}

		case capitan.VariantDuration:
			if gf, ok := f.(capitan.GenericField[time.Duration]); ok {
				// Store as nanoseconds
				attrs = append(attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantBytes:
			if gf, ok := f.(capitan.GenericField[[]byte]); ok {
				attrs = append(attrs, log.Bytes(key, gf.Get()))
			}

		case capitan.VariantError:
			if gf, ok := f.(capitan.GenericField[error]); ok {
				attrs = append(attrs, log.String(key, gf.Get().Error()))
			}

		default:
			// Check registry for custom transformer
			if transformer, ok := globalRegistry.lookup(f.Variant()); ok {
				if customAttrs := transformer(f); len(customAttrs) > 0 {
					attrs = append(attrs, customAttrs...)
				}
			}
			// Otherwise skip unknown type
		}
	}

	return attrs
}

// fieldsToMetricAttributes transforms capitan fields to OTEL metric attributes.
func fieldsToMetricAttributes(fields []capitan.Field) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(fields))

	for _, f := range fields {
		key := f.Key().Name()

		switch f.Variant() {
		case capitan.VariantString:
			if gf, ok := f.(capitan.GenericField[string]); ok {
				attrs = append(attrs, attribute.String(key, gf.Get()))
			}

		case capitan.VariantInt:
			if gf, ok := f.(capitan.GenericField[int]); ok {
				attrs = append(attrs, attribute.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantInt32:
			if gf, ok := f.(capitan.GenericField[int32]); ok {
				attrs = append(attrs, attribute.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantInt64:
			if gf, ok := f.(capitan.GenericField[int64]); ok {
				attrs = append(attrs, attribute.Int64(key, gf.Get()))
			}

		case capitan.VariantUint:
			if gf, ok := f.(capitan.GenericField[uint]); ok {
				attrs = append(attrs, attribute.Int64(key, int64(gf.Get()))) //nolint:gosec // Intentional uint to int64 conversion for OTEL
			}

		case capitan.VariantUint32:
			if gf, ok := f.(capitan.GenericField[uint32]); ok {
				attrs = append(attrs, attribute.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantUint64:
			if gf, ok := f.(capitan.GenericField[uint64]); ok {
				attrs = append(attrs, attribute.Int64(key, int64(gf.Get()))) //nolint:gosec // Intentional uint64 to int64 conversion for OTEL
			}

		case capitan.VariantFloat32:
			if gf, ok := f.(capitan.GenericField[float32]); ok {
				attrs = append(attrs, attribute.Float64(key, float64(gf.Get())))
			}

		case capitan.VariantFloat64:
			if gf, ok := f.(capitan.GenericField[float64]); ok {
				attrs = append(attrs, attribute.Float64(key, gf.Get()))
			}

		case capitan.VariantBool:
			if gf, ok := f.(capitan.GenericField[bool]); ok {
				attrs = append(attrs, attribute.Bool(key, gf.Get()))
			}

		case capitan.VariantTime:
			if gf, ok := f.(capitan.GenericField[time.Time]); ok {
				attrs = append(attrs, attribute.Int64(key, gf.Get().Unix()))
			}

		case capitan.VariantDuration:
			if gf, ok := f.(capitan.GenericField[time.Duration]); ok {
				attrs = append(attrs, attribute.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantBytes:
			if gf, ok := f.(capitan.GenericField[[]byte]); ok {
				attrs = append(attrs, attribute.String(key, string(gf.Get())))
			}

		case capitan.VariantError:
			if gf, ok := f.(capitan.GenericField[error]); ok {
				attrs = append(attrs, attribute.String(key, gf.Get().Error()))
			}

			// Custom types skipped for metrics
		}
	}

	return attrs
}
