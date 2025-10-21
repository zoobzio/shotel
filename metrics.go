package shotel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/metricz"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// MetricsBridge polls a metricz.Registry and exports metrics to OTLP.
type MetricsBridge struct {
	registry *metricz.Registry
	exporter *metric.PeriodicReader
	provider *metric.MeterProvider
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// BridgeMetrics creates a bridge that periodically polls metricz.Registry
// and exports metrics to OTLP.
func BridgeMetrics(ctx context.Context, registry *metricz.Registry, cfg *Config) (*MetricsBridge, error) {
	// Create OTLP exporter
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
	}

	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create periodic reader that exports at configured interval
	reader := metric.NewPeriodicReader(exporter,
		metric.WithInterval(cfg.MetricsInterval),
	)

	// Create meter provider
	provider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)

	bridgeCtx, cancel := context.WithCancel(ctx)

	bridge := &MetricsBridge{
		registry: registry,
		exporter: reader,
		provider: provider,
		interval: cfg.MetricsInterval,
		cancel:   cancel,
	}

	// Start polling goroutine
	bridge.wg.Add(1)
	go bridge.poll(bridgeCtx)

	return bridge, nil
}

// poll periodically reads metrics from metricz.Registry and updates OTLP metrics.
func (b *MetricsBridge) poll(ctx context.Context) {
	defer b.wg.Done()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	meter := b.provider.Meter("github.com/zoobzio/shotel")

	// Create OTLP metric instruments
	// We'll create them lazily as we discover metrics in the registry
	// This is a simplified implementation - in production you'd cache instruments

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Read metrics from registry
			// Note: metricz doesn't expose a way to iterate all metrics
			// This is a limitation we'd need to address by adding an iteration API
			// to metricz, or by tracking metric keys separately

			// For now, this is a placeholder that demonstrates the pattern
			// Real implementation would require metricz to expose registered metrics
			_ = meter
		}
	}
}

// Close stops the metrics bridge and flushes remaining metrics.
func (b *MetricsBridge) Close(ctx context.Context) error {
	b.cancel()
	b.wg.Wait()

	if err := b.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown meter provider: %w", err)
	}

	return nil
}
