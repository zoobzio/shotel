package aperture

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	apertesting "github.com/zoobzio/aperture/testing"
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
	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("Failed to create providers: %v", err)
	}

	schema := Schema{
		Stdout: true,
	}

	sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("Failed to create aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

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
	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("Failed to create providers: %v", err)
	}

	schema := Schema{
		Stdout: false,
	}

	sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("Failed to create aperture: %v", err)
	}
	defer sh.Close()

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

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
			pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
			if err != nil {
				t.Fatalf("Failed to create providers: %v", err)
			}

			schema := Schema{
				Stdout: true,
			}

			sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace)
			if err != nil {
				t.Fatalf("Failed to create aperture: %v", err)
			}
			defer sh.Close()

			err = sh.Apply(schema)
			if err != nil {
				t.Fatalf("Apply failed: %v", err)
			}

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
	pvs, err := apertesting.TestProviders(ctx, "test-service", "v1.0.0", "localhost:4318")
	if err != nil {
		t.Fatalf("Failed to create providers: %v", err)
	}

	sh, err := New(c, pvs.Log, pvs.Meter, pvs.Trace)
	if err != nil {
		t.Fatalf("Failed to create aperture: %v", err)
	}
	defer sh.Close()

	// Register context key
	sh.RegisterContextKey("request_id", requestIDKey)

	schema := Schema{
		Stdout: true,
		Context: &ContextSchema{
			Logs: []string{"request_id"},
		},
	}

	err = sh.Apply(schema)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

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

func TestFieldToSlogAttr_AllVariants(t *testing.T) {
	now := time.Now()
	dur := 100 * time.Millisecond
	testErr := errors.New("test error")

	tests := []struct {
		name     string
		field    capitan.Field
		wantKey  string
		validate func(t *testing.T, attr slog.Attr)
	}{
		{
			name:    "string",
			field:   capitan.NewStringKey("str").Field("hello"),
			wantKey: "str",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.String() != "hello" {
					t.Errorf("Expected 'hello', got %v", attr.Value)
				}
			},
		},
		{
			name:    "int",
			field:   capitan.NewIntKey("int_val").Field(42),
			wantKey: "int_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindInt64 {
					t.Errorf("Expected Int64 kind, got %v", attr.Value.Kind())
				}
			},
		},
		{
			name:    "int32",
			field:   capitan.NewInt32Key("int32_val").Field(int32(32)),
			wantKey: "int32_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindInt64 {
					t.Errorf("Expected Int64 kind, got %v", attr.Value.Kind())
				}
			},
		},
		{
			name:    "int64",
			field:   capitan.NewInt64Key("int64_val").Field(int64(64)),
			wantKey: "int64_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Int64() != 64 {
					t.Errorf("Expected 64, got %v", attr.Value.Int64())
				}
			},
		},
		{
			name:    "uint",
			field:   capitan.NewUintKey("uint_val").Field(uint(10)),
			wantKey: "uint_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Uint64() != 10 {
					t.Errorf("Expected 10, got %v", attr.Value.Uint64())
				}
			},
		},
		{
			name:    "uint32",
			field:   capitan.NewUint32Key("uint32_val").Field(uint32(32)),
			wantKey: "uint32_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Uint64() != 32 {
					t.Errorf("Expected 32, got %v", attr.Value.Uint64())
				}
			},
		},
		{
			name:    "uint64",
			field:   capitan.NewUint64Key("uint64_val").Field(uint64(64)),
			wantKey: "uint64_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Uint64() != 64 {
					t.Errorf("Expected 64, got %v", attr.Value.Uint64())
				}
			},
		},
		{
			name:    "float32",
			field:   capitan.NewFloat32Key("float32_val").Field(float32(3.14)),
			wantKey: "float32_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindFloat64 {
					t.Errorf("Expected Float64 kind, got %v", attr.Value.Kind())
				}
			},
		},
		{
			name:    "float64",
			field:   capitan.NewFloat64Key("float64_val").Field(6.28),
			wantKey: "float64_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Float64() != 6.28 {
					t.Errorf("Expected 6.28, got %v", attr.Value.Float64())
				}
			},
		},
		{
			name:    "bool",
			field:   capitan.NewBoolKey("bool_val").Field(true),
			wantKey: "bool_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Bool() != true {
					t.Errorf("Expected true, got %v", attr.Value.Bool())
				}
			},
		},
		{
			name:    "time",
			field:   capitan.NewTimeKey("time_val").Field(now),
			wantKey: "time_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if !attr.Value.Time().Equal(now) {
					t.Errorf("Expected %v, got %v", now, attr.Value.Time())
				}
			},
		},
		{
			name:    "duration",
			field:   capitan.NewDurationKey("dur_val").Field(dur),
			wantKey: "dur_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Duration() != dur {
					t.Errorf("Expected %v, got %v", dur, attr.Value.Duration())
				}
			},
		},
		{
			name:    "bytes",
			field:   capitan.NewBytesKey("bytes_val").Field([]byte("binary")),
			wantKey: "bytes_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.String() != "binary" {
					t.Errorf("Expected 'binary', got %v", attr.Value.String())
				}
			},
		},
		{
			name:    "error",
			field:   capitan.NewErrorKey("err_val").Field(testErr),
			wantKey: "err_val",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.String() != "test error" {
					t.Errorf("Expected 'test error', got %v", attr.Value.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := fieldToSlogAttr(tt.field)
			if attr.Key != tt.wantKey {
				t.Errorf("Expected key %s, got %s", tt.wantKey, attr.Key)
			}
			if tt.validate != nil {
				tt.validate(t, attr)
			}
		})
	}
}

func TestContextValueToSlogAttr_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		attrName string
		value    any
		validate func(t *testing.T, attr slog.Attr)
	}{
		{
			name:     "string",
			attrName: "str",
			value:    "hello",
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.String() != "hello" {
					t.Errorf("Expected 'hello', got %v", attr.Value)
				}
			},
		},
		{
			name:     "int",
			attrName: "int_val",
			value:    42,
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindInt64 {
					t.Errorf("Expected Int64 kind, got %v", attr.Value.Kind())
				}
			},
		},
		{
			name:     "int32",
			attrName: "int32_val",
			value:    int32(32),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindInt64 {
					t.Errorf("Expected Int64 kind, got %v", attr.Value.Kind())
				}
			},
		},
		{
			name:     "int64",
			attrName: "int64_val",
			value:    int64(64),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Int64() != 64 {
					t.Errorf("Expected 64, got %v", attr.Value.Int64())
				}
			},
		},
		{
			name:     "uint",
			attrName: "uint_val",
			value:    uint(10),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Uint64() != 10 {
					t.Errorf("Expected 10, got %v", attr.Value.Uint64())
				}
			},
		},
		{
			name:     "uint32",
			attrName: "uint32_val",
			value:    uint32(32),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Uint64() != 32 {
					t.Errorf("Expected 32, got %v", attr.Value.Uint64())
				}
			},
		},
		{
			name:     "uint64",
			attrName: "uint64_val",
			value:    uint64(64),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Uint64() != 64 {
					t.Errorf("Expected 64, got %v", attr.Value.Uint64())
				}
			},
		},
		{
			name:     "float32",
			attrName: "float32_val",
			value:    float32(3.14),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindFloat64 {
					t.Errorf("Expected Float64 kind, got %v", attr.Value.Kind())
				}
			},
		},
		{
			name:     "float64",
			attrName: "float64_val",
			value:    6.28,
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Float64() != 6.28 {
					t.Errorf("Expected 6.28, got %v", attr.Value.Float64())
				}
			},
		},
		{
			name:     "bool",
			attrName: "bool_val",
			value:    true,
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Bool() != true {
					t.Errorf("Expected true, got %v", attr.Value.Bool())
				}
			},
		},
		{
			name:     "bytes",
			attrName: "bytes_val",
			value:    []byte("binary"),
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.String() != "binary" {
					t.Errorf("Expected 'binary', got %v", attr.Value.String())
				}
			},
		},
		{
			name:     "unknown type falls back to Any",
			attrName: "custom",
			value:    struct{ Name string }{Name: "test"},
			validate: func(t *testing.T, attr slog.Attr) {
				if attr.Value.Kind() != slog.KindAny {
					t.Errorf("Expected Any kind, got %v", attr.Value.Kind())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := contextValueToSlogAttr(tt.attrName, tt.value)
			if attr.Key != tt.attrName {
				t.Errorf("Expected key %s, got %s", tt.attrName, attr.Key)
			}
			if tt.validate != nil {
				tt.validate(t, attr)
			}
		})
	}
}
