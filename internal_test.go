package aperture

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// mockLogger captures emitted log records for testing with synchronization support.
type mockLogger struct {
	embedded.Logger
	mu      sync.Mutex
	records []log.Record
	notify  chan struct{}
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		notify: make(chan struct{}, 100), // Buffered to avoid blocking
	}
}

func (m *mockLogger) Emit(_ context.Context, record log.Record) {
	m.mu.Lock()
	m.records = append(m.records, record)
	m.mu.Unlock()

	// Non-blocking notify
	select {
	case m.notify <- struct{}{}:
	default:
	}
}

func (m *mockLogger) Enabled(_ context.Context, _ log.EnabledParameters) bool {
	return true
}

// waitForRecords blocks until at least n records have been received or timeout expires.
func (m *mockLogger) waitForRecords(n int, timeout time.Duration) []log.Record {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		m.mu.Lock()
		count := len(m.records)
		if count >= n {
			result := make([]log.Record, len(m.records))
			copy(result, m.records)
			m.mu.Unlock()
			return result
		}
		m.mu.Unlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}

		select {
		case <-m.notify:
			// Check again
			continue
		case <-time.After(remaining):
			// Timeout
		}
	}

	m.mu.Lock()
	result := make([]log.Record, len(m.records))
	copy(result, m.records)
	m.mu.Unlock()
	return result
}

func (m *mockLogger) getRecords() []log.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]log.Record, len(m.records))
	copy(result, m.records)
	return result
}

// mockLoggerProvider returns our mock logger.
type mockLoggerProvider struct {
	embedded.LoggerProvider
	logger *mockLogger
}

func (m *mockLoggerProvider) Logger(_ string, _ ...log.LoggerOption) log.Logger {
	return m.logger
}

// findRecordWithSignal searches records for one with the given aperture.signal attribute.
func findRecordWithSignal(records []log.Record, signalName string) *log.Record {
	for i := range records {
		var found bool
		records[i].WalkAttributes(func(kv log.KeyValue) bool {
			if kv.Key == "aperture.signal" && kv.Value.AsString() == signalName {
				found = true
				return false
			}
			return true
		})
		if found {
			return &records[i]
		}
	}
	return nil
}

// getAttributeValue extracts an attribute value from a record by key.
func getAttributeValue(record *log.Record, key string) string {
	var value string
	record.WalkAttributes(func(kv log.KeyValue) bool {
		if kv.Key == key {
			value = kv.Value.AsString()
			return false
		}
		return true
	})
	return value
}

// --- Tests ---

func TestInternalObserver_EmitsEvents(t *testing.T) {
	logger := newMockLogger()
	io := newInternalObserver(logger)
	defer io.Close()

	ctx := context.Background()

	// Emit a diagnostic event
	io.emit(ctx, SignalTransformSkipped,
		internalFieldKey.Field("test_key"),
		internalFieldVariant.Field("custom.Type"),
		internalSignal.Field("test.signal"),
	)

	// Wait for the record with proper synchronization
	records := logger.waitForRecords(1, 2*time.Second)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	record := records[0]

	// Verify severity
	if record.Severity() != log.SeverityDebug {
		t.Errorf("expected SeverityDebug, got %v", record.Severity())
	}

	// Verify body is the signal description
	if record.Body().AsString() != SignalTransformSkipped.Description() {
		t.Errorf("expected body %q, got %q", SignalTransformSkipped.Description(), record.Body().AsString())
	}

	// Verify attributes
	if v := getAttributeValue(&record, "field_key"); v != "test_key" {
		t.Errorf("expected field_key = 'test_key', got %q", v)
	}
	if v := getAttributeValue(&record, "field_variant"); v != "custom.Type" {
		t.Errorf("expected field_variant = 'custom.Type', got %q", v)
	}
	if v := getAttributeValue(&record, "signal"); v != "test.signal" {
		t.Errorf("expected signal = 'test.signal', got %q", v)
	}
}

func TestInternalObserver_Close(t *testing.T) {
	logger := newMockLogger()
	io := newInternalObserver(logger)

	// Should not panic
	io.Close()

	// Double close should not panic
	io.Close()
}

func TestTransformSkipped_EmittedOnUnknownType(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit event with custom field type (not registered)
	customVariant := capitan.Variant("test.UnknownType")
	customKey := capitan.NewKey[struct{}]("unknown", customVariant)
	testSignal := capitan.NewSignal("test.signal", "Test signal")

	cap.Emit(ctx, testSignal, customKey.Field(struct{}{}))

	// Wait for records - need sufficient time for double async hop
	// We expect at least 2: one from the main log, one from internal diagnostic
	records := mockLog.waitForRecords(2, 2*time.Second)

	// Should have received a diagnostic event about the skipped field
	record := findRecordWithSignal(records, SignalTransformSkipped.Name())
	if record == nil {
		t.Fatal("expected SignalTransformSkipped to be emitted for unknown field type")
	}

	// Verify the diagnostic contains correct information
	if v := getAttributeValue(record, "field_key"); v != "unknown" {
		t.Errorf("expected field_key = 'unknown', got %q", v)
	}
	if v := getAttributeValue(record, "field_variant"); v != "test.UnknownType" {
		t.Errorf("expected field_variant = 'test.UnknownType', got %q", v)
	}
	if v := getAttributeValue(record, "signal"); v != "test.signal" {
		t.Errorf("expected signal = 'test.signal', got %q", v)
	}
}

func TestMetricValueMissing_EmittedOnMissingValue(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	// Configure a gauge metric that requires a value key
	valueKey := capitan.NewInt64Key("value")
	testSignal := capitan.NewSignal("test.metric.signal", "Test metric signal")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   testSignal,
				Name:     "test_gauge",
				Type:     MetricTypeGauge,
				ValueKey: valueKey,
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit event WITHOUT the required value key
	cap.Emit(ctx, testSignal) // Missing valueKey field

	// Wait for records - at least 2: main log + diagnostic
	records := mockLog.waitForRecords(2, 2*time.Second)

	// Should have received a diagnostic event about the missing value
	record := findRecordWithSignal(records, SignalMetricValueMissing.Name())
	if record == nil {
		t.Fatal("expected SignalMetricValueMissing to be emitted when value key is missing")
	}

	// Verify the diagnostic contains correct information
	if v := getAttributeValue(record, "signal"); v != "test.metric.signal" {
		t.Errorf("expected signal = 'test.metric.signal', got %q", v)
	}
	if v := getAttributeValue(record, "metric_name"); v != "test_gauge" {
		t.Errorf("expected metric_name = 'test_gauge', got %q", v)
	}
	if v := getAttributeValue(record, "value_key"); v != "value" {
		t.Errorf("expected value_key = 'value', got %q", v)
	}
}

func TestMetricValueMissing_NotEmittedWhenValuePresent(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	valueKey := capitan.NewInt64Key("value")
	testSignal := capitan.NewSignal("test.metric.signal", "Test metric signal")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal:   testSignal,
				Name:     "test_gauge",
				Type:     MetricTypeGauge,
				ValueKey: valueKey,
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit event WITH the required value key
	cap.Emit(ctx, testSignal, valueKey.Field(int64(42)))

	// Brief wait to allow any async processing
	time.Sleep(50 * time.Millisecond)

	// Should NOT have any diagnostic records
	records := mockLog.getRecords()
	record := findRecordWithSignal(records, SignalMetricValueMissing.Name())
	if record != nil {
		t.Error("did not expect SignalMetricValueMissing when value is present")
	}
}

func TestTraceExpired_EmittedOnTimeout(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	startSignal := capitan.NewSignal("test.span.start", "Span start")
	endSignal := capitan.NewSignal("test.span.end", "Span end")
	correlationKey := capitan.NewStringKey("trace_id")

	// Use a very short timeout for testing
	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          startSignal,
				End:            endSignal,
				CorrelationKey: &correlationKey,
				SpanName:       "test-span",
				SpanTimeout:    50 * time.Millisecond, // Very short for testing
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit only the start event (no matching end)
	cap.Emit(ctx, startSignal, correlationKey.Field("test-correlation-123"))

	// Wait for cleanup to trigger (cleanup runs every 1 minute by default)
	// We need to trigger it manually or wait - let's trigger manually
	sh.capitanObserver.tracesHandler.cleanupStaleSpans()

	// That won't work yet because the timeout hasn't passed - wait for it
	time.Sleep(100 * time.Millisecond)

	// Now trigger cleanup again
	sh.capitanObserver.tracesHandler.cleanupStaleSpans()

	// Wait for diagnostic record - at least 2: main log + diagnostic
	records := mockLog.waitForRecords(2, 2*time.Second)

	// Should have received a diagnostic event about the expired span
	record := findRecordWithSignal(records, SignalTraceExpired.Name())
	if record == nil {
		t.Fatal("expected SignalTraceExpired to be emitted when span times out")
	}

	// Verify the diagnostic contains correct information
	if v := getAttributeValue(record, "correlation_id"); v != "test-correlation-123" {
		t.Errorf("expected correlation_id = 'test-correlation-123', got %q", v)
	}
	if v := getAttributeValue(record, "span_name"); v != "test-span" {
		t.Errorf("expected span_name = 'test-span', got %q", v)
	}
	if v := getAttributeValue(record, "reason"); v != "end event not received" {
		t.Errorf("expected reason = 'end event not received', got %q", v)
	}
}

func TestTraceExpired_PendingEnd(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	startSignal := capitan.NewSignal("test.span.start", "Span start")
	endSignal := capitan.NewSignal("test.span.end", "Span end")
	correlationKey := capitan.NewStringKey("trace_id")

	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          startSignal,
				End:            endSignal,
				CorrelationKey: &correlationKey,
				SpanName:       "test-span",
				SpanTimeout:    50 * time.Millisecond,
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit only the END event (no matching start - out of order arrival that never completes)
	cap.Emit(ctx, endSignal, correlationKey.Field("orphan-end-456"))

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)
	sh.capitanObserver.tracesHandler.cleanupStaleSpans()

	// Wait for records - at least 2: main log + diagnostic
	records := mockLog.waitForRecords(2, 2*time.Second)

	record := findRecordWithSignal(records, SignalTraceExpired.Name())
	if record == nil {
		t.Fatal("expected SignalTraceExpired to be emitted for orphaned end event")
	}

	if v := getAttributeValue(record, "reason"); v != "start event not received" {
		t.Errorf("expected reason = 'start event not received', got %q", v)
	}
}

func TestTraceExpired_NotEmittedWhenMatched(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	startSignal := capitan.NewSignal("test.span.start", "Span start")
	endSignal := capitan.NewSignal("test.span.end", "Span end")
	correlationKey := capitan.NewStringKey("trace_id")

	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          startSignal,
				End:            endSignal,
				CorrelationKey: &correlationKey,
				SpanName:       "test-span",
				SpanTimeout:    50 * time.Millisecond,
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit matching start and end events
	cap.Emit(ctx, startSignal, correlationKey.Field("matched-789"))
	cap.Emit(ctx, endSignal, correlationKey.Field("matched-789"))

	// Wait and trigger cleanup
	time.Sleep(100 * time.Millisecond)
	sh.capitanObserver.tracesHandler.cleanupStaleSpans()

	// Brief wait
	time.Sleep(50 * time.Millisecond)

	// Should NOT have any trace expired diagnostics
	records := mockLog.getRecords()
	record := findRecordWithSignal(records, SignalTraceExpired.Name())
	if record != nil {
		t.Error("did not expect SignalTraceExpired when start and end are matched")
	}
}

func TestAllMetricTypes_EmitDiagnosticOnMissingValue(t *testing.T) {
	testCases := []struct {
		name       string
		metricType MetricType
	}{
		{"UpDownCounter", MetricTypeUpDownCounter},
		{"Gauge", MetricTypeGauge},
		{"Histogram", MetricTypeHistogram},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			cap := capitan.New()

			mockLog := newMockLogger()
			provider := &mockLoggerProvider{logger: mockLog}

			valueKey := capitan.NewInt64Key("value")
			testSignal := capitan.NewSignal("test."+tc.name, "Test "+tc.name)

			config := &Config{
				Metrics: []MetricConfig{
					{
						Signal:   testSignal,
						Name:     "test_" + tc.name,
						Type:     tc.metricType,
						ValueKey: valueKey,
					},
				},
			}

			sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
			if err != nil {
				t.Fatalf("failed to create Aperture: %v", err)
			}
			defer sh.Close()

			// Emit without value
			cap.Emit(ctx, testSignal)

			// Wait for at least 2 records: main log + diagnostic
			records := mockLog.waitForRecords(2, 2*time.Second)

			record := findRecordWithSignal(records, SignalMetricValueMissing.Name())
			if record == nil {
				t.Fatalf("expected SignalMetricValueMissing for %s", tc.name)
			}
		})
	}
}

func TestCounter_NoDiagnosticEmitted(t *testing.T) {
	// Counters don't require a value key, so no diagnostic should be emitted
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	testSignal := capitan.NewSignal("test.counter", "Test counter")

	config := &Config{
		Metrics: []MetricConfig{
			{
				Signal: testSignal,
				Name:   "test_counter",
				Type:   MetricTypeCounter,
				// No ValueKey needed for counter
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit counter event
	cap.Emit(ctx, testSignal)

	// Brief wait
	time.Sleep(50 * time.Millisecond)

	// Should NOT have any metric value missing diagnostics
	records := mockLog.getRecords()
	record := findRecordWithSignal(records, SignalMetricValueMissing.Name())
	if record != nil {
		t.Error("did not expect SignalMetricValueMissing for counter (no value key required)")
	}
}

func TestTraceCorrelationMissing_EmittedOnMissingID(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	startSignal := capitan.NewSignal("test.span.start", "Span start")
	endSignal := capitan.NewSignal("test.span.end", "Span end")
	correlationKey := capitan.NewStringKey("trace_id")

	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          startSignal,
				End:            endSignal,
				CorrelationKey: &correlationKey,
				SpanName:       "test-span",
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit start event WITHOUT the correlation key
	cap.Emit(ctx, startSignal) // Missing trace_id field

	// Wait for records - at least 2: main log + diagnostic
	records := mockLog.waitForRecords(2, 2*time.Second)

	// Should have received a diagnostic event about the missing correlation ID
	record := findRecordWithSignal(records, SignalTraceCorrelationMissing.Name())
	if record == nil {
		t.Fatal("expected SignalTraceCorrelationMissing to be emitted when correlation key is missing")
	}

	// Verify the diagnostic contains correct information
	if v := getAttributeValue(record, "signal"); v != "test.span.start" {
		t.Errorf("expected signal = 'test.span.start', got %q", v)
	}
	if v := getAttributeValue(record, "span_name"); v != "test-span" {
		t.Errorf("expected span_name = 'test-span', got %q", v)
	}
	if v := getAttributeValue(record, "correlation_key"); v != "trace_id" {
		t.Errorf("expected correlation_key = 'trace_id', got %q", v)
	}
}

func TestTraceCorrelationMissing_EmittedForEndEvent(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	startSignal := capitan.NewSignal("test.span.start", "Span start")
	endSignal := capitan.NewSignal("test.span.end", "Span end")
	correlationKey := capitan.NewStringKey("trace_id")

	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          startSignal,
				End:            endSignal,
				CorrelationKey: &correlationKey,
				SpanName:       "test-span",
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit end event WITHOUT the correlation key
	cap.Emit(ctx, endSignal) // Missing trace_id field

	// Wait for records
	records := mockLog.waitForRecords(2, 2*time.Second)

	record := findRecordWithSignal(records, SignalTraceCorrelationMissing.Name())
	if record == nil {
		t.Fatal("expected SignalTraceCorrelationMissing to be emitted for end event missing correlation key")
	}

	if v := getAttributeValue(record, "signal"); v != "test.span.end" {
		t.Errorf("expected signal = 'test.span.end', got %q", v)
	}
}

func TestTraceCorrelationMissing_NotEmittedWhenPresent(t *testing.T) {
	ctx := context.Background()
	cap := capitan.New()

	mockLog := newMockLogger()
	provider := &mockLoggerProvider{logger: mockLog}

	startSignal := capitan.NewSignal("test.span.start", "Span start")
	endSignal := capitan.NewSignal("test.span.end", "Span end")
	correlationKey := capitan.NewStringKey("trace_id")

	config := &Config{
		Traces: []TraceConfig{
			{
				Start:          startSignal,
				End:            endSignal,
				CorrelationKey: &correlationKey,
				SpanName:       "test-span",
			},
		},
	}

	sh, err := New(cap, provider, metricnoop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		t.Fatalf("failed to create Aperture: %v", err)
	}
	defer sh.Close()

	// Emit events WITH the correlation key
	cap.Emit(ctx, startSignal, correlationKey.Field("has-id"))
	cap.Emit(ctx, endSignal, correlationKey.Field("has-id"))

	// Brief wait
	time.Sleep(50 * time.Millisecond)

	// Should NOT have any correlation missing diagnostics
	records := mockLog.getRecords()
	record := findRecordWithSignal(records, SignalTraceCorrelationMissing.Name())
	if record != nil {
		t.Error("did not expect SignalTraceCorrelationMissing when correlation key is present")
	}
}

func TestInternalSignals_Defined(t *testing.T) {
	signals := []struct {
		signal      capitan.Signal
		name        string
		description string
	}{
		{SignalTransformSkipped, "aperture:transform:skipped", "field transformation skipped due to unsupported type"},
		{SignalTraceExpired, "aperture:trace:expired", "pending span expired without matching start/end"},
		{SignalMetricValueMissing, "aperture:metric:value_missing", "metric value could not be extracted from event"},
		{SignalTraceCorrelationMissing, "aperture:trace:correlation_missing", "trace event missing correlation ID field"},
	}

	for _, s := range signals {
		if s.signal.Name() != s.name {
			t.Errorf("signal name = %q, want %q", s.signal.Name(), s.name)
		}
		if s.signal.Description() != s.description {
			t.Errorf("signal description = %q, want %q", s.signal.Description(), s.description)
		}
	}
}

func TestInternalFieldKeys_Defined(t *testing.T) {
	keys := []struct {
		key  capitan.StringKey
		name string
	}{
		{internalFieldKey, "field_key"},
		{internalFieldVariant, "field_variant"},
		{internalSignal, "signal"},
		{internalReason, "reason"},
		{internalCorrelationID, "correlation_id"},
		{internalSpanName, "span_name"},
		{internalMetricName, "metric_name"},
		{internalValueKey, "value_key"},
		{internalCorrelationKey, "correlation_key"},
	}

	for _, k := range keys {
		if k.key.Name() != k.name {
			t.Errorf("key name = %q, want %q", k.key.Name(), k.name)
		}
	}
}
