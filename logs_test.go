package shotel

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/zoobzio/hookz"
)

func TestCreateLogHandler(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("log-handler-test")
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

	// Create log handler
	handler := shotel.CreateLogHandler()

	// Test with simple event
	err = handler(ctx, "test event")
	if err != nil {
		t.Errorf("Handler failed: %v", err)
	}

	// Test with structured event
	type TestEvent struct {
		Name  string
		Count int
	}
	event := TestEvent{Name: "test", Count: 42}
	err = handler(ctx, event)
	if err != nil {
		t.Errorf("Handler failed with struct: %v", err)
	}
}

func TestCreateLogHandlerWithHookz(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("hookz-test")
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

	// Create hookz instance
	type Event struct {
		Type string
		Data map[string]any
	}

	hooks := hookz.New[Event]()
	defer func() { _ = hooks.Close() }() //nolint:errcheck // Test cleanup

	// Register shotel log handler with hookz
	logHandler := shotel.CreateLogHandler()
	_, err = hooks.Hook("event.occurred", func(ctx context.Context, event Event) error {
		return logHandler(ctx, event)
	})
	if err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	// Emit event
	event := Event{
		Type: "test.event",
		Data: map[string]any{"key": "value", "count": 123},
	}

	err = hooks.Emit(ctx, "event.occurred", event)
	if err != nil {
		t.Errorf("Failed to emit event: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestCreateSlogHandler(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("slog-test")
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

	// Create slog logger with shotel handler
	logger := slog.New(shotel.CreateSlogHandler())

	// Test various log levels
	logger.Info("info message", "key", "value", "count", 42)
	logger.Warn("warning message", "warning", true)
	logger.Error("error message", "error_code", 500)
	logger.Debug("debug message", "debug", "data")

	time.Sleep(100 * time.Millisecond)
}

func TestSlogHandlerWithAttrs(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("slog-attrs-test")
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

	// Create logger with default attributes
	handler := shotel.CreateSlogHandler()
	handlerWithAttrs := handler.WithAttrs([]slog.Attr{
		slog.String("service", "test-service"),
		slog.Int("version", 1),
	})

	logger := slog.New(handlerWithAttrs)

	logger.Info("message with default attrs", "request_id", "123")

	time.Sleep(100 * time.Millisecond)
}

func TestSlogHandlerWithGroup(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("slog-group-test")
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

	// Create logger with group
	handler := shotel.CreateSlogHandler()
	handlerWithGroup := handler.WithGroup("request")

	logger := slog.New(handlerWithGroup)

	logger.Info("grouped message", "method", "GET", "path", "/api/users")

	time.Sleep(100 * time.Millisecond)
}

func TestSlogHandlerAllTypes(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("slog-types-test")
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

	logger := slog.New(shotel.CreateSlogHandler())

	// Test all slog value types
	logger.Info("all types",
		slog.String("string", "value"),
		slog.Int("int", 42),
		slog.Int64("int64", 9223372036854775807),
		slog.Uint64("uint64", 18446744073709551615),
		slog.Float64("float64", 3.14159),
		slog.Bool("bool", true),
		slog.Duration("duration", 5*time.Second),
		slog.Time("time", time.Now()),
	)

	time.Sleep(100 * time.Millisecond)
}

func TestSetGlobalSlogHandler(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("global-slog-test")
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

	// Set global handler
	shotel.SetGlobalSlogHandler()

	// Use global slog functions
	slog.Info("global slog message", "test", true)
	slog.Warn("global warning", "code", 123)

	time.Sleep(100 * time.Millisecond)
}

func TestSetGlobalOtelLogger(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultConfig("global-otel-test")
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

	// Set global OTLP logger provider
	shotel.SetGlobalOtelLogger()

	// Verify no panic and method completes
	time.Sleep(10 * time.Millisecond)
}
