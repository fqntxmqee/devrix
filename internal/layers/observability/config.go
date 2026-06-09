package observability

import (
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/coverage"
	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// Config represents the observability configuration
type Config struct {
	Enabled  bool                    `yaml:"enabled"`
	Tracing  settings.TracingConfig   `yaml:"tracing"`
	Metrics  settings.MetricsConfig `yaml:"metrics"`
	Logging  LoggingConfig           `yaml:"logging"`
	LLM      LLMConfig              `yaml:"llm"`
	Health   HealthConfig           `yaml:"health"`
	Coverage coverage.Config        `yaml:"coverage"`
}

// LLMConfig controls LLM request/response capture for tracing and local logs.
type LLMConfig struct {
	LogContent bool   `yaml:"log_content"`
	LogDir     string `yaml:"log_dir"`
}

// TracingConfig holds tracing configuration
type TracingConfig = settings.TracingConfig

// SamplingConfig holds sampling configuration
type SamplingConfig = settings.SamplingConfig

// OTLPConfig holds OTLP exporter configuration
type OTLPConfig = settings.OTLPConfig

// MetricsConfig holds metrics configuration
type MetricsConfig = settings.MetricsConfig

// LabelsConfig holds label allowlist/blocklist configuration
type LabelsConfig = settings.LabelsConfig

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Enabled       bool             `yaml:"enabled"`
	Level         string           `yaml:"level"`
	Format        string           `yaml:"format"`
	IncludeTrace  bool             `yaml:"include_trace_id"`
	Sampling     LogSamplingConfig `yaml:"sampling"`
	Redactor     RedactorConfig   `yaml:"redactor"`
}

// LogSamplingConfig holds log sampling configuration
type LogSamplingConfig struct {
	Enabled          bool `yaml:"enabled"`
	MaxEntriesPerSpan int `yaml:"max_entries_per_span"`
}

// RedactorConfig holds secret redaction configuration
type RedactorConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Patterns []string `yaml:"patterns"`
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		Tracing: TracingConfig{
			Enabled:        true,
			ServiceName:    "devrix",
			ServiceVersion: "1.0.0",
			Exporter:       "otlp",
			Sampling: SamplingConfig{
				Type: "always_on",
				Rate: 1.0,
			},
			OTLP: OTLPConfig{
				Endpoint: "http://localhost:4318/v1/traces",
				Insecure: true,
				Timeout: 5 * time.Second,
			},
		},
		Metrics: MetricsConfig{
			Enabled:  true,
			Exporter: "prometheus",
			Endpoint: "/metrics",
			Labels: LabelsConfig{
				Allowlist: []string{
					"provider", "model", "adapter", "tool",
					"risk_level", "status", "direction", "decision", "error_type",
				},
				Blocklist: []string{
					"session_id", "user_id", "api_key",
				},
			},
		},
		Logging: LoggingConfig{
			Enabled:       true,
			Level:        "info",
			Format:       "json",
			IncludeTrace: true,
			Sampling: LogSamplingConfig{
				Enabled:          true,
				MaxEntriesPerSpan: 100,
			},
			Redactor: RedactorConfig{
				Enabled: true,
				Patterns: []string{
					"password", "token", "secret", "api_key",
					"authorization", "private_key", "access_token",
				},
			},
		},
		Health: HealthConfig{
			Enabled:  true,
			Endpoint: "/health",
		},
		LLM: LLMConfig{
			LogContent: false,
			LogDir:     "~/.devrix/logs/llm",
		},
		Coverage: coverage.DefaultConfig(),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate tracing exporter
	if c.Tracing.Exporter != "console" && c.Tracing.Exporter != "otlp" && c.Tracing.Exporter != "null" && c.Tracing.Exporter != "memory" {
		return fmt.Errorf("tracing.exporter must be one of: console, otlp, null")
	}

	// Validate sampling type
	if c.Tracing.Sampling.Type != "always_on" && c.Tracing.Sampling.Type != "always_off" && c.Tracing.Sampling.Type != "trace_id_ratio" {
		return fmt.Errorf("tracing.sampling.type must be one of: always_on, always_off, trace_id_ratio")
	}

	if c.Tracing.Sampling.Type == "trace_id_ratio" && (c.Tracing.Sampling.Rate < 0 || c.Tracing.Sampling.Rate > 1) {
		return fmt.Errorf("tracing.sampling.rate must be between 0.0 and 1.0")
	}

	// Validate metrics exporter
	if c.Metrics.Exporter != "prometheus" && c.Metrics.Exporter != "otlp" && c.Metrics.Exporter != "null" {
		return fmt.Errorf("metrics.exporter must be one of: prometheus, otlp, null")
	}

	// Validate logging level
	if c.Logging.Level != "debug" && c.Logging.Level != "info" && c.Logging.Level != "warn" && c.Logging.Level != "error" {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}

	// Validate logging format
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("logging.format must be one of: json, text")
	}

	return nil
}

// IsTracingEnabled returns whether tracing is enabled
func (c *Config) IsTracingEnabled() bool {
	return c.Enabled && c.Tracing.Enabled
}

// IsMetricsEnabled returns whether metrics is enabled
func (c *Config) IsMetricsEnabled() bool {
	return c.Enabled && c.Metrics.Enabled
}

// IsLoggingEnabled returns whether logging is enabled
func (c *Config) IsLoggingEnabled() bool {
	return c.Enabled && c.Logging.Enabled
}
