package shotel

import (
	"context"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

// capitanObserver observes all capitan events and transforms them to OTEL signals.
type capitanObserver struct {
	observer       *capitan.Observer
	logger         log.Logger
	metricsHandler *metricsHandler
	tracesHandler  *tracesHandler
	logWhitelist   map[capitan.Signal]struct{}
}

// newCapitanObserver creates and attaches an observer to the capitan instance.
//
// Events are transformed to OTEL signals based on configuration.
func newCapitanObserver(s *Shotel, c *capitan.Capitan) (*capitanObserver, error) {
	// Create metrics handler if configured
	metricsHandler, err := newMetricsHandler(s)
	if err != nil {
		return nil, err
	}

	// Build log whitelist if configured
	var logWhitelist map[capitan.Signal]struct{}
	if s.config.Logs != nil && len(s.config.Logs.Whitelist) > 0 {
		logWhitelist = make(map[capitan.Signal]struct{})
		for _, sig := range s.config.Logs.Whitelist {
			logWhitelist[sig] = struct{}{}
		}
	}

	// Create traces handler if configured
	tracesHandler := newTracesHandler(s)

	co := &capitanObserver{
		logger:         s.logProvider.Logger("capitan"),
		metricsHandler: metricsHandler,
		tracesHandler:  tracesHandler,
		logWhitelist:   logWhitelist,
	}

	// Observe all signals
	co.observer = c.Observe(co.handleEvent)

	return co, nil
}

// handleEvent transforms a capitan event to OTEL signals based on configuration.
func (co *capitanObserver) handleEvent(ctx context.Context, e *capitan.Event) {
	// Handle metrics if configured
	if co.metricsHandler != nil {
		co.metricsHandler.handleEvent(ctx, e)
	}

	// Handle traces if configured
	if co.tracesHandler != nil {
		co.tracesHandler.handleEvent(ctx, e)
	}

	// Handle logs with whitelist filtering
	if co.logWhitelist != nil {
		// Whitelist configured - only log if signal is in whitelist
		if _, ok := co.logWhitelist[e.Signal()]; !ok {
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

	// Add signal as attribute
	record.AddAttributes(log.String("capitan.signal", string(e.Signal())))

	// Transform and add all fields
	attrs := fieldsToAttributes(e.Fields())
	record.AddAttributes(attrs...)

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

// Close stops observing capitan events.
func (co *capitanObserver) Close() {
	if co.observer != nil {
		co.observer.Close()
	}
	if co.tracesHandler != nil {
		co.tracesHandler.Close()
	}
}
