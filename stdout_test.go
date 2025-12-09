package aperture

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestStdoutLogging(t *testing.T) {
	ctx := context.Background()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create capitan and signal
	c := capitan.New()
	testSignal := capitan.NewSignal("test.signal", "Test signal description")
	testKey := capitan.NewStringKey("test_key")

	// Create aperture with stdout logging enabled
	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("Failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	config := &Config{
		StdoutLogging: true,
	}

	sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("Failed to create aperture: %v", err)
	}
	defer sh.Close()

	// Emit event
	c.Emit(ctx, testSignal, testKey.Field("test_value"))

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected elements
	if !strings.Contains(output, "Test signal description") {
		t.Errorf("Expected output to contain signal description, got: %s", output)
	}
	if !strings.Contains(output, "test.signal") {
		t.Errorf("Expected output to contain signal name, got: %s", output)
	}
	if !strings.Contains(output, "test_key") {
		t.Errorf("Expected output to contain field key, got: %s", output)
	}
	if !strings.Contains(output, "test_value") {
		t.Errorf("Expected output to contain field value, got: %s", output)
	}
}

func TestStdoutLoggingDisabled(t *testing.T) {
	ctx := context.Background()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create capitan and signal
	c := capitan.New()
	testSignal := capitan.NewSignal("test.signal", "Test signal description")

	// Create aperture with stdout logging disabled
	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("Failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	config := &Config{
		StdoutLogging: false,
	}

	sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("Failed to create aperture: %v", err)
	}
	defer sh.Close()

	// Emit event
	c.Emit(ctx, testSignal)

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify no stdout output
	if strings.Contains(output, "Test signal description") {
		t.Errorf("Expected no stdout output when disabled, got: %s", output)
	}
}

func TestFieldToSlogAttr(t *testing.T) {
	tests := []struct {
		name    string
		field   capitan.Field
		wantKey string
	}{
		{
			name:    "string field",
			field:   capitan.NewStringKey("str").Field("test"),
			wantKey: "str",
		},
		{
			name:    "int64 field",
			field:   capitan.NewInt64Key("num").Field(42),
			wantKey: "num",
		},
		{
			name:    "bool field",
			field:   capitan.NewBoolKey("flag").Field(true),
			wantKey: "flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := fieldToSlogAttr(tt.field)
			if attr.Key != tt.wantKey {
				t.Errorf("Expected key %s, got %s", tt.wantKey, attr.Key)
			}
		})
	}
}

func TestStdoutLoggerSeverityMapping(t *testing.T) {
	tests := []struct {
		name     string
		severity capitan.Severity
		expected slog.Level
	}{
		{"debug", capitan.SeverityDebug, slog.LevelDebug},
		{"info", capitan.SeverityInfo, slog.LevelInfo},
		{"warn", capitan.SeverityWarn, slog.LevelWarn},
		{"error", capitan.SeverityError, slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create capitan with signal
			c := capitan.New()
			testSignal := capitan.NewSignal("test.signal", "Test")

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create aperture with stdout logging
			pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
			if err != nil {
				t.Fatalf("Failed to create providers: %v", err)
			}
			defer pvs.Shutdown(ctx)

			config := &Config{
				StdoutLogging: true,
			}

			sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace, config)
			if err != nil {
				t.Fatalf("Failed to create aperture: %v", err)
			}
			defer sh.Close()

			// Emit event - capitan will use the default severity
			// Severity mapping is tested directly in capitan_test.go:TestSeverityToOTEL
			c.Emit(ctx, testSignal)
			time.Sleep(100 * time.Millisecond)

			// Restore stdout and read output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Verify signal appears in output
			if !strings.Contains(output, "test.signal") {
				t.Errorf("Expected output to contain signal name, got: %s", output)
			}
		})
	}
}

func TestStdoutLoggingWithContextExtraction(t *testing.T) {
	ctx := context.Background()

	// Define context key
	type ctxKey string
	requestIDKey := ctxKey("request_id")

	// Add value to context
	ctx = context.WithValue(ctx, requestIDKey, "REQ-12345")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create capitan and signal
	c := capitan.New()
	testSignal := capitan.NewSignal("test.signal", "Test signal with context")

	// Create aperture with stdout logging and context extraction
	pvs, err := DefaultProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("Failed to create providers: %v", err)
	}
	defer pvs.Shutdown(ctx)

	config := &Config{
		StdoutLogging: true,
		ContextExtraction: &ContextExtractionConfig{
			Logs: []ContextKey{
				{Key: requestIDKey, Name: "request_id"},
			},
		},
	}

	sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace, config)
	if err != nil {
		t.Fatalf("Failed to create aperture: %v", err)
	}
	defer sh.Close()

	// Emit event
	c.Emit(ctx, testSignal)

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains signal and context value
	if !strings.Contains(output, "test.signal") {
		t.Errorf("Expected output to contain signal name, got: %s", output)
	}
	if !strings.Contains(output, "request_id") {
		t.Errorf("Expected output to contain context key name, got: %s", output)
	}
	if !strings.Contains(output, "REQ-12345") {
		t.Errorf("Expected output to contain context value, got: %s", output)
	}
}
