package shotel

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	serviceName := "test-service"
	cfg := DefaultConfig(serviceName)

	if cfg.ServiceName != serviceName {
		t.Errorf("expected ServiceName %q, got %q", serviceName, cfg.ServiceName)
	}

	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("expected Endpoint %q, got %q", "localhost:4317", cfg.Endpoint)
	}

	expectedInterval := 10 * time.Second
	if cfg.MetricsInterval != expectedInterval {
		t.Errorf("expected MetricsInterval %v, got %v", expectedInterval, cfg.MetricsInterval)
	}

	if cfg.Insecure != false {
		t.Errorf("expected Insecure false, got true")
	}
}

func TestConfig_CustomValues(t *testing.T) {
	cfg := &Config{
		ServiceName:     "custom-service",
		Endpoint:        "otel-collector:4317",
		MetricsInterval: 30 * time.Second,
		Insecure:        true,
	}

	if cfg.ServiceName != "custom-service" {
		t.Errorf("expected ServiceName %q, got %q", "custom-service", cfg.ServiceName)
	}

	if cfg.Endpoint != "otel-collector:4317" {
		t.Errorf("expected Endpoint %q, got %q", "otel-collector:4317", cfg.Endpoint)
	}

	if cfg.MetricsInterval != 30*time.Second {
		t.Errorf("expected MetricsInterval %v, got %v", 30*time.Second, cfg.MetricsInterval)
	}

	if cfg.Insecure != true {
		t.Errorf("expected Insecure true, got false")
	}
}
