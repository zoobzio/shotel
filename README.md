# aperture

[![CI Status](https://github.com/zoobzio/aperture/workflows/CI/badge.svg)](https://github.com/zoobzio/aperture/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/aperture/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/aperture)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/aperture)](https://goreportcard.com/report/github.com/zoobzio/aperture)
[![CodeQL](https://github.com/zoobzio/aperture/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/aperture/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/aperture.svg)](https://pkg.go.dev/github.com/zoobzio/aperture)
[![License](https://img.shields.io/github/license/zoobzio/aperture)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/aperture)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/aperture)](https://github.com/zoobzio/aperture/releases)

Opinionated capitan-to-OTEL bridge for automatic event observability.

Aperture observes all [capitan](https://github.com/zoobzio/capitan) events and transforms them into OpenTelemetry signals (logs, metrics, traces) based on configuration, while exposing standard OTEL Logger, Meter, and Tracer interfaces for direct use.

## Quick Start

```go
package main

import (
    "context"
    "github.com/zoobzio/capitan"
    "github.com/zoobzio/aperture"
)

func main() {
    ctx := context.Background()

    // Create OTEL providers with sensible defaults
    pvs, err := aperture.DefaultProviders(ctx, "my-service", "v1.0.0", "localhost:4318")
    if err != nil {
        panic(err)
    }
    defer pvs.Shutdown(ctx)

    // Create aperture bridge (nil config = log all events)
    sh, err := aperture.New(capitan.Default(), pvs.Log, pvs.Meter, pvs.Trace, nil)
    if err != nil {
        panic(err)
    }
    defer sh.Close()

    // Use OTEL primitives directly
    logger := sh.Logger("orders")
    meter := sh.Meter("orders")
    tracer := sh.Tracer("orders")

    // Capitan events automatically become OTEL logs
    sig := capitan.NewSignal("order.created", "Order created")
    orderID := capitan.NewStringKey("order_id")

    capitan.Default().Emit(ctx, sig, orderID.Field("ORDER-123"))
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

Aperture supports flexible configuration for transforming capitan events into OTEL signals.

### Metrics: Auto-convert Signals to OTEL Metrics

Aperture supports four metric instrument types:

#### Counter - Count Signal Occurrences

Counters increment each time a signal is emitted:

```go
orderCreated := capitan.NewSignal("order.created", "Order created")

config := &aperture.Config{
    Metrics: []aperture.MetricConfig{
        {
            Signal:      orderCreated,
            Name:        "orders_created_total",
            Type:        aperture.MetricTypeCounter, // Optional, default
            Description: "Total orders created",
        },
    },
}
```

#### Gauge - Record Instantaneous Values

Gauges record the current value from a field:

```go
cpuUsage := capitan.NewSignal("system.cpu.usage", "CPU usage measurement")
usageKey := capitan.NewFloat64Key("percent")

config := &aperture.Config{
    Metrics: []aperture.MetricConfig{
        {
            Signal:   cpuUsage,
            Name:     "cpu_usage_percent",
            Type:     aperture.MetricTypeGauge,
            ValueKey: usageKey, // Extract value from this field
        },
    },
}

// Emit gauge value
c.Emit(ctx, cpuUsage, usageKey.Field(45.2))
// ↑ Gauge set to 45.2
```

#### Histogram - Record Value Distributions

Histograms record distributions (e.g., latencies, sizes):

```go
requestCompleted := capitan.NewSignal("request.completed", "Request completed")
durationKey := capitan.NewDurationKey("duration")

config := &aperture.Config{
    Metrics: []aperture.MetricConfig{
        {
            Signal:   requestCompleted,
            Name:     "request_duration_ms",
            Type:     aperture.MetricTypeHistogram,
            ValueKey: durationKey,
        },
    },
}

// Emit duration measurements
c.Emit(ctx, requestCompleted, durationKey.Field(250*time.Millisecond))
// ↑ Histogram records 250ms
```

#### UpDownCounter - Track Increments and Decrements

UpDownCounters can increase or decrease (e.g., queue depth, active connections):

```go
queueDepth := capitan.NewSignal("queue.depth.changed", "Queue depth changed")
deltaKey := capitan.NewInt64Key("delta")

config := &aperture.Config{
    Metrics: []aperture.MetricConfig{
        {
            Signal:   queueDepth,
            Name:     "queue_depth",
            Type:     aperture.MetricTypeUpDownCounter,
            ValueKey: deltaKey,
        },
    },
}

// Emit changes
c.Emit(ctx, queueDepth, deltaKey.Field(int64(5)))   // +5
c.Emit(ctx, queueDepth, deltaKey.Field(int64(-2)))  // -2
```

**Note:** Event fields automatically become metric dimensions (attributes) for all metric types.

### Logs: Whitelist Filtering

Filter which signals get logged:

```go
orderCreated := capitan.NewSignal("order.created", "Order created")
orderFailed := capitan.NewSignal("order.failed", "Order failed")

config := &aperture.Config{
    Logs: &aperture.LogConfig{
        Whitelist: []capitan.Signal{
            orderCreated,
            orderFailed,
            // Only these signals are logged
        },
    },
}
```

If no whitelist is configured, all events are logged (default behavior).

### Traces: Correlate Signal Pairs into Spans

Create spans from start/end signal pairs:

```go
requestStarted := capitan.NewSignal("request.started", "Request started")
requestCompleted := capitan.NewSignal("request.completed", "Request completed")
requestIDKey := capitan.NewStringKey("request_id")

config := &aperture.Config{
    Traces: []aperture.TraceConfig{
        {
            Start:          requestStarted,
            End:            requestCompleted,
            CorrelationKey: &requestIDKey,
            SpanName:       "http_request",
        },
    },
}

// Emit correlated events
c.Emit(ctx, requestStarted, requestIDKey.Field("REQ-123"))
// ... work happens ...
c.Emit(ctx, requestCompleted, requestIDKey.Field("REQ-123"))
// ↑ Creates a span from start to end
```

Both start and end events must have matching correlation key values.

### Combined Configuration

Mix metrics, logs, and traces:

```go
config := &aperture.Config{
    Metrics: []aperture.MetricConfig{
        {Signal: orderCreated, Name: "orders_created_total"},
    },
    Logs: &aperture.LogConfig{
        Whitelist: []capitan.Signal{orderCreated, orderFailed},
    },
    Traces: []aperture.TraceConfig{
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

### Context Extraction: Enrich Signals with Context Values

Extract values from `context.Context` and automatically add them as attributes to logs, metrics, and traces:

```go
// Define custom context keys
type ctxKey string
const (
    userIDKey  ctxKey = "user_id"
    regionKey  ctxKey = "region"
)

config := &aperture.Config{
    ContextExtraction: &aperture.ContextExtractionConfig{
        // Extract for logs
        Logs: []aperture.ContextKey{
            {Key: userIDKey, Name: "user_id"},
            {Key: regionKey, Name: "region"},
        },

        // Extract for metrics (use low-cardinality values only!)
        Metrics: []aperture.ContextKey{
            {Key: regionKey, Name: "region"},  // Good: limited values
            // Avoid: {Key: userIDKey, ...}    // Bad: high cardinality
        },

        // Extract for traces
        Traces: []aperture.ContextKey{
            {Key: userIDKey, Name: "user_id"},
            {Key: regionKey, Name: "region"},
        },
    },
}

// Add values to context
ctx := context.Background()
ctx = context.WithValue(ctx, userIDKey, "user-123")
ctx = context.WithValue(ctx, regionKey, "us-east-1")

// Emit event - context values automatically extracted
c.Emit(ctx, orderCreated, orderIDKey.Field("ORDER-456"))
// ↑ Logs/metrics/traces will include user_id and region attributes
```

**Supported Context Value Types:**
- `string`, `int`, `int32`, `int64`, `uint`, `uint32`, `uint64`
- `float32`, `float64`, `bool`, `[]byte`

**Missing Values:** If a context key is configured but not present in the context, it is skipped (no nil/empty attributes added).

**Metrics Cardinality Warning:** Only use low-cardinality context values for metrics (e.g., region, environment, service tier). High-cardinality values like user IDs or request IDs can exponentially increase metric storage costs.

## API

### Aperture

```go
// Create aperture with pre-configured providers
func New(
    c *capitan.Capitan,
    logProvider log.LoggerProvider,
    meterProvider metric.MeterProvider,
    traceProvider trace.TracerProvider,
    config *Config, // Optional: nil means log all events
) (*Aperture, error)

// Get OTEL interfaces
func (s *Aperture) Logger(name string) log.Logger
func (s *Aperture) Meter(name string) metric.Meter
func (s *Aperture) Tracer(name string) trace.Tracer

// Stop observing (does NOT shutdown providers)
func (s *Aperture) Close()
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

Aperture transforms capitan event fields to OTEL attributes:

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

// Register transformer via Config.Transformers
config := &aperture.Config{
    Transformers: map[capitan.Variant]aperture.FieldTransformer{
        orderVariant: aperture.MakeTransformer(func(key string, order OrderInfo) []log.KeyValue {
            return []log.KeyValue{
                log.String(key+".id", order.ID),
                log.Float64(key+".total", order.Total),
                // Secret field intentionally omitted
            }
        }),
    },
}

// Now OrderInfo fields are automatically transformed
orderKey := capitan.NewKey[OrderInfo]("order", orderVariant)
c.Emit(ctx, sig, orderKey.Field(OrderInfo{...}))
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

// Pass to aperture
sh, err := aperture.New(capitan.Default(), logProvider, meterProvider, traceProvider, nil)
```

Aperture doesn't care how providers are configured - it just bridges capitan to OTEL.

## DefaultProviders() Configuration

The `DefaultProviders()` helper uses these opinionated defaults:

- **Logs**: Batch processor with OTLP HTTP exporter
- **Metrics**: Periodic reader (60s interval) with OTLP HTTP exporter
- **Traces**: Batch span processor, always-sample strategy, OTLP HTTP exporter
- **Connection**: Insecure HTTP (for local development)
- **Resource**: Service name, version, and host metadata

All exporters connect to the same `otlpEndpoint` (typically `localhost:4318` for local OTEL collectors).

## Schema-Based Configuration

For serializable configuration (YAML/JSON), use the Registry and Schema API:

### Loading Schemas

```go
// From YAML file
schema, err := aperture.LoadSchemaFromFile("observability.yaml")

// From YAML bytes
schema, err := aperture.LoadSchemaFromYAML(data)

// From JSON bytes
schema, err := aperture.LoadSchemaFromJSON(data)

// Validate before building
if err := schema.Validate(); err != nil {
    log.Fatal(err)
}
```

### Schema Format

```yaml
# observability.yaml
metrics:
  - signal: OrderPlaced
    name: orders.placed
    type: counter

  - signal: OrderCompleted
    name: orders.duration
    type: histogram
    value_key: duration

traces:
  - start: OrderPlaced
    end: OrderCompleted
    correlation_key: order_id
    span_name: order-processing
    span_timeout: 5m

logs:
  whitelist:
    - OrderPlaced
    - OrderCompleted

context:
  logs:
    - request_id
  metrics:
    - region
  traces:
    - request_id

stdout: true
```

### Registry

The registry maps string names in the schema to actual capitan types:

```go
registry := aperture.NewRegistry()

// Register signals (name extracted from signal)
registry.Register(OrderPlaced, OrderCompleted)

// Register field keys (name extracted from key)
registry.RegisterKey(OrderID, Duration, Region)

// Register context keys for extraction
registry.RegisterContextKey("request_id", requestIDKey{})
registry.RegisterContextKey("region", regionKey{})

// Register custom type transformers
registry.RegisterTransformer(orderVariant, aperture.MakeTransformer(
    func(key string, order OrderInfo) []log.KeyValue {
        return []log.KeyValue{
            log.String(key+".id", order.ID),
            log.Float64(key+".total", order.Total),
        }
    },
))

// Build runtime config from schema
config, err := registry.Build(schema)
if err != nil {
    log.Fatal(err)
}

// Create aperture with built config
s, err := aperture.New(cap, providers.Log, providers.Meter, providers.Trace, config)
```

### Introspection

Export registered components for tooling and validation:

```go
spec := registry.Spec()

// spec.Signals - registered signals with descriptions
// spec.Keys - registered keys with variants
// spec.ContextKeys - registered context key names
// spec.Transformers - registered transformer variants

// Useful for generating documentation or validating schemas
```

### Hot Reloading with Flux

Integrate with [flux](https://github.com/zoobzio/flux) for live configuration reloading:

```go
registry := aperture.NewRegistry()
registry.Register(OrderPlaced, OrderCompleted)
registry.RegisterKey(OrderID, Duration)

var currentAperture *aperture.Aperture
var mu sync.Mutex

capacitor := flux.New[aperture.Schema](
    flux.FileWatcher("observability.yaml"),
    func(schema aperture.Schema) error {
        if err := schema.Validate(); err != nil {
            return err
        }

        config, err := registry.Build(schema)
        if err != nil {
            return err
        }

        mu.Lock()
        defer mu.Unlock()

        if currentAperture != nil {
            currentAperture.Close()
        }

        currentAperture, err = aperture.New(cap, providers.Log, providers.Meter, providers.Trace, config)
        return err
    },
)

capacitor.Start(ctx)
```

### Schema Reference

#### Metrics

| Field | Required | Description |
|-------|----------|-------------|
| `signal` | Yes | Signal name to observe |
| `name` | Yes | OTEL metric name |
| `type` | No | `counter` (default), `gauge`, `histogram`, `updowncounter` |
| `value_key` | For non-counters | Field key to extract numeric value |
| `description` | No | Metric description |

#### Traces

| Field | Required | Description |
|-------|----------|-------------|
| `start` | Yes | Signal that begins the span |
| `end` | Yes | Signal that completes the span |
| `correlation_key` | Yes | Field key to match start/end events |
| `span_name` | No | Span name (defaults to start signal) |
| `span_timeout` | No | Max wait for end event (default: 5m) |

#### Logs

| Field | Description |
|-------|-------------|
| `whitelist` | Signal names to log (empty = log all) |

#### Context

| Field | Description |
|-------|-------------|
| `logs` | Context key names to extract for log attributes |
| `metrics` | Context key names to extract for metric dimensions |
| `traces` | Context key names to extract for span attributes |

## Philosophy

**Aperture is opinionated about capitan integration, agnostic about OTEL configuration.**

- How events transform to logs? **Opinionated** (type-safe, field mapping)
- Which exporter to use? **Your choice**
- How to batch? **Your choice**
- Security/TLS? **Your choice**

Aperture's value: "I observe capitan, I transform fields correctly, I give you back OTEL interfaces."

Everything else is provider configuration, which OTEL already solves.

## Installation

```bash
go get github.com/zoobzio/aperture
```

Requirements: Go 1.24+

## License

MIT
