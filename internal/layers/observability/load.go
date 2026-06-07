package observability

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type configFile struct {
	Observability Config `yaml:"observability"`
}

// LoadConfigFromFile loads observability settings from a devrix YAML file.
func LoadConfigFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read observability config: %w", err)
	}

	var file configFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse observability config: %w", err)
	}

	cfg := DefaultConfig()
	mergeConfig(cfg, &file.Observability)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func mergeConfig(base *Config, loaded *Config) {
	if loaded == nil {
		return
	}
	// Only apply loaded values when the config file actually contains an
	// observability section. Otherwise, zero-valued fields would overwrite
	// DefaultConfig (e.g. Enabled=true → false when the key is absent).
	hasContent := loaded.Enabled ||
		loaded.Tracing.Enabled || loaded.Tracing.Exporter != "" || loaded.Tracing.ServiceName != "" ||
		loaded.Metrics.Enabled || loaded.Metrics.Exporter != "" ||
		loaded.Logging.Enabled || loaded.Logging.Level != "" ||
		loaded.Health.Enabled || loaded.Health.Endpoint != ""
	if !hasContent {
		return
	}
	base.Enabled = loaded.Enabled
	if loaded.Tracing.Enabled || loaded.Tracing.Exporter != "" || loaded.Tracing.ServiceName != "" {
		base.Tracing = loaded.Tracing
	}
	if loaded.Metrics.Enabled || loaded.Metrics.Exporter != "" {
		base.Metrics = loaded.Metrics
	}
	if loaded.Logging.Enabled || loaded.Logging.Level != "" {
		base.Logging = loaded.Logging
	}
	if loaded.Health.Enabled || loaded.Health.Endpoint != "" {
		base.Health = loaded.Health
	}
}
