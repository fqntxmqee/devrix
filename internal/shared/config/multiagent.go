package config

import "time"

// AgentToolFileConfig is the YAML shape for a single agent tool entry.
type AgentToolFileConfig struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display_name"`
	Description  string   `yaml:"description"`
	Capabilities []string `yaml:"capabilities"`
	Command      string   `yaml:"command"`
	Args         []string `yaml:"args"`
	WorkDir      string   `yaml:"work_dir"`
	Timeout      string   `yaml:"timeout"`
	IdleTimeout  string   `yaml:"idle_timeout"`
}

// AgentToolsFileConfig is the YAML shape for the agent_tools section.
type AgentToolsFileConfig struct {
	Enabled     bool                 `yaml:"enabled"`
	Tools       []AgentToolFileConfig `yaml:"tools"`
}

// AgentToolConfig is the resolved runtime configuration for a single tool.
type AgentToolConfig struct {
	Name         string
	DisplayName  string
	Description  string
	Capabilities []string
	Command      string
	Args         []string
	WorkDir      string
	Timeout      time.Duration
	IdleTimeout  time.Duration
}

// AgentToolsConfig is the resolved runtime configuration container.
type AgentToolsConfig struct {
	Enabled bool
	Tools   []AgentToolConfig
}

// DefaultAgentToolsConfig returns V1 defaults.
func DefaultAgentToolsConfig() *AgentToolsConfig {
	return &AgentToolsConfig{Enabled: false}
}

// BuildAgentToolsConfig merges file config over defaults.
func BuildAgentToolsConfig(file *AgentToolsFileConfig) *AgentToolsConfig {
	cfg := DefaultAgentToolsConfig()
	if file == nil {
		return cfg
	}
	cfg.Enabled = file.Enabled
	for _, ft := range file.Tools {
		t := AgentToolConfig{
			Name:         ft.Name,
			DisplayName:  ft.DisplayName,
			Description:  ft.Description,
			Capabilities: ft.Capabilities,
			Command:      ft.Command,
			Args:         ft.Args,
			WorkDir:      ft.WorkDir,
			Timeout:      5 * time.Minute,
			IdleTimeout:  5 * time.Minute,
		}
		if d, err := time.ParseDuration(ft.Timeout); err == nil && d > 0 {
			t.Timeout = d
		}
		if d, err := time.ParseDuration(ft.IdleTimeout); err == nil && d > 0 {
			t.IdleTimeout = d
		}
		cfg.Tools = append(cfg.Tools, t)
	}
	return cfg
}

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
