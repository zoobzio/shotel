# shotel

[![CI Status](https://github.com/zoobzio/shotel/workflows/CI/badge.svg)](https://github.com/zoobzio/shotel/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/shotel/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/shotel)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/shotel)](https://goreportcard.com/report/github.com/zoobzio/shotel)
[![CodeQL](https://github.com/zoobzio/shotel/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/shotel/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/shotel.svg)](https://pkg.go.dev/github.com/zoobzio/shotel)
[![License](https://img.shields.io/github/license/zoobzio/shotel)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/shotel)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/shotel)](https://github.com/zoobzio/shotel/releases)

OpenTelemetry bridge for Go observability libraries - connects metricz, tracez, hookz, and slog to OTLP collectors.

## Why shotel?

Your application is instrumented with lightweight observability libraries (metricz, tracez, hookz), but you need to send that data to an OTLP collector (Jaeger, Prometheus, etc.). Shotel bridges the gap with zero reflection and minimal overhead.

**What it does:**
- Translates metricz metrics → OTLP metrics
- Converts tracez spans → OTLP traces
- Bridges hookz events → OTLP logs
- Integrates slog → OTLP logs

**What it doesn't do:**
- Replace your instrumentation libraries
- Force you to use OpenTelemetry APIs everywhere
- Add complexity to your application code

## Installation

```bash
go get github.com/zoobzio/shotel
```

Requirements: Go 1.24+

## Quick Start

```go
package main

import (
    "context"
    "log/slog"

    "github.com/zoobzio/rocco"
    "github.com/zoobzio/shotel"
)

func main() {
    ctx := context.Background()

    // Create shotel instance
    sh, err := shotel.New(ctx, &shotel.Config{
        ServiceName: "my-service",
        Endpoint:    "localhost:4317",
        Insecure:    true,
    })
    if err != nil {
        panic(err)
    }
    defer sh.Shutdown(ctx)

    // Create your instrumented service (rocco example)
    engine := rocco.NewEngine(rocco.DefaultConfig())

    // Wire up observability
    sh.ObserveMetrics(engine,
        rocco.MetricRequestsReceived,
        rocco.MetricRequestDuration,
    )
    sh.ObserveTraces(engine)

    // Hook up logs
    logHandler := sh.CreateLogHandler()
    engine.OnRequestReceived(logHandler)
    engine.OnRequestCompleted(logHandler)

    // Or use slog globally
    sh.SetGlobalSlogHandler()
    slog.Info("service started", "port", 8080)

    engine.Start()
}
```

## Configuration

```go
cfg := &shotel.Config{
    ServiceName:     "my-service",      // Service identifier in telemetry
    Endpoint:        "localhost:4317",  // OTLP collector endpoint
    MetricsInterval: 10 * time.Second,  // How often to poll metrics
    Insecure:        true,              // Disable TLS (dev only)
}
```

Default configuration:
```go
cfg := shotel.DefaultConfig("my-service")
```

## Metrics Bridge

Shotel polls metrics from any component exposing a `Metrics() *metricz.Registry` method.

```go
// Observable interface
type Observable interface {
    Metrics() *metricz.Registry
}

// Example with pipz
pipeline := pipz.NewSequence("order-processing", ...)
sh.ObserveMetrics(pipeline,
    pipz.ProcessorCallsTotal,
    pipz.ProcessorErrorsTotal,
)

// Example with rocco
engine := rocco.NewEngine(...)
sh.ObserveMetrics(engine,
    rocco.MetricRequestsReceived,
    rocco.MetricRequestDuration,
)
```

**Supported metric types:**
- Counters → OTLP Int64Counter (delta calculation)
- Gauges → OTLP Float64ObservableGauge (pull-based)
- Histograms → OTLP Float64Histogram
- Timers → OTLP Float64Histogram (milliseconds)

**Key reuse:** The same key can be used for multiple metric types (counter + gauge) without conflict.

## Traces Bridge

Shotel registers handlers on any component exposing a `Tracer() *tracez.Tracer` method.

```go
// Traceable interface
type Traceable interface {
    Tracer() *tracez.Tracer
}

// Example with rocco
engine := rocco.NewEngine(...)
sh.ObserveTraces(engine)

// All spans automatically flow to OTLP
// - Span names, timestamps, tags preserved
// - Parent-child relationships maintained
// - TraceIDs and SpanIDs tracked
```

## Logs Bridge

Shotel provides factory methods for log integration - you control the registration.

### hookz Integration

```go
// Create handler
logHandler := sh.CreateLogHandler()

// Register with your hookz events
engine.OnRequestReceived(logHandler)
engine.OnRequestCompleted(logHandler)
engine.OnRequestRejected(logHandler)

// Events flow: hookz → shotel → OTLP logs
```

### slog Integration

```go
// Option 1: Explicit logger
logger := slog.New(sh.CreateSlogHandler())
logger.Info("message", "key", "value")

// Option 2: Global default
sh.SetGlobalSlogHandler()
slog.Info("message", "key", "value")  // All slog calls → OTLP

// Option 3: With attributes
handler := sh.CreateSlogHandler()
logger := slog.New(handler.WithAttrs([]slog.Attr{
    slog.String("service", "api"),
    slog.Int("version", 1),
}))

// Option 4: With groups
handler := sh.CreateSlogHandler()
logger := slog.New(handler.WithGroup("request"))
logger.Info("received", "method", "GET", "path", "/api/users")
// Produces: request.method=GET, request.path=/api/users
```

**Supported slog types:**
- String, Int64, Uint64, Float64, Bool
- Duration (as nanoseconds)
- Time (RFC3339Nano format)

**Log levels:** Debug, Info, Warn, Error → OTLP severity mapping

## Real-World Example

Complete integration with rocco web framework:

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "github.com/zoobzio/rocco"
    "github.com/zoobzio/shotel"
)

func main() {
    ctx := context.Background()

    // Configure shotel
    sh, err := shotel.New(ctx, &shotel.Config{
        ServiceName:     "rosetta-api",
        Endpoint:        "localhost:4317",
        MetricsInterval: 10 * time.Second,
        Insecure:        true,
    })
    if err != nil {
        panic(err)
    }
    defer sh.Shutdown(context.Background())

    // Create rocco engine (has Metrics(), Tracer(), Hooks())
    engine := rocco.NewEngine(rocco.DefaultConfig())

    // Bridge metrics
    sh.ObserveMetrics(engine,
        rocco.MetricRequestsReceived,
        rocco.MetricRequestsCompleted,
        rocco.MetricRequestsRejected,
        rocco.MetricRequestDuration,
    )

    // Bridge traces
    sh.ObserveTraces(engine)

    // Bridge hookz events to logs
    logHandler := sh.CreateLogHandler()
    engine.OnRequestReceived(logHandler)
    engine.OnRequestCompleted(logHandler)
    engine.OnRequestRejected(logHandler)

    // Use slog for structured logging → OTLP
    sh.SetGlobalSlogHandler()

    // Register handlers
    engine.Register(handlers.NewUsersHandler())

    // Start server
    slog.Info("server starting", "port", 8080)
    engine.Start()
}
```

Result: All metrics, traces, and logs flow to your OTLP collector (Jaeger, Prometheus, etc.)

## Architecture

```
Application
    ├─ metricz.Registry ──┐
    ├─ tracez.Tracer ─────┤
    ├─ hookz.Hooks ───────┼──> Shotel ──> OTLP Collector ──> Jaeger/Prometheus
    └─ slog.Logger ───────┘                                   Loki/etc.
```

**No reflection, no type assertions** - everything is compile-time safe.

## Interfaces

Shotel works with any component implementing these interfaces:

```go
// For metrics observation
type Observable interface {
    Metrics() *metricz.Registry
}

// For trace observation
type Traceable interface {
    Tracer() *tracez.Tracer
}

// For logs - user-controlled registration
logHandler := shotel.CreateLogHandler()
slogHandler := shotel.CreateSlogHandler()
```

## Performance

- **Metrics**: Polled at configured interval (default 10s)
- **Traces**: Zero overhead when no handlers registered
- **Logs**: Direct translation, no buffering
- **Zero reflection**: All operations are type-safe
- **Minimal allocations**: Optimized for production use

## Shutdown

Always shutdown gracefully to flush pending data:

```go
defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    sh.Shutdown(shutdownCtx)
}()
```

## Dependencies

- `github.com/zoobzio/metricz` - Metrics primitives
- `github.com/zoobzio/tracez` - Tracing primitives
- `go.opentelemetry.io/otel/*` - OTLP exporters

## Compatible Libraries

Shotel works with any library exposing the Observable/Traceable interfaces:

- **[pipz](https://github.com/zoobzio/pipz)** - Data pipelines with built-in metrics/tracing
- **[rocco](https://github.com/zoobzio/rocco)** - HTTP framework with observability
- **[hookz](https://github.com/zoobzio/hookz)** - Event hooks
- Custom components implementing the interfaces

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

See [SECURITY.md](SECURITY.md) for security policy and vulnerability reporting.
