// Package shotel provides an OpenTelemetry bridge for Go observability libraries.
package shotel

import (
	"log/slog"
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

	// Logger for shotel operations (default: slog.Default())
	Logger *slog.Logger
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(serviceName string) *Config {
	return &Config{
		ServiceName:     serviceName,
		Endpoint:        "localhost:4317",
		MetricsInterval: 10 * time.Second,
		Insecure:        false,
		Logger:          slog.Default(),
	}
}

// WithLogger sets the logger for shotel operations.
func (c *Config) WithLogger(logger *slog.Logger) *Config {
	c.Logger = logger
	return c
}
