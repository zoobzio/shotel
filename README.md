# aperture

[![CI Status](https://github.com/zoobzio/aperture/workflows/CI/badge.svg)](https://github.com/zoobzio/aperture/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/aperture/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/aperture)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/aperture)](https://goreportcard.com/report/github.com/zoobzio/aperture)
[![CodeQL](https://github.com/zoobzio/aperture/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/aperture/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/aperture.svg)](https://pkg.go.dev/github.com/zoobzio/aperture)
[![License](https://img.shields.io/github/license/zoobzio/aperture)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/aperture)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/aperture)](https://github.com/zoobzio/aperture/releases)

Config-driven bridge from [capitan](https://github.com/zoobzio/capitan) events to OpenTelemetry signals.

Emit a capitan event, aperture transforms it into OTEL logs, metrics, and traces. Define your signals in code, configure observability in YAML, change what's observed at runtime without recompiling.

## Two Layers

```go
// 1. Register your domain signals (compile-time, type-safe)
registry := aperture.NewRegistry()
registry.Register(orderCreated, orderCompleted)
registry.RegisterKey(orderID, duration)
```

```yaml
# 2. Configure what becomes observable (runtime, hot-reloadable)
# observability.yaml
metrics:
  - signal: order.created
    name: orders_total
    type: counter

traces:
  - start: order.created
    end: order.completed
    correlation_key: order_id
    span_name: order_processing
```

```go
// 3. Build and bridge
schema, _ := aperture.LoadSchemaFromFile("observability.yaml")
config, _ := registry.Build(schema)  // Validates references exist
ap, _ := aperture.New(capitan.Default(), logProvider, meterProvider, traceProvider, config)
```

Signals are type-safe. Configuration is data. Change what's observed without touching code.

## Installation

```bash
go get github.com/zoobzio/aperture
```

Requires Go 1.24+.

## Quick Start

```go
package main

import (
    "context"
    "log"

    "time"

    "github.com/zoobzio/aperture"
    "github.com/zoobzio/capitan"
    // OTEL imports omitted for brevity
)

// Domain signals - defined once, used everywhere
var (
    orderCreated   = capitan.NewSignal("order.created", "Order placed")
    orderCompleted = capitan.NewSignal("order.completed", "Order fulfilled")
    orderID        = capitan.NewStringKey("order_id")
    duration       = capitan.NewDurationKey("duration")
)

func main() {
    ctx := context.Background()

    // Register what exists in the system
    registry := aperture.NewRegistry()
    registry.Register(orderCreated, orderCompleted)
    registry.RegisterKey(orderID, duration)

    // Load configuration (could be from file, env, remote config...)
    schema, err := aperture.LoadSchemaFromFile("observability.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Validate and build - fails if config references unknown signals
    config, err := registry.Build(schema)
    if err != nil {
        log.Fatal(err)
    }

    // Create OTEL providers (your setup, your exporters)
    logProvider, meterProvider, traceProvider := setupOTEL()

    // Bridge capitan to OTEL
    ap, _ := aperture.New(capitan.Default(), logProvider, meterProvider, traceProvider, config)
    defer ap.Close()

    // Emit domain events - aperture handles observability
    capitan.Emit(ctx, orderCreated, orderID.Field("ORD-123"))
    capitan.Emit(ctx, orderCompleted, orderID.Field("ORD-123"), duration.Field(250*time.Millisecond))
    // → Logged, counted, traced - based on config

    capitan.Shutdown()
}
```

**observability.yaml:**
```yaml
metrics:
  - signal: order.created
    name: orders_total
    type: counter
  - signal: order.completed
    name: order_duration_ms
    type: histogram
    value_key: duration

traces:
  - start: order.created
    end: order.completed
    correlation_key: order_id
    span_name: order_processing

logs:
  whitelist:
    - order.created
    - order.completed
```

## Why aperture?

- **Config-driven** — Change what's observed without recompiling
- **Type-safe registration** — Unknown signals in config fail at startup, not runtime
- **All three signals** — Logs, metrics, and traces from a single event stream
- **Hot-reloadable** — Pair with [flux](https://github.com/zoobzio/flux) for live config updates
- **Zero instrumentation** — Domain events become telemetry automatically
- **Trace correlation** — Pair start/end events into spans automatically

## Signal Transformations

| Config | OTEL Signal | Use Case |
|--------|-------------|----------|
| (default) | Log record | Audit trail, debugging |
| `type: counter` | Counter | Count occurrences |
| `type: gauge` + `value_key` | Gauge | Current measurements |
| `type: histogram` + `value_key` | Histogram | Latency distributions |
| `start`/`end` pair | Trace span | Request flows |

## Documentation

Full documentation is available in the [docs/](docs/) directory:

### Learn
- [Overview](docs/1.overview.md) — Architecture and philosophy
- [Quickstart](docs/2.learn/1.quickstart.md) — Get running in five minutes
- [Concepts](docs/2.learn/2.concepts.md) — Core mental model

### Guides
- [Metrics](docs/3.guides/1.metrics.md) — Counters, gauges, histograms, up/down counters
- [Traces](docs/3.guides/2.traces.md) — Span correlation from event pairs
- [Logs](docs/3.guides/3.logs.md) — Whitelist filtering
- [Context](docs/3.guides/4.context.md) — Extract context values as attributes
- [Schema](docs/3.guides/5.schema.md) — File-based configuration
- [Testing](docs/3.guides/6.testing.md) — Test utilities

### Cookbook
- [HTTP Server](docs/4.cookbook/1.http-server.md) — Complete HTTP server example
- [Background Workers](docs/4.cookbook/2.background-workers.md) — Job queue observability

### Reference
- [API Reference](docs/5.reference/1.api.md) — Complete API documentation

## Contributing

Contributions welcome! Please ensure:
- Tests pass: `go test ./...`
- Code is formatted: `go fmt ./...`
- No lint errors: `golangci-lint run`

## License

MIT License — see [LICENSE](LICENSE) for details.
