package shotel

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Observable defines components that expose metrics.
type Observable interface {
	Metrics() *metricz.Registry
}

// Traceable defines components that expose a tracer.
type Traceable interface {
	Tracer() *tracez.Tracer
}

// Shotel manages the bridge between application observables and OTLP.
type Shotel struct {
	config        *Config
	exporter      *Exporter
	provider      *sdkmetric.MeterProvider
	meter         metric.Meter
	traceProvider *sdktrace.TracerProvider
	tracer        trace.Tracer
	logProvider   *sdklog.LoggerProvider
	logger        otellog.Logger
	slogger       *slog.Logger

	// Metric observation tracking
	observations []*metricObservation
	// Trace observation tracking
	traceObservations []*traceObservation
	mu                sync.Mutex
}

// traceObservation tracks a single traceable's span handler.
type traceObservation struct {
	traceable Traceable
	handlerID uint64
}

// metricObservation tracks a single observable's metric keys.
type metricObservation struct {
	observable Observable
	mapping    *metricMapping
	cancel     context.CancelFunc
}

// metricMapping tracks which metric types to observe for given keys.
type metricMapping struct {
	counters   map[metricz.Key]bool
	gauges     map[metricz.Key]bool
	histograms map[metricz.Key]bool
	timers     map[metricz.Key]bool
}

// New creates a new Shotel instance.
func New(ctx context.Context, cfg *Config) (*Shotel, error) {
	if cfg == nil {
		cfg = DefaultConfig("shotel")
	}

	cfg.Logger.Info("creating shotel instance",
		slog.String("service", cfg.ServiceName),
		slog.String("endpoint", cfg.Endpoint),
		slog.Duration("metrics_interval", cfg.MetricsInterval))

	// Create OTLP exporter
	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		cfg.Logger.Error("failed to create OTLP exporter", slog.Any("error", err))
		return nil, err
	}
	cfg.Logger.Debug("created OTLP exporter")

	// Create meter provider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(exporter.resource),
		sdkmetric.WithReader(exporter.MetricReader()),
	)

	// Create meter for this shotel instance
	meter := provider.Meter("github.com/zoobzio/shotel")

	// Create tracer for this shotel instance
	tracer := exporter.TraceProvider().Tracer("github.com/zoobzio/shotel")

	// Create logger for this shotel instance
	logger := exporter.LogProvider().Logger("github.com/zoobzio/shotel")

	return &Shotel{
		config:            cfg,
		exporter:          exporter,
		provider:          provider,
		meter:             meter,
		traceProvider:     exporter.TraceProvider(),
		tracer:            tracer,
		logProvider:       exporter.LogProvider(),
		logger:            logger,
		slogger:           cfg.Logger,
		observations:      make([]*metricObservation, 0),
		traceObservations: make([]*traceObservation, 0),
	}, nil
}

// ObserveMetrics registers an observable and starts polling specified metric keys.
func (s *Shotel) ObserveMetrics(observable Observable, keys ...metricz.Key) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.slogger.Info("observing metrics", slog.Int("key_count", len(keys)))

	// Build metric mapping by checking which types contain each key
	mapping := &metricMapping{
		counters:   make(map[metricz.Key]bool),
		gauges:     make(map[metricz.Key]bool),
		histograms: make(map[metricz.Key]bool),
		timers:     make(map[metricz.Key]bool),
	}

	registry := observable.Metrics()

	// Discover which metric types each key belongs to
	for _, key := range keys {
		if _, exists := registry.GetCounters()[key]; exists {
			mapping.counters[key] = true
		}
		if _, exists := registry.GetGauges()[key]; exists {
			mapping.gauges[key] = true
		}
		if _, exists := registry.GetHistograms()[key]; exists {
			mapping.histograms[key] = true
		}
		if _, exists := registry.GetTimers()[key]; exists {
			mapping.timers[key] = true
		}
	}

	s.slogger.Debug("discovered metric types",
		slog.Int("counters", len(mapping.counters)),
		slog.Int("gauges", len(mapping.gauges)),
		slog.Int("histograms", len(mapping.histograms)),
		slog.Int("timers", len(mapping.timers)))

	// Create cancellable context for this observation
	ctx, cancel := context.WithCancel(context.Background())

	obs := &metricObservation{
		observable: observable,
		mapping:    mapping,
		cancel:     cancel,
	}

	s.observations = append(s.observations, obs)

	// Start polling goroutine
	go s.pollMetrics(ctx, obs)
}

// pollMetrics periodically reads metrics from the observable and updates OTLP instruments.
func (s *Shotel) pollMetrics(ctx context.Context, obs *metricObservation) {
	ticker := time.NewTicker(s.config.MetricsInterval)
	defer ticker.Stop()

	registry := obs.observable.Metrics()

	// Create OTLP instruments for each metric key
	otelCounters := make(map[metricz.Key]metric.Int64Counter)
	otelGauges := make(map[metricz.Key]metric.Float64ObservableGauge)
	otelHistograms := make(map[metricz.Key]metric.Float64Histogram)

	// Register counters
	for key := range obs.mapping.counters {
		counter, err := s.meter.Int64Counter(string(key))
		if err == nil {
			otelCounters[key] = counter
			s.slogger.Debug("registered counter", slog.String("key", string(key)))
		} else {
			s.slogger.Error("failed to register counter", slog.String("key", string(key)), slog.Any("error", err))
		}
	}

	// Register gauges (using observable gauges for pull-based metrics)
	for key := range obs.mapping.gauges {
		gauge, err := s.meter.Float64ObservableGauge(
			string(key),
			metric.WithFloat64Callback(func(_ context.Context, observer metric.Float64Observer) error {
				if g, exists := registry.GetGauges()[key]; exists {
					observer.Observe(g.Value(), metric.WithAttributes(attribute.String("source", "metricz")))
				}
				return nil
			}),
		)
		if err == nil {
			otelGauges[key] = gauge
			s.slogger.Debug("registered gauge", slog.String("key", string(key)))
		} else {
			s.slogger.Error("failed to register gauge", slog.String("key", string(key)), slog.Any("error", err))
		}
	}

	// Register histograms
	for key := range obs.mapping.histograms {
		histogram, err := s.meter.Float64Histogram(string(key))
		if err == nil {
			otelHistograms[key] = histogram
			s.slogger.Debug("registered histogram", slog.String("key", string(key)))
		} else {
			s.slogger.Error("failed to register histogram", slog.String("key", string(key)), slog.Any("error", err))
		}
	}

	// Register timers as histograms
	for key := range obs.mapping.timers {
		histogram, err := s.meter.Float64Histogram(string(key))
		if err == nil {
			otelHistograms[key] = histogram
			s.slogger.Debug("registered timer", slog.String("key", string(key)))
		} else {
			s.slogger.Error("failed to register timer", slog.String("key", string(key)), slog.Any("error", err))
		}
	}

	// Track previous counter values for delta calculation
	previousCounters := make(map[metricz.Key]float64)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Poll counters (calculate deltas since OTLP counters are cumulative)
			for key, otelCounter := range otelCounters {
				counter, exists := registry.GetCounters()[key]
				if !exists {
					continue
				}
				current := counter.Value()
				previous := previousCounters[key]
				delta := current - previous
				if delta > 0 {
					otelCounter.Add(ctx, int64(delta), metric.WithAttributes(attribute.String("source", "metricz")))
				}
				previousCounters[key] = current
			}

			// Gauges are handled by callbacks (pull-based)

			// Poll histograms
			for key, otelHistogram := range otelHistograms {
				if hist, exists := registry.GetHistograms()[key]; exists {
					buckets, counts := hist.Buckets()
					// Record all observations from the histogram
					for i, bucket := range buckets {
						count := counts[i]
						for j := uint64(0); j < count; j++ {
							otelHistogram.Record(ctx, bucket, metric.WithAttributes(attribute.String("source", "metricz")))
						}
					}
				}
			}

			// Poll timers (same as histograms)
			for key, otelHistogram := range otelHistograms {
				if timer, exists := registry.GetTimers()[key]; exists {
					buckets, counts := timer.Buckets()
					// Record all observations from the timer
					for i, bucket := range buckets {
						count := counts[i]
						for j := uint64(0); j < count; j++ {
							otelHistogram.Record(ctx, bucket, metric.WithAttributes(attribute.String("source", "metricz")))
						}
					}
				}
			}
		}
	}
}

// ObserveTraces registers a traceable and bridges its spans to OTLP.
func (s *Shotel) ObserveTraces(traceable Traceable) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.slogger.Info("observing traces")

	tracer := traceable.Tracer()

	// Register handler to convert tracez spans to OTLP spans
	handlerID := tracer.OnSpanComplete(func(span tracez.Span) {
		s.slogger.Debug("bridging span to OTLP",
			slog.String("span_name", span.Name),
			slog.String("trace_id", span.TraceID))
		// Create OTLP span
		ctx := context.Background()
		_, otelSpan := s.tracer.Start(ctx, span.Name,
			trace.WithTimestamp(span.StartTime),
			trace.WithSpanKind(trace.SpanKindInternal),
		)

		// Copy tags as attributes
		if span.Tags != nil {
			attrs := make([]attribute.KeyValue, 0, len(span.Tags))
			for key, value := range span.Tags {
				attrs = append(attrs, attribute.String(key, value))
			}
			otelSpan.SetAttributes(attrs...)
		}

		// Set span IDs if available
		if span.TraceID != "" {
			otelSpan.SetAttributes(attribute.String("tracez.trace_id", span.TraceID))
		}
		if span.SpanID != "" {
			otelSpan.SetAttributes(attribute.String("tracez.span_id", span.SpanID))
		}
		if span.ParentID != "" {
			otelSpan.SetAttributes(attribute.String("tracez.parent_id", span.ParentID))
		}

		// Set status as OK (tracez doesn't track errors separately)
		otelSpan.SetStatus(codes.Ok, "")

		// End span with original end time
		otelSpan.End(trace.WithTimestamp(span.EndTime))
	})

	obs := &traceObservation{
		traceable: traceable,
		handlerID: handlerID,
	}

	s.traceObservations = append(s.traceObservations, obs)
}

// Shutdown gracefully stops all observations and shuts down the exporter.
func (s *Shotel) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.slogger.Info("shutting down shotel",
		slog.Int("metric_observations", len(s.observations)),
		slog.Int("trace_observations", len(s.traceObservations)))

	// Cancel all metric observations
	for _, obs := range s.observations {
		obs.cancel()
	}
	s.slogger.Debug("canceled all metric observations")

	// Remove all trace handlers
	for _, obs := range s.traceObservations {
		obs.traceable.Tracer().RemoveHandler(obs.handlerID)
	}
	s.slogger.Debug("removed all trace handlers")

	// Shutdown provider
	if err := s.provider.Shutdown(ctx); err != nil {
		s.slogger.Error("failed to shutdown meter provider", slog.Any("error", err))
		return err
	}
	s.slogger.Debug("shutdown meter provider")

	// Shutdown exporter
	if err := s.exporter.Shutdown(ctx); err != nil {
		s.slogger.Error("failed to shutdown exporter", slog.Any("error", err))
		return err
	}
	s.slogger.Info("shotel shutdown complete")
	return nil
}
