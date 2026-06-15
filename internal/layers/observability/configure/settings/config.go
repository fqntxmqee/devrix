package settings

import "time"

// TracingConfig holds tracing configuration.
type TracingConfig struct {
	Enabled        bool           `yaml:"enabled"`
	ServiceName    string         `yaml:"service_name"`
	ServiceVersion string         `yaml:"service_version"`
	Exporter       string         `yaml:"exporter"`
	Sampling       SamplingConfig `yaml:"sampling"`
	OTLP           OTLPConfig     `yaml:"otlp"`
}

// SamplingConfig holds sampling configuration.
type SamplingConfig struct {
	Type string  `yaml:"type"`
	Rate float64 `yaml:"rate"`
}

// OTLPConfig holds OTLP exporter configuration.
type OTLPConfig struct {
	Endpoint string        `yaml:"endpoint"`
	Insecure bool          `yaml:"insecure"`
	Timeout  time.Duration `yaml:"timeout"`
}

// MetricsConfig holds metrics configuration.
type MetricsConfig struct {
	Enabled  bool         `yaml:"enabled"`
	Exporter string       `yaml:"exporter"`
	Endpoint string       `yaml:"endpoint"`
	Labels   LabelsConfig `yaml:"labels"`
}

// LabelsConfig holds label allowlist/blocklist configuration.
type LabelsConfig struct {
	Allowlist []string `yaml:"allowlist"`
	Blocklist []string `yaml:"blocklist"`
}
