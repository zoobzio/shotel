# Integration Tests

This directory contains integration tests that verify aperture components work together correctly in real-world scenarios.

## Test Categories

### Scenario Tests (`scenarios_test.go`)
Tests complete workflows from capitan events through OTEL signal transformation:

- **Metrics Integration**: Signal → MetricConfig → OTEL Meter
- **Traces Integration**: Start/End signals → TraceConfig → OTEL Spans
- **Logs Integration**: Signal → LogConfig whitelist → OTEL Logger
- **Context Extraction**: Context values → OTEL attributes

### Concurrency Tests (`concurrency_test.go`)
Tests thread-safety and concurrent access patterns:

- Multiple goroutines emitting simultaneously
- Hook and emit during concurrent operations
- Aperture creation and shutdown under load
- Trace correlation with concurrent start/end events

## Running Integration Tests

```bash
# Run all integration tests
go test -v ./testing/integration/...

# Run with race detection (recommended)
go test -v -race ./testing/integration/...

# Run specific test
go test -v -run TestScenario_MetricsCounter ./testing/integration/...
```

## Test Patterns

### Standard Integration Test Structure

```go
func TestScenario_Example(t *testing.T) {
    ctx := context.Background()

    // 1. Setup capitan
    cap := capitan.New()
    defer cap.Shutdown()

    // 2. Define signals and keys
    signal := capitan.NewSignal("test.example", "Example signal")
    key := capitan.NewStringKey("key")

    // 3. Create aperture with test providers
    pvs, err := testing.TestProviders(ctx, "test", "v1", "localhost:4318")
    require.NoError(t, err)
    defer pvs.Shutdown(ctx)

    ap, err := aperture.New(cap, pvs.Log, pvs.Meter, pvs.Trace)
    require.NoError(t, err)
    defer ap.Close()

    // 4. Apply schema configuration
    schema := aperture.Schema{...}
    ap.Apply(schema)

    // 5. Emit events
    cap.Emit(ctx, signal, key.Field("value"))

    // 6. Verify outcomes
    // ...
}
```

### Using Mock Providers for Verification

```go
func TestScenario_WithMocks(t *testing.T) {
    cap := capitan.New()
    defer cap.Shutdown()

    // Create aperture with mock providers
    mockLog := testing.NewMockLoggerProvider()
    ap, err := aperture.New(cap, mockLog, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
    require.NoError(t, err)
    defer ap.Close()

    // Apply schema if needed
    ap.Apply(schema)

    // Emit events...

    // Verify log records
    records := mockLog.Capture().Records()
    assert.Len(t, records, 1)
}
```

## Best Practices

1. **Always use race detection** when running integration tests
2. **Clean up resources** with defer statements
3. **Use timeouts** for async operations
4. **Test both success and failure paths**
5. **Verify complete data flow**, not just endpoints

## Dependencies

Integration tests may require:
- Local OTLP collector (for full end-to-end tests)
- Or use mock providers (for isolated testing)
