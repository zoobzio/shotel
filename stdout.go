package aperture

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/zoobzio/capitan"
)

// stdoutLogger writes human-readable logs to stdout using slog.
type stdoutLogger struct {
	logger *slog.Logger
}

// newStdoutLogger creates a new stdout logger.
func newStdoutLogger() *stdoutLogger {
	return &stdoutLogger{
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
	}
}

// logEvent writes a capitan event to stdout in human-readable format.
func (sl *stdoutLogger) logEvent(ctx context.Context, e *capitan.Event, contextKeys []ContextKey) {
	// Map capitan severity to slog level
	var level slog.Level
	switch e.Severity() {
	case capitan.SeverityDebug:
		level = slog.LevelDebug
	case capitan.SeverityInfo:
		level = slog.LevelInfo
	case capitan.SeverityWarn:
		level = slog.LevelWarn
	case capitan.SeverityError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Build slog attributes from event fields
	attrs := []slog.Attr{
		slog.String("signal", e.Signal().Name()),
		slog.Time("timestamp", e.Timestamp()),
	}

	// Add all event fields
	for _, field := range e.Fields() {
		attrs = append(attrs, fieldToSlogAttr(field))
	}

	// Extract and add context values if configured
	if len(contextKeys) > 0 {
		for _, ck := range contextKeys {
			val := ctx.Value(ck.Key)
			if val == nil {
				continue
			}

			// Convert context value to slog attribute
			attrs = append(attrs, contextValueToSlogAttr(ck.Name, val))
		}
	}

	// Log with signal description as message
	sl.logger.LogAttrs(ctx, level, e.Signal().Description(), attrs...)
}

// fieldToSlogAttr converts a capitan field to a slog attribute.
func fieldToSlogAttr(field capitan.Field) slog.Attr {
	key := field.Key().Name()

	switch field.Variant() {
	case capitan.VariantString:
		if gf, ok := field.(capitan.GenericField[string]); ok {
			return slog.String(key, gf.Get())
		}
	case capitan.VariantInt:
		if gf, ok := field.(capitan.GenericField[int]); ok {
			return slog.Int(key, gf.Get())
		}
	case capitan.VariantInt32:
		if gf, ok := field.(capitan.GenericField[int32]); ok {
			return slog.Int(key, int(gf.Get()))
		}
	case capitan.VariantInt64:
		if gf, ok := field.(capitan.GenericField[int64]); ok {
			return slog.Int64(key, gf.Get())
		}
	case capitan.VariantUint:
		if gf, ok := field.(capitan.GenericField[uint]); ok {
			return slog.Uint64(key, uint64(gf.Get()))
		}
	case capitan.VariantUint32:
		if gf, ok := field.(capitan.GenericField[uint32]); ok {
			return slog.Uint64(key, uint64(gf.Get()))
		}
	case capitan.VariantUint64:
		if gf, ok := field.(capitan.GenericField[uint64]); ok {
			return slog.Uint64(key, gf.Get())
		}
	case capitan.VariantFloat32:
		if gf, ok := field.(capitan.GenericField[float32]); ok {
			return slog.Float64(key, float64(gf.Get()))
		}
	case capitan.VariantFloat64:
		if gf, ok := field.(capitan.GenericField[float64]); ok {
			return slog.Float64(key, gf.Get())
		}
	case capitan.VariantBool:
		if gf, ok := field.(capitan.GenericField[bool]); ok {
			return slog.Bool(key, gf.Get())
		}
	case capitan.VariantTime:
		if gf, ok := field.(capitan.GenericField[time.Time]); ok {
			return slog.Time(key, gf.Get())
		}
	case capitan.VariantDuration:
		if gf, ok := field.(capitan.GenericField[time.Duration]); ok {
			return slog.Duration(key, gf.Get())
		}
	case capitan.VariantBytes:
		if gf, ok := field.(capitan.GenericField[[]byte]); ok {
			return slog.String(key, string(gf.Get()))
		}
	case capitan.VariantError:
		if gf, ok := field.(capitan.GenericField[error]); ok {
			return slog.String(key, gf.Get().Error())
		}
	}

	// Fallback for unknown types
	return slog.String(key, "unsupported")
}

// contextValueToSlogAttr converts a context value to a slog attribute.
func contextValueToSlogAttr(name string, val any) slog.Attr {
	switch v := val.(type) {
	case string:
		return slog.String(name, v)
	case int:
		return slog.Int(name, v)
	case int32:
		return slog.Int(name, int(v))
	case int64:
		return slog.Int64(name, v)
	case uint:
		return slog.Uint64(name, uint64(v))
	case uint32:
		return slog.Uint64(name, uint64(v))
	case uint64:
		return slog.Uint64(name, v)
	case float32:
		return slog.Float64(name, float64(v))
	case float64:
		return slog.Float64(name, v)
	case bool:
		return slog.Bool(name, v)
	case []byte:
		return slog.String(name, string(v))
	default:
		return slog.Any(name, v)
	}
}
