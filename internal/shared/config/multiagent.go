package config

import "time"

// MultiAgentFileConfig is the YAML shape for multi_agent.
type MultiAgentFileConfig struct {
	Enabled           bool          `yaml:"enabled"`
	MaxChildren       int           `yaml:"max_children"`
	MaxTotalAgents    int           `yaml:"max_total_agents"`
	DefaultTimeout    time.Duration `yaml:"default_timeout"`
	DefaultMaxIter    int           `yaml:"default_max_iter"`
	PermissionTimeout time.Duration `yaml:"permission_timeout"`
	DefaultMode       string        `yaml:"default_mode"`
}

// MultiAgentConfig is the resolved Layer 4 configuration.
type MultiAgentConfig struct {
	Enabled           bool
	MaxChildren       int
	MaxTotalAgents    int
	DefaultTimeout    time.Duration
	DefaultMaxIter    int
	PermissionTimeout time.Duration
	DefaultMode       string
}

// DefaultMultiAgentConfig returns V1 defaults.
func DefaultMultiAgentConfig() *MultiAgentConfig {
	return &MultiAgentConfig{
		MaxChildren:       3,
		MaxTotalAgents:    5,
		DefaultTimeout:    5 * time.Minute,
		DefaultMaxIter:    50,
		PermissionTimeout: 60 * time.Second,
		DefaultMode:       "default",
	}
}

// BuildMultiAgentConfig merges file config over defaults.
func BuildMultiAgentConfig(file *MultiAgentFileConfig) *MultiAgentConfig {
	cfg := DefaultMultiAgentConfig()
	if file == nil {
		return cfg
	}
	cfg.Enabled = file.Enabled
	if file.MaxChildren > 0 && file.MaxChildren <= 10 {
		cfg.MaxChildren = file.MaxChildren
	}
	if file.MaxTotalAgents > 0 && file.MaxTotalAgents <= 20 {
		cfg.MaxTotalAgents = file.MaxTotalAgents
	}
	if file.DefaultTimeout > 0 {
		cfg.DefaultTimeout = file.DefaultTimeout
	}
	if file.DefaultMaxIter > 0 {
		cfg.DefaultMaxIter = file.DefaultMaxIter
	}
	if file.PermissionTimeout > 0 {
		cfg.PermissionTimeout = file.PermissionTimeout
	}
	if file.DefaultMode != "" {
		cfg.DefaultMode = file.DefaultMode
	}
	return cfg
}
