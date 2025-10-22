// Package shotel provides an OpenTelemetry bridge for Go observability libraries.
package shotel

import (
	"time"
)

// Config holds configuration for the OpenTelemetry bridge.
type Config struct {
	// ServiceName identifies this service in telemetry data
	ServiceName string

	// Endpoint is the OTLP collector endpoint (default: localhost:4317)
	Endpoint string

	// MetricsInterval is how often to poll and export metrics (default: 10s)
	MetricsInterval time.Duration

	// Insecure disables TLS for the OTLP connection (default: false)
	Insecure bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(serviceName string) *Config {
	return &Config{
		ServiceName:     serviceName,
		Endpoint:        "localhost:4317",
		MetricsInterval: 10 * time.Second,
		Insecure:        false,
	}
}
