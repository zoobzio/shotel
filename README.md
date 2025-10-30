# shotel

[![CI Status](https://github.com/zoobzio/shotel/workflows/CI/badge.svg)](https://github.com/zoobzio/shotel/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/shotel/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/shotel)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/shotel)](https://goreportcard.com/report/github.com/zoobzio/shotel)
[![CodeQL](https://github.com/zoobzio/shotel/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/shotel/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/shotel.svg)](https://pkg.go.dev/github.com/zoobzio/shotel)
[![License](https://img.shields.io/github/license/zoobzio/shotel)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/shotel)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/shotel)](https://github.com/zoobzio/shotel/releases)

Opinionated capitan-to-OTEL bridge for automatic event observability.

Shotel observes all [capitan](https://github.com/zoobzio/capitan) events and transforms them into OpenTelemetry signals (logs, metrics, traces) based on configuration, while exposing standard OTEL Logger, Meter, and Tracer interfaces for direct use.

## Quick Start

```go
package main

import (
    "context"
    "github.com/zoobzio/capitan"
    "github.com/zoobzio/shotel"
)

func main() {
    ctx := context.Background()

    // Create OTEL providers with sensible defaults
    pvs, err := shotel.DefaultProviders(ctx, "my-service", "v1.0.0", "localhost:4318")
    if err != nil {
        panic(err)
    }
    defer pvs.Shutdown(ctx)

    // Create shotel bridge (nil config = log all events)
    sh, err := shotel.New(capitan.Default(), pvs.Log, pvs.Meter, pvs.Trace, nil)
    if err != nil {
        panic(err)
    }
    defer sh.Close()

    // Use OTEL primitives directly
    logger := sh.Logger("orders")
    meter := sh.Meter("orders")
    tracer := sh.Tracer("orders")

    // Capitan events automatically become OTEL logs
    sig := capitan.Signal("order.created")
    orderID := capitan.NewStringKey("order_id")

    capitan.Emit(ctx, sig, orderID.Field("ORDER-123"))
    // ↑ Automatically logged to OTEL
}
```

## Architecture

### Separation of Concerns

**Capitan integration:** Opinionated observability
- Observes all capitan events
- Transforms events to OTEL signals (logs, metrics, traces) based on config
- Transforms fields to OTEL attributes
- Exposes OTEL interfaces

**Provider configuration:** Flexible setup
- `DefaultProviders()` for quick setup with sensible defaults
- Or construct your own OTEL providers for full control
- Handles exporters, processors, samplers
- Manages lifecycle

This separation allows full control over OTEL configuration while keeping capitan integration opinionated and consistent.

## Configuration

Shotel supports flexible configuration for transforming capitan events into OTEL signals.

### Metrics: Auto-convert Signals to OTEL Metrics

Shotel supports four metric instrument types:

#### Counter - Count Signal Occurrences

Counters increment each time a signal is emitted:

```go
orderCreated := capitan.Signal("order.created")

config := &shotel.Config{
    Metrics: []shotel.MetricConfig{
        {
            Signal:      orderCreated,
            Name:        "orders_created_total",
            Type:        shotel.MetricTypeCounter, // Optional, default
            Description: "Total orders created",
        },
    },
}
```

#### Gauge - Record Instantaneous Values

Gauges record the current value from a field:

```go
cpuUsage := capitan.Signal("system.cpu.usage")
usageKey := capitan.NewFloat64Key("percent")

config := &shotel.Config{
    Metrics: []shotel.MetricConfig{
        {
            Signal:   cpuUsage,
            Name:     "cpu_usage_percent",
            Type:     shotel.MetricTypeGauge,
            ValueKey: usageKey, // Extract value from this field
        },
    },
}

// Emit gauge value
cap.Emit(ctx, cpuUsage, usageKey.Field(45.2))
// ↑ Gauge set to 45.2
```

#### Histogram - Record Value Distributions

Histograms record distributions (e.g., latencies, sizes):

```go
requestCompleted := capitan.Signal("request.completed")
durationKey := capitan.NewDurationKey("duration")

config := &shotel.Config{
    Metrics: []shotel.MetricConfig{
        {
            Signal:   requestCompleted,
            Name:     "request_duration_ms",
            Type:     shotel.MetricTypeHistogram,
            ValueKey: durationKey,
        },
    },
}

// Emit duration measurements
cap.Emit(ctx, requestCompleted, durationKey.Field(250*time.Millisecond))
// ↑ Histogram records 250ms
```

#### UpDownCounter - Track Increments and Decrements

UpDownCounters can increase or decrease (e.g., queue depth, active connections):

```go
queueDepth := capitan.Signal("queue.depth.changed")
deltaKey := capitan.NewInt64Key("delta")

config := &shotel.Config{
    Metrics: []shotel.MetricConfig{
        {
            Signal:   queueDepth,
            Name:     "queue_depth",
            Type:     shotel.MetricTypeUpDownCounter,
            ValueKey: deltaKey,
        },
    },
}

// Emit changes
cap.Emit(ctx, queueDepth, deltaKey.Field(5))   // +5
cap.Emit(ctx, queueDepth, deltaKey.Field(-2))  // -2
```

**Note:** Event fields automatically become metric dimensions (attributes) for all metric types.

### Logs: Whitelist Filtering

Filter which signals get logged:

```go
config := &shotel.Config{
    Logs: &shotel.LogConfig{
        Whitelist: []capitan.Signal{
            capitan.Signal("order.created"),
            capitan.Signal("order.failed"),
            // Only these signals are logged
        },
    },
}
```

If no whitelist is configured, all events are logged (default behavior).

### Traces: Correlate Signal Pairs into Spans

Create spans from start/end signal pairs:

```go
requestStarted := capitan.Signal("request.started")
requestCompleted := capitan.Signal("request.completed")
requestIDKey := capitan.NewStringKey("request_id")

config := &shotel.Config{
    Traces: []shotel.TraceConfig{
        {
            Start:          requestStarted,
            End:            requestCompleted,
            CorrelationKey: &requestIDKey,
            SpanName:       "http_request",
        },
    },
}

// Emit correlated events
cap.Emit(ctx, requestStarted, requestIDKey.Field("REQ-123"))
// ... work happens ...
cap.Emit(ctx, requestCompleted, requestIDKey.Field("REQ-123"))
// ↑ Creates a span from start to end
```

Both start and end events must have matching correlation key values.

### Combined Configuration

Mix metrics, logs, and traces:

```go
config := &shotel.Config{
    Metrics: []shotel.MetricConfig{
        {Signal: orderCreated, Name: "orders_created_total"},
    },
    Logs: &shotel.LogConfig{
        Whitelist: []capitan.Signal{orderCreated, orderFailed},
    },
    Traces: []shotel.TraceConfig{
        {
            Start:          orderCreated,
            End:            orderCompleted,
            CorrelationKey: &orderIDKey,
            SpanName:       "order_processing",
        },
    },
}
```

A single event can trigger multiple signal types (counted as metric + logged + start/end span).

## API

### Shotel

```go
// Create shotel with pre-configured providers
func New(
    c *capitan.Capitan,
    logProvider log.LoggerProvider,
    meterProvider metric.MeterProvider,
    traceProvider trace.TracerProvider,
    config *Config, // Optional: nil means log all events
) (*Shotel, error)

// Get OTEL interfaces
func (s *Shotel) Logger(name string) log.Logger
func (s *Shotel) Meter(name string) metric.Meter
func (s *Shotel) Tracer(name string) trace.Tracer

// Stop observing (does NOT shutdown providers)
func (s *Shotel) Close()
```

### Providers

```go
// DefaultProviders creates providers with opinionated configuration
func DefaultProviders(
    ctx context.Context,
    serviceName string,
    serviceVersion string,
    otlpEndpoint string,
) (*Providers, error)

// Providers holds all three OTEL providers
type Providers struct {
    Log   *log.LoggerProvider
    Meter *metric.MeterProvider
    Trace *trace.TracerProvider
}

// Shutdown all providers
func (p *Providers) Shutdown(ctx context.Context) error
```

## Capitan Field Transformation

Shotel transforms capitan event fields to OTEL attributes:

- `Event.Signal()` → `log.String("capitan.signal", ...)`
- `Event.Timestamp()` → `LogRecord.SetTimestamp(...)`
- `Event.Context()` → Trace context propagation
- `Event.Fields()` → Log attributes via type-safe transformation

### Supported Field Types

All built-in capitan field types are supported:

- `string`, `int`, `int32`, `int64`, `uint`, `uint32`, `uint64`
- `float32`, `float64`, `bool`
- `time.Time` (as Unix timestamp), `time.Duration` (as nanoseconds)
- `[]byte`, `error` (as string)

### Custom Field Types

Register transformers for custom types:

```go
type OrderInfo struct {
    ID     string
    Total  float64
    Secret string // Not logged
}

orderVariant := capitan.Variant("myapp.OrderInfo")

// Register transformer - receives typed value directly
shotel.RegisterTransformer(orderVariant, func(key string, order OrderInfo) []log.KeyValue {
    return []log.KeyValue{
        log.String(key+".id", order.ID),
        log.Float64(key+".total", order.Total),
        // Secret field intentionally omitted
    }
})

// Now OrderInfo fields are automatically transformed
orderKey := capitan.NewKey[OrderInfo]("order", orderVariant)
capitan.Emit(ctx, sig, orderKey.Field(OrderInfo{...}))
```

No manual type assertions needed - you receive the typed value directly.

## Custom Provider Configuration

For full control over OTEL configuration, construct providers yourself:

```go
// Build your own providers with custom configuration
logProvider := log.NewLoggerProvider(
    log.WithResource(myResource),
    log.WithProcessor(myProcessor),
    // Your custom configuration
)

meterProvider := metric.NewMeterProvider(
    metric.WithResource(myResource),
    metric.WithReader(myReader),
    // Your custom configuration
)

traceProvider := trace.NewTracerProvider(
    trace.WithResource(myResource),
    trace.WithSpanProcessor(myProcessor),
    trace.WithSampler(mySampler),
    // Your custom configuration
)

// Pass to shotel
sh, err := shotel.New(capitan.Default(), logProvider, meterProvider, traceProvider)
```

Shotel doesn't care how providers are configured - it just bridges capitan to OTEL.

## DefaultProviders() Configuration

The `DefaultProviders()` helper uses these opinionated defaults:

- **Logs**: Batch processor with OTLP HTTP exporter
- **Metrics**: Periodic reader (60s interval) with OTLP HTTP exporter
- **Traces**: Batch span processor, always-sample strategy, OTLP HTTP exporter
- **Connection**: Insecure HTTP (for local development)
- **Resource**: Service name, version, and host metadata

All exporters connect to the same `otlpEndpoint` (typically `localhost:4318` for local OTEL collectors).

## Philosophy

**Shotel is opinionated about capitan integration, agnostic about OTEL configuration.**

- How events transform to logs? **Opinionated** (type-safe, field mapping)
- Which exporter to use? **Your choice**
- How to batch? **Your choice**
- Security/TLS? **Your choice**

Shotel's value: "I observe capitan, I transform fields correctly, I give you back OTEL interfaces."

Everything else is provider configuration, which OTEL already solves.

## Installation

```bash
go get github.com/zoobzio/shotel
```

Requirements: Go 1.24+

## License

MIT
