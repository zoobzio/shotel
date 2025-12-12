# Performance Benchmarks

This directory contains performance benchmarks for the aperture package. These benchmarks help track performance characteristics and prevent regressions.

## Benchmark Categories

### Core Benchmarks (`core_test.go`)

| Benchmark | Description |
|-----------|-------------|
| `BenchmarkEmit_NoConfig` | Event emission without any aperture config |
| `BenchmarkEmit_WithMetrics` | Event emission with metric recording |
| `BenchmarkEmit_WithLogs` | Event emission with log transformation |
| `BenchmarkEmit_WithTraces` | Event emission with trace correlation |
| `BenchmarkTransform_Fields` | Field transformation to OTEL attributes |

## Running Benchmarks

### Basic Benchmark Run
```bash
go test -bench=. ./testing/benchmarks/...
```

### With Memory Allocation Tracking
```bash
go test -bench=. -benchmem ./testing/benchmarks/...
```

### Multiple Iterations for Statistical Significance
```bash
go test -bench=. -count=5 ./testing/benchmarks/...
```

### Specific Benchmark
```bash
go test -bench=BenchmarkEmit_WithMetrics ./testing/benchmarks/...
```

### Extended Benchmark Time
```bash
go test -bench=. -benchtime=5s ./testing/benchmarks/...
```

## Comparing Benchmarks

### Save Baseline
```bash
go test -bench=. -count=5 ./testing/benchmarks/... > baseline.txt
```

### Compare After Changes
```bash
go test -bench=. -count=5 ./testing/benchmarks/... > current.txt
benchstat baseline.txt current.txt
```

Install benchstat:
```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

## Performance Expectations

### Event Emission
- **No config**: ~1-2μs per event
- **With metrics**: ~2-5μs per event (varies by instrument type)
- **With logs**: ~2-5μs per event
- **With traces**: ~3-10μs per event (correlation overhead)

### Memory Allocations
- **Field transformation**: Minimal allocations (reuses buffers)
- **Metric recording**: Zero allocations on hot path
- **Log emission**: 1-2 allocations per event

## Best Practices

1. **Run multiple times**: Use `-count=5` or higher for stable results
2. **Control environment**: Close other applications, use consistent hardware
3. **Use benchstat**: For statistical comparison between runs
4. **Track trends**: Store benchmark results over time
5. **Profile when needed**: Use `-cpuprofile` and `-memprofile` for investigation

## Profiling

### CPU Profile
```bash
go test -bench=BenchmarkEmit_WithMetrics -cpuprofile=cpu.prof ./testing/benchmarks/...
go tool pprof cpu.prof
```

### Memory Profile
```bash
go test -bench=BenchmarkEmit_WithMetrics -memprofile=mem.prof ./testing/benchmarks/...
go tool pprof mem.prof
```

### Trace
```bash
go test -bench=BenchmarkEmit_WithMetrics -trace=trace.out ./testing/benchmarks/...
go tool trace trace.out
```
