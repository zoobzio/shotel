package shotel

import (
	"errors"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

func TestFieldsToAttributes(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	testErr := errors.New("test error")

	tests := []struct {
		name     string
		fields   []capitan.Field
		wantLen  int
		validate func(t *testing.T, attrs []log.KeyValue)
	}{
		{
			name:    "empty fields",
			fields:  []capitan.Field{},
			wantLen: 0,
		},
		{
			name: "string field",
			fields: []capitan.Field{
				capitan.NewStringKey("msg").Field("hello"),
			},
			wantLen: 1,
			validate: func(t *testing.T, attrs []log.KeyValue) {
				if attrs[0].Key != "msg" {
					t.Errorf("expected key 'msg', got %q", attrs[0].Key)
				}
			},
		},
		{
			name: "int variants",
			fields: []capitan.Field{
				capitan.NewIntKey("int").Field(42),
				capitan.NewInt32Key("int32").Field(int32(32)),
				capitan.NewInt64Key("int64").Field(int64(64)),
			},
			wantLen: 3,
		},
		{
			name: "uint variants",
			fields: []capitan.Field{
				capitan.NewUintKey("uint").Field(uint(42)),
				capitan.NewUint32Key("uint32").Field(uint32(32)),
				capitan.NewUint64Key("uint64").Field(uint64(64)),
			},
			wantLen: 3,
		},
		{
			name: "float variants",
			fields: []capitan.Field{
				capitan.NewFloat32Key("float32").Field(float32(3.14)),
				capitan.NewFloat64Key("float64").Field(float64(2.718)),
			},
			wantLen: 2,
		},
		{
			name: "bool field",
			fields: []capitan.Field{
				capitan.NewBoolKey("flag").Field(true),
			},
			wantLen: 1,
		},
		{
			name: "time field",
			fields: []capitan.Field{
				capitan.NewTimeKey("timestamp").Field(now),
			},
			wantLen: 1,
			validate: func(t *testing.T, attrs []log.KeyValue) {
				if attrs[0].Key != "timestamp" {
					t.Errorf("expected key 'timestamp', got %q", attrs[0].Key)
				}
			},
		},
		{
			name: "duration field",
			fields: []capitan.Field{
				capitan.NewDurationKey("elapsed").Field(5 * time.Second),
			},
			wantLen: 1,
		},
		{
			name: "bytes field",
			fields: []capitan.Field{
				capitan.NewBytesKey("data").Field([]byte("hello")),
			},
			wantLen: 1,
		},
		{
			name: "error field",
			fields: []capitan.Field{
				capitan.NewErrorKey("err").Field(testErr),
			},
			wantLen: 1,
			validate: func(t *testing.T, attrs []log.KeyValue) {
				if attrs[0].Key != "err" {
					t.Errorf("expected key 'err', got %q", attrs[0].Key)
				}
			},
		},
		{
			name: "mixed fields",
			fields: []capitan.Field{
				capitan.NewStringKey("msg").Field("hello"),
				capitan.NewIntKey("count").Field(42),
				capitan.NewBoolKey("active").Field(true),
				capitan.NewFloat64Key("ratio").Field(0.95),
			},
			wantLen: 4,
		},
		{
			name: "custom field type (skipped)",
			fields: []capitan.Field{
				capitan.NewKey[struct{}]("custom", "test.Custom").Field(struct{}{}),
			},
			wantLen: 0, // Custom types are skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := fieldsToAttributes(tt.fields)

			if len(attrs) != tt.wantLen {
				t.Errorf("expected %d attributes, got %d", tt.wantLen, len(attrs))
			}

			if tt.validate != nil {
				tt.validate(t, attrs)
			}
		})
	}
}

func TestFieldsToAttributesAllTypes(t *testing.T) {
	// Test that all built-in capitan types are handled
	fields := []capitan.Field{
		capitan.NewStringKey("string").Field("value"),
		capitan.NewIntKey("int").Field(1),
		capitan.NewInt32Key("int32").Field(int32(2)),
		capitan.NewInt64Key("int64").Field(int64(3)),
		capitan.NewUintKey("uint").Field(uint(4)),
		capitan.NewUint32Key("uint32").Field(uint32(5)),
		capitan.NewUint64Key("uint64").Field(uint64(6)),
		capitan.NewFloat32Key("float32").Field(float32(7.0)),
		capitan.NewFloat64Key("float64").Field(float64(8.0)),
		capitan.NewBoolKey("bool").Field(true),
		capitan.NewTimeKey("time").Field(time.Now()),
		capitan.NewDurationKey("duration").Field(time.Second),
		capitan.NewBytesKey("bytes").Field([]byte("data")),
		capitan.NewErrorKey("error").Field(errors.New("err")),
	}

	attrs := fieldsToAttributes(fields)

	// All 14 built-in types should be converted
	if len(attrs) != 14 {
		t.Errorf("expected 14 attributes, got %d", len(attrs))
	}

	// Verify keys are preserved
	keys := make(map[string]bool)
	for _, attr := range attrs {
		keys[attr.Key] = true
	}

	expectedKeys := []string{
		"string", "int", "int32", "int64",
		"uint", "uint32", "uint64",
		"float32", "float64",
		"bool", "time", "duration", "bytes", "error",
	}

	for _, key := range expectedKeys {
		if !keys[key] {
			t.Errorf("missing key: %s", key)
		}
	}
}

func TestFieldsToMetricAttributes(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	testErr := errors.New("test error")

	tests := []struct {
		name    string
		fields  []capitan.Field
		wantLen int
	}{
		{
			name:    "empty fields",
			fields:  []capitan.Field{},
			wantLen: 0,
		},
		{
			name: "string field",
			fields: []capitan.Field{
				capitan.NewStringKey("label").Field("production"),
			},
			wantLen: 1,
		},
		{
			name: "int variants",
			fields: []capitan.Field{
				capitan.NewIntKey("int").Field(42),
				capitan.NewInt32Key("int32").Field(int32(32)),
				capitan.NewInt64Key("int64").Field(int64(64)),
			},
			wantLen: 3,
		},
		{
			name: "uint variants",
			fields: []capitan.Field{
				capitan.NewUintKey("uint").Field(uint(42)),
				capitan.NewUint32Key("uint32").Field(uint32(32)),
				capitan.NewUint64Key("uint64").Field(uint64(64)),
			},
			wantLen: 3,
		},
		{
			name: "float variants",
			fields: []capitan.Field{
				capitan.NewFloat32Key("float32").Field(float32(3.14)),
				capitan.NewFloat64Key("float64").Field(float64(2.718)),
			},
			wantLen: 2,
		},
		{
			name: "bool field",
			fields: []capitan.Field{
				capitan.NewBoolKey("enabled").Field(true),
			},
			wantLen: 1,
		},
		{
			name: "time field",
			fields: []capitan.Field{
				capitan.NewTimeKey("timestamp").Field(now),
			},
			wantLen: 1,
		},
		{
			name: "duration field",
			fields: []capitan.Field{
				capitan.NewDurationKey("elapsed").Field(5 * time.Second),
			},
			wantLen: 1,
		},
		{
			name: "bytes field",
			fields: []capitan.Field{
				capitan.NewBytesKey("payload").Field([]byte("data")),
			},
			wantLen: 1,
		},
		{
			name: "error field",
			fields: []capitan.Field{
				capitan.NewErrorKey("error").Field(testErr),
			},
			wantLen: 1,
		},
		{
			name: "mixed fields as metric dimensions",
			fields: []capitan.Field{
				capitan.NewStringKey("env").Field("prod"),
				capitan.NewStringKey("region").Field("us-east"),
				capitan.NewIntKey("shard").Field(5),
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := fieldsToMetricAttributes(tt.fields)

			if len(attrs) != tt.wantLen {
				t.Errorf("expected %d metric attributes, got %d", tt.wantLen, len(attrs))
			}
		})
	}
}

func TestFieldsToMetricAttributes_AllBuiltinTypes(t *testing.T) {
	// Test that all built-in capitan types convert to metric attributes
	fields := []capitan.Field{
		capitan.NewStringKey("string").Field("value"),
		capitan.NewIntKey("int").Field(1),
		capitan.NewInt32Key("int32").Field(int32(2)),
		capitan.NewInt64Key("int64").Field(int64(3)),
		capitan.NewUintKey("uint").Field(uint(4)),
		capitan.NewUint32Key("uint32").Field(uint32(5)),
		capitan.NewUint64Key("uint64").Field(uint64(6)),
		capitan.NewFloat32Key("float32").Field(float32(7.0)),
		capitan.NewFloat64Key("float64").Field(float64(8.0)),
		capitan.NewBoolKey("bool").Field(true),
		capitan.NewTimeKey("time").Field(time.Now()),
		capitan.NewDurationKey("duration").Field(time.Second),
		capitan.NewBytesKey("bytes").Field([]byte("data")),
		capitan.NewErrorKey("error").Field(errors.New("err")),
	}

	attrs := fieldsToMetricAttributes(fields)

	// All 14 built-in types should be converted
	if len(attrs) != 14 {
		t.Errorf("expected 14 metric attributes, got %d", len(attrs))
	}
}
