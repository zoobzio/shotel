package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/aperture"
	apertesting "github.com/zoobzio/aperture/testing"
	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestConcurrency_MultipleGoroutinesEmitting(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("concurrent.emit", "Concurrent emission test")
	counterKey := capitan.NewIntKey("counter")

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{
				Signal: sig,
				Name:   "concurrent_emits_total",
				Type:   aperture.MetricTypeCounter,
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	// Launch multiple goroutines emitting concurrently
	const goroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				cap.Emit(ctx, sig, counterKey.Field(goroutineID*eventsPerGoroutine+i))
			}
		}(g)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

func TestConcurrency_TraceCorrelationUnderLoad(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	reqStarted := capitan.NewSignal("request.started", "Request started")
	reqCompleted := capitan.NewSignal("request.completed", "Request completed")
	requestID := capitan.NewStringKey("request_id")

	config := &aperture.Config{
		Traces: []aperture.TraceConfig{
			{
				Start:          reqStarted,
				End:            reqCompleted,
				CorrelationKey: &requestID,
				SpanName:       "http_request",
			},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	// Launch multiple goroutines creating overlapping traces
	const requests = 50

	var wg sync.WaitGroup
	wg.Add(requests)

	for i := 0; i < requests; i++ {
		go func(reqNum int) {
			defer wg.Done()
			reqID := "REQ-" + string(rune('A'+reqNum%26)) + string(rune('0'+reqNum/26))

			cap.Emit(ctx, reqStarted, requestID.Field(reqID))
			time.Sleep(time.Duration(reqNum%10) * time.Millisecond) // Variable delay
			cap.Emit(ctx, reqCompleted, requestID.Field(reqID))
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

func TestConcurrency_CreateAndClose(t *testing.T) {
	ctx := context.Background()

	// Test creating and closing aperture instances concurrently
	const iterations = 20

	var wg sync.WaitGroup
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()

			cap := capitan.New()
			defer cap.Shutdown()

			sig := capitan.NewSignal("lifecycle.test", "Lifecycle test signal")

			mockLog := apertesting.NewMockLoggerProvider()
			ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
			if err != nil {
				t.Errorf("failed to create aperture: %v", err)
				return
			}

			// Emit a few events
			for j := 0; j < 10; j++ {
				cap.Emit(ctx, sig)
			}

			ap.Close()
		}()
	}

	wg.Wait()
}

func TestConcurrency_MixedOperations(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig1 := capitan.NewSignal("mixed.signal1", "Mixed signal 1")
	sig2 := capitan.NewSignal("mixed.signal2", "Mixed signal 2")
	reqStarted := capitan.NewSignal("mixed.started", "Mixed request started")
	reqEnded := capitan.NewSignal("mixed.ended", "Mixed request ended")
	requestID := capitan.NewStringKey("request_id")
	valueKey := capitan.NewFloat64Key("value")

	config := &aperture.Config{
		Metrics: []aperture.MetricConfig{
			{Signal: sig1, Name: "mixed_counter", Type: aperture.MetricTypeCounter},
			{Signal: sig2, Name: "mixed_gauge", Type: aperture.MetricTypeGauge, ValueKey: valueKey},
		},
		Traces: []aperture.TraceConfig{
			{Start: reqStarted, End: reqEnded, CorrelationKey: &requestID, SpanName: "mixed"},
		},
		Logs: &aperture.LogConfig{
			Whitelist: []capitan.Signal{sig1, sig2},
		},
	}

	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("failed to create providers: %v", err)
	}

	ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	// Mix of different operations concurrently
	var wg sync.WaitGroup

	// Counter emissions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			cap.Emit(ctx, sig1)
		}
	}()

	// Gauge emissions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			cap.Emit(ctx, sig2, valueKey.Field(float64(i)))
		}
	}()

	// Trace pairs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			reqID := "MIX-" + string(rune('A'+i))
			cap.Emit(ctx, reqStarted, requestID.Field(reqID))
			cap.Emit(ctx, reqEnded, requestID.Field(reqID))
		}
	}()

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

func TestConcurrency_HighVolumeLogging(t *testing.T) {
	ctx := context.Background()

	cap := capitan.New()
	defer cap.Shutdown()

	sig := capitan.NewSignal("highvolume.log", "High volume logging test")

	mockLog := apertesting.NewMockLoggerProvider()
	ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), nil)
	if err != nil {
		t.Fatalf("failed to create aperture: %v", err)
	}
	defer ap.Close()

	const goroutines = 5
	const eventsPerGoroutine = 200
	expectedTotal := goroutines * eventsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				cap.Emit(ctx, sig)
			}
		}()
	}

	wg.Wait()
	cap.Shutdown()

	// Verify all events were captured
	records := mockLog.Capture().Records()
	if len(records) != expectedTotal {
		t.Errorf("expected %d log records, got %d", expectedTotal, len(records))
	}
}
