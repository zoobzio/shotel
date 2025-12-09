package aperture

import (
	"context"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// transformResult holds the result of field transformation.
type transformResult struct {
	attrs   []log.KeyValue
	skipped []skippedField
}

// skippedField records a field that could not be transformed.
type skippedField struct {
	key     string
	variant string
}

// fieldsToAttributes transforms capitan fields to OTEL log attributes.
//
// Only built-in capitan field variants are supported. Custom field types
// are skipped unless a transformer is provided.
func fieldsToAttributes(fields []capitan.Field, transformers map[capitan.Variant]FieldTransformer) transformResult {
	result := transformResult{
		attrs: make([]log.KeyValue, 0, len(fields)),
	}

	for _, f := range fields {
		key := f.Key().Name()

		switch f.Variant() {
		case capitan.VariantString:
			if gf, ok := f.(capitan.GenericField[string]); ok {
				result.attrs = append(result.attrs, log.String(key, gf.Get()))
			}

		case capitan.VariantInt:
			if gf, ok := f.(capitan.GenericField[int]); ok {
				result.attrs = append(result.attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantInt32:
			if gf, ok := f.(capitan.GenericField[int32]); ok {
				result.attrs = append(result.attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantInt64:
			if gf, ok := f.(capitan.GenericField[int64]); ok {
				result.attrs = append(result.attrs, log.Int64(key, gf.Get()))
			}

		case capitan.VariantUint:
			if gf, ok := f.(capitan.GenericField[uint]); ok {
				result.attrs = append(result.attrs, log.Int64(key, int64(gf.Get()))) //nolint:gosec // Intentional uint to int64 conversion for OTEL
			}

		case capitan.VariantUint32:
			if gf, ok := f.(capitan.GenericField[uint32]); ok {
				result.attrs = append(result.attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantUint64:
			if gf, ok := f.(capitan.GenericField[uint64]); ok {
				result.attrs = append(result.attrs, log.Int64(key, int64(gf.Get()))) //nolint:gosec // Intentional uint64 to int64 conversion for OTEL
			}

		case capitan.VariantFloat32:
			if gf, ok := f.(capitan.GenericField[float32]); ok {
				result.attrs = append(result.attrs, log.Float64(key, float64(gf.Get())))
			}

		case capitan.VariantFloat64:
			if gf, ok := f.(capitan.GenericField[float64]); ok {
				result.attrs = append(result.attrs, log.Float64(key, gf.Get()))
			}

		case capitan.VariantBool:
			if gf, ok := f.(capitan.GenericField[bool]); ok {
				result.attrs = append(result.attrs, log.Bool(key, gf.Get()))
			}

		case capitan.VariantTime:
			if gf, ok := f.(capitan.GenericField[time.Time]); ok {
				// Store as Unix timestamp in seconds
				result.attrs = append(result.attrs, log.Int64(key, gf.Get().Unix()))
			}

		case capitan.VariantDuration:
			if gf, ok := f.(capitan.GenericField[time.Duration]); ok {
				// Store as nanoseconds
				result.attrs = append(result.attrs, log.Int64(key, int64(gf.Get())))
			}

		case capitan.VariantBytes:
			if gf, ok := f.(capitan.GenericField[[]byte]); ok {
				result.attrs = append(result.attrs, log.Bytes(key, gf.Get()))
			}

		case capitan.VariantError:
			if gf, ok := f.(capitan.GenericField[error]); ok {
				result.attrs = append(result.attrs, log.String(key, gf.Get().Error()))
			}

		default:
			// Check for custom transformer
			if transformer, ok := transformers[f.Variant()]; ok {
				if customAttrs := transformer(f); len(customAttrs) > 0 {
					result.attrs = append(result.attrs, customAttrs...)
				}
			} else {
				// Record skipped field for diagnostics
				result.skipped = append(result.skipped, skippedField{
					key:     key,
					variant: string(f.Variant()),
				})
			}
		}
	}

	return result
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

// extractContextValuesForLogs extracts values from context and converts them to log attributes.
// Values that don't exist in context are skipped.
func extractContextValuesForLogs(ctx context.Context, keys []ContextKey) []log.KeyValue {
	if len(keys) == 0 {
		return nil
	}

	attrs := make([]log.KeyValue, 0, len(keys))
	for _, ck := range keys {
		val := ctx.Value(ck.Key)
		if val == nil {
			continue
		}

		// Convert value to appropriate OTEL log attribute type
		switch v := val.(type) {
		case string:
			attrs = append(attrs, log.String(ck.Name, v))
		case int:
			attrs = append(attrs, log.Int64(ck.Name, int64(v)))
		case int32:
			attrs = append(attrs, log.Int64(ck.Name, int64(v)))
		case int64:
			attrs = append(attrs, log.Int64(ck.Name, v))
		case uint:
			attrs = append(attrs, log.Int64(ck.Name, int64(v))) //nolint:gosec // Intentional uint to int64 conversion for OTEL
		case uint32:
			attrs = append(attrs, log.Int64(ck.Name, int64(v)))
		case uint64:
			attrs = append(attrs, log.Int64(ck.Name, int64(v))) //nolint:gosec // Intentional uint64 to int64 conversion for OTEL
		case float32:
			attrs = append(attrs, log.Float64(ck.Name, float64(v)))
		case float64:
			attrs = append(attrs, log.Float64(ck.Name, v))
		case bool:
			attrs = append(attrs, log.Bool(ck.Name, v))
		case []byte:
			attrs = append(attrs, log.Bytes(ck.Name, v))
			// Skip unsupported types
		}
	}

	return attrs
}

// extractContextValuesForMetrics extracts values from context and converts them to metric attributes.
// Values that don't exist in context are skipped.
func extractContextValuesForMetrics(ctx context.Context, keys []ContextKey) []attribute.KeyValue {
	if len(keys) == 0 {
		return nil
	}

	attrs := make([]attribute.KeyValue, 0, len(keys))
	for _, ck := range keys {
		val := ctx.Value(ck.Key)
		if val == nil {
			continue
		}

		// Convert value to appropriate OTEL metric attribute type
		switch v := val.(type) {
		case string:
			attrs = append(attrs, attribute.String(ck.Name, v))
		case int:
			attrs = append(attrs, attribute.Int64(ck.Name, int64(v)))
		case int32:
			attrs = append(attrs, attribute.Int64(ck.Name, int64(v)))
		case int64:
			attrs = append(attrs, attribute.Int64(ck.Name, v))
		case uint:
			attrs = append(attrs, attribute.Int64(ck.Name, int64(v))) //nolint:gosec // Intentional uint to int64 conversion for OTEL
		case uint32:
			attrs = append(attrs, attribute.Int64(ck.Name, int64(v)))
		case uint64:
			attrs = append(attrs, attribute.Int64(ck.Name, int64(v))) //nolint:gosec // Intentional uint64 to int64 conversion for OTEL
		case float32:
			attrs = append(attrs, attribute.Float64(ck.Name, float64(v)))
		case float64:
			attrs = append(attrs, attribute.Float64(ck.Name, v))
		case bool:
			attrs = append(attrs, attribute.Bool(ck.Name, v))
		case []byte:
			attrs = append(attrs, attribute.String(ck.Name, string(v)))
			// Skip unsupported types
		}
	}

	return attrs
}
