# Testing Infrastructure for aperture

This directory contains the comprehensive testing infrastructure for the aperture package, designed to support robust development and maintenance of the capitan-to-OTEL bridge.

## Directory Structure

```
testing/
├── README.md             # This file - testing strategy overview
├── helpers.go            # Shared test utilities and mocks
├── helpers_test.go       # Tests for the test helpers
├── integration/          # Integration and end-to-end tests
│   ├── README.md         # Integration testing documentation
│   └── scenarios_test.go # Real-world scenario tests
└── benchmarks/           # Performance benchmarks
    ├── README.md         # Benchmark documentation
    └── core_test.go      # Core component benchmarks
```

## Testing Strategy

### Unit Tests (Root Package)
- **Location**: Alongside source files (`*_test.go`)
- **Purpose**: Test individual components in isolation
- **Coverage Goal**: High coverage with race detection
- **Focus**: Transform logic, configuration, handlers

### Integration Tests (`testing/integration/`)
- **Purpose**: Test component interactions and real-world scenarios
- **Scope**: Full aperture → capitan → OTEL flows
- **Focus**: Ensure components work together correctly

### Benchmarks (`testing/benchmarks/`)
- **Purpose**: Measure and track performance characteristics
- **Scope**: Event transformation, metric recording, trace correlation
- **Focus**: Prevent performance regressions

### Test Helpers (`testing/helpers.go`)
- **Purpose**: Provide reusable testing utilities for aperture users
- **Scope**: Mock providers, log capture, event capture
- **Focus**: Make testing aperture-based applications easier

## Running Tests

### All Tests
```bash
# Run all tests with coverage
go test -v -coverprofile=coverage.out ./...

# Generate coverage report
go tool cover -html=coverage.out
```

### Unit Tests Only
```bash
# Run only unit tests (root package)
go test -v .
```

### Integration Tests Only
```bash
# Run integration tests
go test -v ./testing/integration/...

# With race detection
go test -v -race ./testing/integration/...
```

### Benchmarks Only
```bash
# Run all benchmarks
go test -v -bench=. ./testing/benchmarks/...

# Run with memory allocation tracking
go test -v -bench=. -benchmem ./testing/benchmarks/...

# Run multiple times for statistical significance
go test -v -bench=. -count=5 ./testing/benchmarks/...
```

## Test Helpers

### TestProviders
Creates OTLP providers configured for testing with insecure connections:

```go
pvs, err := testing.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
if err != nil {
    t.Fatal(err)
}
defer pvs.Shutdown(ctx)
```

### MockLoggerProvider
Captures OTEL log records for verification:

```go
mockLog := testing.NewMockLoggerProvider()
ap, err := aperture.New(cap, mockLog, meterProv, traceProv, config)

// ... emit events ...

records := mockLog.Capture().Records()
if len(records) != 1 {
    t.Errorf("expected 1 record, got %d", len(records))
}
```

### LogCapture
Thread-safe log record capture with wait functionality:

```go
capture := testing.NewLogCapture()

// Wait for records with timeout
if !capture.WaitForCount(5, time.Second) {
    t.Fatal("timeout waiting for records")
}
```

### EventCapture
Captures capitan events for verification:

```go
capture := testing.NewEventCapture()
cap.Hook(signal, capture.Handler())

// ... emit events ...

events := capture.Events()
```

### TestAperture
Creates an Aperture instance with mock providers for unit testing:

```go
ap, mockLog, err := testing.TestAperture(cap, config)
if err != nil {
    t.Fatal(err)
}
defer ap.Close()

// mockLog.Capture() provides access to captured logs
```

## Best Practices

### Test Organization
1. **Hierarchical naming**: Use descriptive test names that form a hierarchy
2. **Consistent structure**: Follow the same pattern across all test files
3. **Isolated tests**: Each test should be completely independent
4. **Fast tests**: Keep unit tests fast, put slow tests in integration

### OTEL Testing
1. **Use mock providers** for unit tests (avoid network calls)
2. **Test both emission and transformation**
3. **Verify attribute types and values**
4. **Include context propagation scenarios**

### Concurrency Testing
1. **Use `-race` flag regularly**
2. **Test concurrent emit/close operations**
3. **Verify proper cleanup in failure scenarios**
4. **Test trace correlation under load**

## Continuous Integration

### Pre-commit Checks
```bash
# Run this before committing
make test          # All tests with race detection
make lint          # Code quality checks
make coverage      # Coverage verification
```

### CI Pipeline Requirements
- All tests must pass
- No race conditions detected
- Benchmarks must not regress significantly

---

This testing infrastructure ensures aperture remains reliable, performant, and easy to use while providing comprehensive tools for users to test their own aperture-based applications.
