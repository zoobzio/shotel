package aperture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSchemaFromYAML(t *testing.T) {
	yaml := `
metrics:
  - signal: TestStart
    name: test.started
    type: counter
  - signal: TestEnd
    name: test.duration
    type: histogram
    value_key: duration

traces:
  - start: TestStart
    end: TestEnd
    correlation_key: id
    span_name: test-operation
    span_timeout: 5m

logs:
  whitelist:
    - TestStart

stdout: true
`

	schema, err := LoadSchemaFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadSchemaFromYAML failed: %v", err)
	}

	// Validate structure
	if len(schema.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(schema.Metrics))
	}
	if len(schema.Traces) != 1 {
		t.Errorf("expected 1 trace, got %d", len(schema.Traces))
	}
	if schema.Logs == nil || len(schema.Logs.Whitelist) != 1 {
		t.Error("expected logs whitelist with 1 entry")
	}
	if !schema.Stdout {
		t.Error("expected stdout to be true")
	}

	// Validate metric values
	if schema.Metrics[0].Signal != "TestStart" {
		t.Errorf("expected signal TestStart, got %s", schema.Metrics[0].Signal)
	}
	if schema.Metrics[1].ValueKey != "duration" {
		t.Errorf("expected value_key duration, got %s", schema.Metrics[1].ValueKey)
	}

	// Validate trace values
	if schema.Traces[0].SpanTimeout != "5m" {
		t.Errorf("expected span_timeout 5m, got %s", schema.Traces[0].SpanTimeout)
	}
}

func TestLoadSchemaFromYAML_Invalid(t *testing.T) {
	_, err := LoadSchemaFromYAML([]byte("invalid: [yaml"))
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadSchemaFromJSON(t *testing.T) {
	json := `{
		"metrics": [{"signal": "TestStart", "name": "test.started"}],
		"stdout": true
	}`

	schema, err := LoadSchemaFromJSON([]byte(json))
	if err != nil {
		t.Fatalf("LoadSchemaFromJSON failed: %v", err)
	}

	if len(schema.Metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(schema.Metrics))
	}
	if !schema.Stdout {
		t.Error("expected stdout to be true")
	}
}

func TestLoadSchemaFromJSON_Invalid(t *testing.T) {
	_, err := LoadSchemaFromJSON([]byte("{invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadSchemaFromFile(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
	}{
		{
			name:     "yaml file",
			filename: "config.yaml",
			content:  "metrics:\n  - signal: Test\n    name: test\n",
			wantErr:  false,
		},
		{
			name:     "yml file",
			filename: "config.yml",
			content:  "metrics:\n  - signal: Test\n    name: test\n",
			wantErr:  false,
		},
		{
			name:     "json file",
			filename: "config.json",
			content:  `{"metrics": [{"signal": "Test", "name": "test"}]}`,
			wantErr:  false,
		},
		{
			name:     "unknown extension defaults to yaml",
			filename: "config.txt",
			content:  "metrics:\n  - signal: Test\n    name: test\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			schema, err := LoadSchemaFromFile(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadSchemaFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(schema.Metrics) != 1 {
				t.Errorf("expected 1 metric, got %d", len(schema.Metrics))
			}
		})
	}
}

func TestLoadSchemaFromFile_NotFound(t *testing.T) {
	_, err := LoadSchemaFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSchemaValidate(t *testing.T) {
	tests := []struct {
		name    string
		schema  Schema
		wantErr bool
	}{
		{
			name:    "empty schema is valid",
			schema:  Schema{},
			wantErr: false,
		},
		{
			name: "valid metric",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test", Name: "test"}},
			},
			wantErr: false,
		},
		{
			name: "metric missing signal",
			schema: Schema{
				Metrics: []MetricSchema{{Name: "test"}},
			},
			wantErr: true,
		},
		{
			name: "metric missing name",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test"}},
			},
			wantErr: true,
		},
		{
			name: "gauge missing value_key",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test", Name: "test", Type: "gauge"}},
			},
			wantErr: true,
		},
		{
			name: "gauge with value_key is valid",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test", Name: "test", Type: "gauge", ValueKey: "val"}},
			},
			wantErr: false,
		},
		{
			name: "histogram missing value_key",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test", Name: "test", Type: "histogram"}},
			},
			wantErr: true,
		},
		{
			name: "updowncounter missing value_key",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test", Name: "test", Type: "updowncounter"}},
			},
			wantErr: true,
		},
		{
			name: "counter without value_key is valid",
			schema: Schema{
				Metrics: []MetricSchema{{Signal: "Test", Name: "test", Type: "counter"}},
			},
			wantErr: false,
		},
		{
			name: "valid trace",
			schema: Schema{
				Traces: []TraceSchema{{Start: "A", End: "B", CorrelationKey: "id"}},
			},
			wantErr: false,
		},
		{
			name: "trace missing start",
			schema: Schema{
				Traces: []TraceSchema{{End: "B", CorrelationKey: "id"}},
			},
			wantErr: true,
		},
		{
			name: "trace missing end",
			schema: Schema{
				Traces: []TraceSchema{{Start: "A", CorrelationKey: "id"}},
			},
			wantErr: true,
		},
		{
			name: "trace missing correlation_key",
			schema: Schema{
				Traces: []TraceSchema{{Start: "A", End: "B"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadSchemaFromYAML_Context(t *testing.T) {
	yaml := `
context:
  logs:
    - request_id
    - user_id
  metrics:
    - tenant_id
  traces:
    - correlation_id
`

	schema, err := LoadSchemaFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadSchemaFromYAML failed: %v", err)
	}

	if schema.Context == nil {
		t.Fatal("expected context to be set")
	}
	if len(schema.Context.Logs) != 2 {
		t.Errorf("expected 2 log context keys, got %d", len(schema.Context.Logs))
	}
	if len(schema.Context.Metrics) != 1 {
		t.Errorf("expected 1 metric context key, got %d", len(schema.Context.Metrics))
	}
	if len(schema.Context.Traces) != 1 {
		t.Errorf("expected 1 trace context key, got %d", len(schema.Context.Traces))
	}
}
