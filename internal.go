package aperture

import (
	"context"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

// Diagnostic signals emitted by Aperture for operational visibility.
//
// These signals are written to the OTEL logger at DEBUG severity with a
// "aperture.signal" attribute containing the signal name. They help diagnose
// configuration issues and unexpected runtime conditions.
//
// Filter for these in your log aggregator using:
//
//	aperture.signal = "aperture:*"
var (
	// SignalTraceExpired is emitted when a span's start or end event was received
	// but the matching counterpart never arrived within the configured span timeout.
	//
	// Attributes:
	//   - correlation_id: The correlation ID that was never matched
	//   - span_name: The configured span name
	//   - reason: Either "end event not received" or "start event not received"
	//
	// Resolution: Check that both start and end signals are being emitted with
	// matching correlation IDs, or increase span_timeout for long-running operations.
	SignalTraceExpired = capitan.NewSignal("aperture:trace:expired", "pending span expired without matching start/end")

	// SignalMetricValueMissing is emitted when a metric that requires a value
	// (gauge, histogram, updowncounter) receives an event without the configured
	// value_key field.
	//
	// Attributes:
	//   - signal: The originating capitan signal name
	//   - metric_name: The OTEL metric name
	//   - value_key: The expected field key name
	//
	// Resolution: Ensure the signal is emitted with the required value field.
	SignalMetricValueMissing = capitan.NewSignal("aperture:metric:value_missing", "metric value could not be extracted from event")

	// SignalTraceCorrelationMissing is emitted when a trace start or end event
	// lacks the correlation_key field required to match spans.
	//
	// Attributes:
	//   - signal: The originating capitan signal name
	//   - span_name: The configured span name
	//   - correlation_key: The expected field key name
	//
	// Resolution: Ensure trace events include the correlation key field.
	SignalTraceCorrelationMissing = capitan.NewSignal("aperture:trace:correlation_missing", "trace event missing correlation ID field")
)

// Internal field keys for diagnostic events.
var (
	internalSignal         = capitan.NewStringKey("signal")
	internalReason         = capitan.NewStringKey("reason")
	internalCorrelationID  = capitan.NewStringKey("correlation_id")
	internalSpanName       = capitan.NewStringKey("span_name")
	internalMetricName     = capitan.NewStringKey("metric_name")
	internalValueKey       = capitan.NewStringKey("value_key")
	internalCorrelationKey = capitan.NewStringKey("correlation_key")
)

// internalObserver handles Aperture's private diagnostic events.
// It writes directly to the OTEL logger without field transformation.
type internalObserver struct {
	capitan  *capitan.Capitan
	observer *capitan.Observer
	logger   log.Logger
}

// newInternalObserver creates the internal diagnostic system.
func newInternalObserver(logger log.Logger) *internalObserver {
	internal := capitan.New()

	io := &internalObserver{
		capitan: internal,
		logger:  logger,
	}

	io.observer = internal.Observe(io.handleEvent)

	return io
}

// handleEvent writes internal diagnostic events directly to OTEL.
// No field transformation is performed to avoid recursion.
func (io *internalObserver) handleEvent(ctx context.Context, e *capitan.Event) {
	var record log.Record

	record.SetTimestamp(e.Timestamp())
	record.SetSeverity(log.SeverityDebug)
	record.SetSeverityText("DEBUG")
	record.SetBody(log.StringValue(e.Signal().Description()))

	// Add signal identifier
	record.AddAttributes(log.String("aperture.signal", e.Signal().Name()))

	// Convert fields directly (hardcoded string fields only)
	for _, f := range e.Fields() {
		if gf, ok := f.(capitan.GenericField[string]); ok {
			record.AddAttributes(log.String(f.Key().Name(), gf.Get()))
		}
	}

	io.logger.Emit(ctx, record)
}

// emit emits an internal diagnostic event.
func (io *internalObserver) emit(ctx context.Context, signal capitan.Signal, fields ...capitan.Field) {
	io.capitan.Emit(ctx, signal, fields...)
}

// Close stops the internal observer.
func (io *internalObserver) Close() {
	if io.observer != nil {
		io.observer.Close()
	}
}
