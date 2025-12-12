package benchmarks

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/aperture"
	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// BenchmarkEmit_NoConfig benchmarks event emission with no aperture configuration.
func BenchmarkEmit_NoConfig(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.noconfig", "Benchmark signal")
	key := capitan.NewStringKey("key")

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig, key.Field("value"))
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkEmit_WithMetricsCounter benchmarks event emission with counter metric.
func BenchmarkEmit_WithMetricsCounter(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.counter", "Benchmark counter signal")
	key := capitan.NewStringKey("key")

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal: sig,
				Name:   "bench_counter_total",
				Type:   aperture.MetricTypeCounter,
			},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig, key.Field("value"))
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkEmit_WithMetricsHistogram benchmarks event emission with histogram metric.
func BenchmarkEmit_WithMetricsHistogram(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.histogram", "Benchmark histogram signal")
	durationKey := capitan.NewDurationKey("duration")

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal:   sig,
				Name:     "bench_duration_ms",
				Type:     aperture.MetricTypeHistogram,
				ValueKey: durationKey,
			},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	duration := 100 * time.Millisecond

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig, durationKey.Field(duration))
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkEmit_WithLogs benchmarks event emission with log recording.
func BenchmarkEmit_WithLogs(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.logs", "Benchmark log signal")
	key := capitan.NewStringKey("key")

	config := &aperture.Config{
		Logs: &aperture.LogConfig{
			Whitelist: []capitan.Signal{sig},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig, key.Field("value"))
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkEmit_MultipleFields benchmarks event emission with multiple fields.
func BenchmarkEmit_MultipleFields(b *testing.B) {
	ctx := context.Background()

	sig := capitan.NewSignal("bench.fields", "Benchmark multi-field signal")
	strKey := capitan.NewStringKey("str")
	intKey := capitan.NewIntKey("int")
	floatKey := capitan.NewFloat64Key("float")
	boolKey := capitan.NewBoolKey("bool")

	benchCases := []struct {
		name   string
		fields []capitan.Field
	}{
		{"1_field", []capitan.Field{strKey.Field("value")}},
		{"2_fields", []capitan.Field{strKey.Field("value"), intKey.Field(42)}},
		{"4_fields", []capitan.Field{
			strKey.Field("value"),
			intKey.Field(42),
			floatKey.Field(3.14),
			boolKey.Field(true),
		}},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			cap := capitan.New()
			defer cap.Shutdown()

			mockLog := apertesting.NewMockLoggerProvider()
			ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
			if err != nil {
				b.Fatalf("failed to create aperture: %v", err)
			}
			defer ap.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				cap.Emit(ctx, sig, bc.fields...)
			}

			b.StopTimer()
			cap.Shutdown()
		})
	}
}

// BenchmarkEmit_Combined benchmarks event emission with metrics, logs, and traces.
func BenchmarkEmit_Combined(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.combined", "Benchmark combined signal")
	key := capitan.NewStringKey("key")

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{Signal: sig, Name: "bench_combined_total", Type: aperture.MetricTypeCounter},
		},
		Logs: &aperture.LogConfig{
			Whitelist: []capitan.Signal{sig},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig, key.Field("value"))
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkTraceCorrelation benchmarks trace start/end correlation.
func BenchmarkTraceCorrelation(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	reqStarted := capitan.NewSignal("bench.trace.start", "Benchmark trace start")
	reqEnded := capitan.NewSignal("bench.trace.end", "Benchmark trace end")
	requestID := capitan.NewStringKey("request_id")

	config := &aperture.Config{
		Traces: []aperture.TraceConfig{
			{
				Start:          reqStarted,
				End:            reqEnded,
				CorrelationKey: &requestID,
				SpanName:       "bench_span",
			},
		},
	}

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), config)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reqID := "REQ-" + string(rune('A'+i%26))
		cap.Emit(ctx, reqStarted, requestID.Field(reqID))
		cap.Emit(ctx, reqEnded, requestID.Field(reqID))
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkLogCapture benchmarks the log capture helper.
func BenchmarkLogCapture(b *testing.B) {
	capture := apertesting.NewLogCapture()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = capture.Count()
	}
}

// BenchmarkEventCapture benchmarks the event capture helper.
func BenchmarkEventCapture(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.capture", "Benchmark capture signal")

	capture := apertesting.NewEventCapture()
	cap.Hook(sig, capture.Handler())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig)
	}

	b.StopTimer()
	cap.Shutdown()
}

// BenchmarkMockLoggerEmit benchmarks the mock logger emit path.
func BenchmarkMockLoggerEmit(b *testing.B) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("bench.mock", "Benchmark mock logger")

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
	if err != nil {
		b.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cap.Emit(ctx, sig)
	}

	b.StopTimer()
	cap.Shutdown()
}
