package shotel

import (
	"context"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
)

// mockObservable implements Observable for testing.
type mockObservable struct {
	registry *metricz.Registry
}

func (m *mockObservable) Metrics() *metricz.Registry {
	return m.registry
}

func TestShotelMetricsObservation(t *testing.T) {
	// Create a test observable with metrics
	registry := metricz.New()
	observable := &mockObservable{registry: registry}

	// Define test metric keys
	const (
		TestCounter   = metricz.Key("test.counter")
		TestGauge     = metricz.Key("test.gauge")
		TestHistogram = metricz.Key("test.histogram")
		TestTimer     = metricz.Key("test.timer")
	)

	// Create metrics
	counter := registry.Counter(TestCounter)
	gauge := registry.Gauge(TestGauge)
	histogram := registry.Histogram(TestHistogram, []float64{1, 5, 10, 50, 100})
	timer := registry.Timer(TestTimer)

	// Set initial values
	counter.Inc()
	counter.Add(5)
	gauge.Set(42.5)
	histogram.Observe(7.3)
	histogram.Observe(25.0)
	stop := timer.Start()
	time.Sleep(10 * time.Millisecond)
	stop.Stop()

	// Create Shotel instance with in-memory exporters
	ctx := context.Background()
	cfg := DefaultConfig("test-service")
	cfg.Endpoint = "" // Empty endpoint triggers in-memory mode
	cfg.MetricsInterval = 100 * time.Millisecond

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	// Observe metrics
	shotel.ObserveMetrics(observable, TestCounter, TestGauge, TestHistogram, TestTimer)

	// Wait for at least one polling cycle
	time.Sleep(200 * time.Millisecond)

	// Verify observations were registered
	shotel.mu.Lock()
	if len(shotel.observations) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(shotel.observations))
	}

	obs := shotel.observations[0]
	if obs.observable != observable {
		t.Error("Observable mismatch")
	}

	// Verify metric mapping
	if !obs.mapping.counters[TestCounter] {
		t.Error("Counter not mapped")
	}
	if !obs.mapping.gauges[TestGauge] {
		t.Error("Gauge not mapped")
	}
	if !obs.mapping.histograms[TestHistogram] {
		t.Error("Histogram not mapped")
	}
	if !obs.mapping.timers[TestTimer] {
		t.Error("Timer not mapped")
	}
	shotel.mu.Unlock()

	// Update metrics and wait for another poll
	counter.Add(3)
	gauge.Set(100.0)
	histogram.Observe(42.0)

	time.Sleep(150 * time.Millisecond)

	// Verify metrics were updated
	if counter.Value() != 9.0 {
		t.Errorf("Counter value = %f, want 9.0", counter.Value())
	}
	if gauge.Value() != 100.0 {
		t.Errorf("Gauge value = %f, want 100.0", gauge.Value())
	}
	if histogram.Count() != 3 {
		t.Errorf("Histogram count = %d, want 3", histogram.Count())
	}
}

func TestShotelCounterMetrics(t *testing.T) {
	registry := metricz.New()
	observable := &mockObservable{registry: registry}

	const CounterKey = metricz.Key("test.requests.total")
	counter := registry.Counter(CounterKey)

	ctx := context.Background()
	cfg := DefaultConfig("counter-test")
	cfg.Endpoint = ""
	cfg.MetricsInterval = 50 * time.Millisecond

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	shotel.ObserveMetrics(observable, CounterKey)

	// Increment counter multiple times
	counter.Inc()
	counter.Add(10)
	counter.Inc()

	time.Sleep(100 * time.Millisecond)

	if counter.Value() != 12.0 {
		t.Errorf("Counter value = %f, want 12.0", counter.Value())
	}
}

func TestShotelGaugeMetrics(t *testing.T) {
	registry := metricz.New()
	observable := &mockObservable{registry: registry}

	const GaugeKey = metricz.Key("test.memory.bytes")
	gauge := registry.Gauge(GaugeKey)

	ctx := context.Background()
	cfg := DefaultConfig("gauge-test")
	cfg.MetricsInterval = 50 * time.Millisecond
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	shotel.ObserveMetrics(observable, GaugeKey)

	// Set gauge values
	gauge.Set(1024)
	time.Sleep(100 * time.Millisecond)

	gauge.Add(512)
	time.Sleep(100 * time.Millisecond)

	gauge.Add(-256)
	time.Sleep(100 * time.Millisecond)

	if gauge.Value() != 1280.0 {
		t.Errorf("Gauge value = %f, want 1280.0", gauge.Value())
	}
}

func TestShotelHistogramMetrics(t *testing.T) {
	registry := metricz.New()
	observable := &mockObservable{registry: registry}

	const HistKey = metricz.Key("test.response.size")
	histogram := registry.Histogram(HistKey, []float64{100, 500, 1000, 5000})

	ctx := context.Background()
	cfg := DefaultConfig("histogram-test")
	cfg.MetricsInterval = 50 * time.Millisecond
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	shotel.ObserveMetrics(observable, HistKey)

	// Observe values
	histogram.Observe(250)
	histogram.Observe(750)
	histogram.Observe(1500)

	time.Sleep(100 * time.Millisecond)

	if histogram.Count() != 3 {
		t.Errorf("Histogram count = %d, want 3", histogram.Count())
	}

	sum := histogram.Sum()
	expectedSum := 250.0 + 750.0 + 1500.0
	if sum != expectedSum {
		t.Errorf("Histogram sum = %f, want %f", sum, expectedSum)
	}
}

func TestShotelTimerMetrics(t *testing.T) {
	registry := metricz.New()
	observable := &mockObservable{registry: registry}

	const TimerKey = metricz.Key("test.request.duration")
	timer := registry.Timer(TimerKey)

	ctx := context.Background()
	cfg := DefaultConfig("timer-test")
	cfg.MetricsInterval = 50 * time.Millisecond
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	shotel.ObserveMetrics(observable, TimerKey)

	// Record durations
	timer.Record(10 * time.Millisecond)
	timer.Record(25 * time.Millisecond)
	timer.Record(50 * time.Millisecond)

	time.Sleep(100 * time.Millisecond)

	if timer.Count() != 3 {
		t.Errorf("Timer count = %d, want 3", timer.Count())
	}
}

func TestShotelMultipleObservables(t *testing.T) {
	// Create two observables
	registry1 := metricz.New()
	observable1 := &mockObservable{registry: registry1}

	registry2 := metricz.New()
	observable2 := &mockObservable{registry: registry2}

	const (
		Key1 = metricz.Key("observable1.counter")
		Key2 = metricz.Key("observable2.counter")
	)

	counter1 := registry1.Counter(Key1)
	counter2 := registry2.Counter(Key2)

	ctx := context.Background()
	cfg := DefaultConfig("multi-test")
	cfg.MetricsInterval = 50 * time.Millisecond
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	// Observe both
	shotel.ObserveMetrics(observable1, Key1)
	shotel.ObserveMetrics(observable2, Key2)

	counter1.Add(5)
	counter2.Add(10)

	time.Sleep(100 * time.Millisecond)

	shotel.mu.Lock()
	if len(shotel.observations) != 2 {
		t.Errorf("Expected 2 observations, got %d", len(shotel.observations))
	}
	shotel.mu.Unlock()

	if counter1.Value() != 5.0 {
		t.Errorf("Counter1 value = %f, want 5.0", counter1.Value())
	}
	if counter2.Value() != 10.0 {
		t.Errorf("Counter2 value = %f, want 10.0", counter2.Value())
	}
}

func TestShotelKeyReuse(t *testing.T) {
	registry := metricz.New()
	observable := &mockObservable{registry: registry}

	// Use same key for counter and gauge
	const SharedKey = metricz.Key("test.shared")

	counter := registry.Counter(SharedKey)
	gauge := registry.Gauge(SharedKey)

	ctx := context.Background()
	cfg := DefaultConfig("reuse-test")
	cfg.MetricsInterval = 50 * time.Millisecond
	cfg.Endpoint = ""

	shotel, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create Shotel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = shotel.Shutdown(shutdownCtx) //nolint:errcheck // Test cleanup
	}()

	shotel.ObserveMetrics(observable, SharedKey)

	counter.Add(42)
	gauge.Set(100)

	time.Sleep(100 * time.Millisecond)

	shotel.mu.Lock()
	obs := shotel.observations[0]
	if !obs.mapping.counters[SharedKey] {
		t.Error("Shared key not mapped to counter")
	}
	if !obs.mapping.gauges[SharedKey] {
		t.Error("Shared key not mapped to gauge")
	}
	shotel.mu.Unlock()

	if counter.Value() != 42.0 {
		t.Errorf("Counter value = %f, want 42.0", counter.Value())
	}
	if gauge.Value() != 100.0 {
		t.Errorf("Gauge value = %f, want 100.0", gauge.Value())
	}
}
