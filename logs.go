package shotel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// CreateLogHandler returns a hook handler function that sends events to OTLP logs.
// This handler can be registered with hookz events or any context-aware event system.
//
// Usage with hookz:
//
//	handler := shotel.CreateLogHandler()
//	engine.OnRequestReceived(handler)
//	engine.OnRequestCompleted(handler)
func (s *Shotel) CreateLogHandler() func(context.Context, any) error {
	s.slogger.Debug("creating log handler for event bridging")
	return func(ctx context.Context, event any) error {
		// Build log record
		record := otellog.Record{}
		record.SetTimestamp(time.Now())
		record.SetSeverity(otellog.SeverityInfo)

		// Format event as body
		body := fmt.Sprintf("%v", event)
		record.SetBody(otellog.StringValue(body))

		// Emit to OTLP
		s.logger.Emit(ctx, record)

		return nil
	}
}

// CreateSlogHandler returns an slog.Handler that sends logs to OTLP.
// This allows standard structured logging to be exported via OTLP.
//
// Usage:
//
//	logger := slog.New(shotel.CreateSlogHandler())
//	logger.Info("request received", "method", "GET", "path", "/api/users")
func (s *Shotel) CreateSlogHandler() slog.Handler {
	s.slogger.Debug("creating slog handler for OTLP bridging")
	return &slogHandler{
		logger: s.logger,
		attrs:  make([]otellog.KeyValue, 0),
	}
}

// slogHandler implements slog.Handler to bridge slog -> OTLP logs.
type slogHandler struct {
	logger otellog.Logger
	attrs  []otellog.KeyValue
	group  string
}

// Enabled reports whether the handler handles records at the given level.
func (h *slogHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle processes a log record.
func (h *slogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Build OTLP log record
	otelRecord := otellog.Record{}
	otelRecord.SetTimestamp(record.Time)
	otelRecord.SetSeverity(convertSlogLevel(record.Level))
	otelRecord.SetBody(otellog.StringValue(record.Message))

	// Add handler-level attributes
	attrs := make([]otellog.KeyValue, 0, len(h.attrs)+record.NumAttrs())
	attrs = append(attrs, h.attrs...)

	// Add record-level attributes
	record.Attrs(func(attr slog.Attr) bool {
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs = append(attrs, convertSlogAttr(key, attr.Value))
		return true
	})

	otelRecord.AddAttributes(attrs...)

	// Emit to OTLP
	h.logger.Emit(ctx, otelRecord)

	return nil
}

// WithAttrs returns a new handler with additional attributes.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]otellog.KeyValue, len(h.attrs), len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)

	for _, attr := range attrs {
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		newAttrs = append(newAttrs, convertSlogAttr(key, attr.Value))
	}

	return &slogHandler{
		logger: h.logger,
		attrs:  newAttrs,
		group:  h.group,
	}
}

// WithGroup returns a new handler with a group prefix.
func (h *slogHandler) WithGroup(name string) slog.Handler {
	newGroup := name
	if h.group != "" {
		newGroup = h.group + "." + name
	}

	return &slogHandler{
		logger: h.logger,
		attrs:  h.attrs,
		group:  newGroup,
	}
}

// convertSlogLevel converts slog.Level to OTLP severity.
func convertSlogLevel(level slog.Level) otellog.Severity {
	switch {
	case level >= slog.LevelError:
		return otellog.SeverityError
	case level >= slog.LevelWarn:
		return otellog.SeverityWarn
	case level >= slog.LevelInfo:
		return otellog.SeverityInfo
	default:
		return otellog.SeverityDebug
	}
}

// convertSlogAttr converts slog.Value to OTLP KeyValue.
func convertSlogAttr(key string, value slog.Value) otellog.KeyValue {
	switch value.Kind() {
	case slog.KindString:
		return otellog.String(key, value.String())
	case slog.KindInt64:
		return otellog.Int64(key, value.Int64())
	case slog.KindUint64:
		uval := value.Uint64()
		if uval > uint64(1<<63-1) {
			return otellog.String(key, fmt.Sprintf("%d", uval))
		}
		return otellog.Int64(key, int64(uval)) //nolint:gosec // Range checked above
	case slog.KindFloat64:
		return otellog.Float64(key, value.Float64())
	case slog.KindBool:
		return otellog.Bool(key, value.Bool())
	case slog.KindDuration:
		return otellog.Int64(key, value.Duration().Nanoseconds())
	case slog.KindTime:
		return otellog.String(key, value.Time().Format(time.RFC3339Nano))
	default:
		return otellog.String(key, value.String())
	}
}

// SetGlobalSlogHandler sets the global slog default handler to use OTLP.
// This redirects all slog.Info/Warn/Error calls to OTLP automatically.
func (s *Shotel) SetGlobalSlogHandler() {
	s.slogger.Info("setting global slog handler to OTLP")
	slog.SetDefault(slog.New(s.CreateSlogHandler()))
}

// SetGlobalOtelLogger sets the global OTLP logger provider.
// This allows other OTLP-aware libraries to use the same log provider.
func (s *Shotel) SetGlobalOtelLogger() {
	s.slogger.Info("setting global OTLP logger provider")
	global.SetLoggerProvider(s.logProvider)
}
