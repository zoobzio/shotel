// Package testing provides test utilities and helpers for aperture users.
// These utilities help users test their own aperture-based applications.
package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
)

// Providers holds OTEL providers for testing.
// Pass the individual providers to aperture.New().
type Providers struct {
	Log   *sdklog.LoggerProvider
	Meter *sdkmetric.MeterProvider
	Trace *sdktrace.TracerProvider
}

// Shutdown gracefully shuts down all providers.
func (p *Providers) Shutdown(ctx context.Context) error {
	var errs []error

	if p.Trace != nil {
		if err := p.Trace.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("trace provider: %w", err))
		}
	}

	if p.Meter != nil {
		if err := p.Meter.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider: %w", err))
		}
	}

	if p.Log != nil {
		if err := p.Log.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("log provider: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// TestProviders creates OTLP providers configured for testing.
// Uses insecure connections suitable for local OTLP collectors.
//
// Example:
//
//	pvs, err := testing.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
//	if err != nil {
//	    t.Fatal(err)
//	}
//	defer pvs.Shutdown(ctx)
//
//	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
//	if err != nil {
//	    t.Fatal(err)
//	}
//	defer ap.Close()
func TestProviders(ctx context.Context, serviceName, serviceVersion, otlpEndpoint string) (*Providers, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if serviceVersion == "" {
		return nil, fmt.Errorf("service version is required")
	}
	if otlpEndpoint == "" {
		return nil, fmt.Errorf("OTLP endpoint is required")
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(otlpEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating log exporter: %w", err)
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)

	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(otlpEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		_ = logProvider.Shutdown(ctx) //nolint:errcheck // best-effort cleanup
		return nil, fmt.Errorf("creating metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(60*time.Second),
		)),
	)

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		_ = logProvider.Shutdown(ctx)   //nolint:errcheck // best-effort cleanup
		_ = meterProvider.Shutdown(ctx) //nolint:errcheck // best-effort cleanup
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(traceExporter)),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	return &Providers{
		Log:   logProvider,
		Meter: meterProvider,
		Trace: traceProvider,
	}, nil
}

// LogCapture captures OTEL log records for testing and verification.
// Thread-safe for concurrent log capture.
type LogCapture struct {
	notify  chan struct{} // channel pointer first
	records []log.Record  // slice (pointer in first 8 bytes)
	mu      sync.Mutex    // no pointers
}

// NewLogCapture creates a new LogCapture instance.
func NewLogCapture() *LogCapture {
	return &LogCapture{
		records: make([]log.Record, 0),
		notify:  make(chan struct{}, 100),
	}
}

// Records returns a copy of all captured log records.
func (lc *LogCapture) Records() []log.Record {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	result := make([]log.Record, len(lc.records))
	copy(result, lc.records)
	return result
}

// Count returns the number of captured records.
func (lc *LogCapture) Count() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return len(lc.records)
}

// Reset clears all captured records.
func (lc *LogCapture) Reset() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.records = lc.records[:0]
}

// WaitForCount blocks until the capture has at least n records or timeout occurs.
// Returns true if count reached, false if timeout.
func (lc *LogCapture) WaitForCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if lc.Count() >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// MockLogger is a mock OTEL logger that captures records for testing.
type MockLogger struct {
	embedded.Logger
	capture *LogCapture
}

// NewMockLogger creates a new MockLogger with its own capture.
func NewMockLogger() *MockLogger {
	return &MockLogger{
		capture: NewLogCapture(),
	}
}

// Emit captures the log record.
func (m *MockLogger) Emit(_ context.Context, record log.Record) {
	m.capture.mu.Lock()
	m.capture.records = append(m.capture.records, record)
	m.capture.mu.Unlock()

	select {
	case m.capture.notify <- struct{}{}:
	default:
	}
}

// Enabled returns true.
func (*MockLogger) Enabled(_ context.Context, _ log.EnabledParameters) bool {
	return true
}

// Capture returns the underlying LogCapture for assertions.
func (m *MockLogger) Capture() *LogCapture {
	return m.capture
}

// MockLoggerProvider provides MockLoggers for testing.
type MockLoggerProvider struct {
	embedded.LoggerProvider
	logger *MockLogger
}

// NewMockLoggerProvider creates a new MockLoggerProvider.
func NewMockLoggerProvider() *MockLoggerProvider {
	return &MockLoggerProvider{
		logger: NewMockLogger(),
	}
}

// Logger returns the mock logger.
func (p *MockLoggerProvider) Logger(_ string, _ ...log.LoggerOption) log.Logger {
	return p.logger
}

// Capture returns the underlying LogCapture for assertions.
func (p *MockLoggerProvider) Capture() *LogCapture {
	return p.logger.capture
}

// EventCapture wraps capitan's event capture for aperture testing.
// Captures capitan events for verification.
type EventCapture struct {
	events []CapturedEvent
	mu     sync.Mutex
}

// CapturedEvent represents a snapshot of a capitan event.
type CapturedEvent struct {
	Signal    capitan.Signal
	Timestamp time.Time
	Severity  capitan.Severity
	Fields    []capitan.Field
}

// NewEventCapture creates a new EventCapture instance.
func NewEventCapture() *EventCapture {
	return &EventCapture{
		events: make([]CapturedEvent, 0),
	}
}

// Handler returns an EventCallback that captures events.
func (ec *EventCapture) Handler() capitan.EventCallback {
	return func(_ context.Context, e *capitan.Event) {
		ec.mu.Lock()
		defer ec.mu.Unlock()
		ec.events = append(ec.events, CapturedEvent{
			Signal:    e.Signal(),
			Timestamp: e.Timestamp(),
			Severity:  e.Severity(),
			Fields:    e.Fields(),
		})
	}
}

// Events returns a copy of all captured events.
func (ec *EventCapture) Events() []CapturedEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	result := make([]CapturedEvent, len(ec.events))
	copy(result, ec.events)
	return result
}

// Count returns the number of captured events.
func (ec *EventCapture) Count() int {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.events)
}

// Reset clears all captured events.
func (ec *EventCapture) Reset() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.events = ec.events[:0]
}

// WaitForCount blocks until the capture has at least n events or timeout occurs.
func (ec *EventCapture) WaitForCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ec.Count() >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}
