package aperture

import (
	"context"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

// capitanObserver observes all capitan events and transforms them to OTEL signals.
type capitanObserver struct {
	logger         log.Logger          // interface (16 bytes) - pointers first
	observer       *capitan.Observer   // pointers (8 bytes each)
	metricsHandler *metricsHandler
	tracesHandler  *tracesHandler
	logWhitelist   map[string]struct{} // signal name → allowed
	stdoutLogger   *stdoutLogger
	internal       *internalObserver
	logContextKeys []ContextKey // slice last (pointer in first 8 bytes)
}

// newCapitanObserver creates and attaches an observer to the capitan instance.
//
// Events are transformed to OTEL signals based on configuration.
func newCapitanObserver(s *Aperture, c *capitan.Capitan) (*capitanObserver, error) {
	// Create metrics handler if configured
	metricsHandler, err := newMetricsHandler(s)
	if err != nil {
		return nil, err
	}

	// Build log whitelist if configured (now uses signal names)
	var logWhitelist map[string]struct{}
	if s.config.Logs != nil && len(s.config.Logs.WhitelistNames) > 0 {
		logWhitelist = make(map[string]struct{})
		for _, name := range s.config.Logs.WhitelistNames {
			logWhitelist[name] = struct{}{}
		}
	}

	// Create traces handler if configured
	tracesHandler := newTracesHandler(s)

	// Extract context keys if configured
	var logContextKeys []ContextKey
	if s.config.ContextExtraction != nil {
		logContextKeys = s.config.ContextExtraction.Logs
	}

	// Create stdout logger if enabled
	var stdoutLogger *stdoutLogger
	if s.config.StdoutLogging {
		stdoutLogger = newStdoutLogger()
	}

	co := &capitanObserver{
		logger:         s.logProvider.Logger("capitan"),
		metricsHandler: metricsHandler,
		tracesHandler:  tracesHandler,
		logWhitelist:   logWhitelist,
		logContextKeys: logContextKeys,
		stdoutLogger:   stdoutLogger,
		internal:       s.internalObserver,
	}

	// Observe all signals
	co.observer = c.Observe(co.handleEvent)

	return co, nil
}

// handleEvent transforms a capitan event to OTEL signals based on configuration.
func (co *capitanObserver) handleEvent(ctx context.Context, e *capitan.Event) {
	// Log to stdout if enabled (before any filtering)
	if co.stdoutLogger != nil {
		co.stdoutLogger.logEvent(ctx, e, co.logContextKeys)
	}

	// Handle metrics if configured
	if co.metricsHandler != nil {
		co.metricsHandler.handleEvent(ctx, e, co.internal)
	}

	// Handle traces if configured
	if co.tracesHandler != nil {
		co.tracesHandler.handleEvent(ctx, e)
	}

	// Handle logs with whitelist filtering (now matches by signal name)
	if co.logWhitelist != nil {
		// Whitelist configured - only log if signal name is in whitelist
		if _, ok := co.logWhitelist[e.Signal().Name()]; !ok {
			return
		}
	}

	// Build log record
	var record log.Record

	// Set timestamp from event
	record.SetTimestamp(e.Timestamp())

	// Map capitan severity to OTEL severity
	record.SetSeverity(severityToOTEL(e.Severity()))
	record.SetSeverityText(string(e.Severity()))

	// Set message from signal description
	record.SetBody(log.StringValue(e.Signal().Description()))

	// Add signal as attribute
	record.AddAttributes(log.String("capitan.signal", e.Signal().Name()))

	// Transform and add all fields (no transformers - use JSON fallback)
	result := fieldsToAttributes(e.Fields())
	record.AddAttributes(result.attrs...)

	// Extract and add context values if configured
	if len(co.logContextKeys) > 0 {
		contextAttrs := extractContextValuesForLogs(ctx, co.logContextKeys)
		record.AddAttributes(contextAttrs...)
	}

	// Emit log record
	co.logger.Emit(ctx, record)
}

// severityToOTEL maps capitan severity to OTEL log severity.
func severityToOTEL(s capitan.Severity) log.Severity {
	switch s {
	case capitan.SeverityDebug:
		return log.SeverityDebug
	case capitan.SeverityInfo:
		return log.SeverityInfo
	case capitan.SeverityWarn:
		return log.SeverityWarn
	case capitan.SeverityError:
		return log.SeverityError
	default:
		return log.SeverityInfo // Default to Info for unrecognized severities
	}
}

// Drain blocks until all queued events have been processed.
func (co *capitanObserver) Drain(ctx context.Context) error {
	if co.observer != nil {
		return co.observer.Drain(ctx)
	}
	return nil
}

// Close stops observing capitan events.
func (co *capitanObserver) Close() {
	if co.observer != nil {
		co.observer.Close()
	}
	if co.tracesHandler != nil {
		co.tracesHandler.Close()
	}
}
