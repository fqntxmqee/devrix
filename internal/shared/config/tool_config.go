package config

// ToolConfig holds tool execution and sandbox settings.
type ToolConfig struct {
	Sandbox       ToolSandboxConfig `yaml:"sandbox"`
	ConcurrentMax int               `yaml:"concurrent_max"`
}

// ToolSandboxConfig controls bash command sandboxing.
type ToolSandboxConfig struct {
	Enabled           *bool    `yaml:"enabled"`
	AllowlistExtra    []string `yaml:"allowlist_extra"`
	DenyPatternsExtra []string `yaml:"deny_patterns_extra"`
}

func toolSandboxEnabledDefault() bool {
	return true
}

// DefaultToolConfig returns production defaults.
func DefaultToolConfig() *ToolConfig {
	enabled := toolSandboxEnabledDefault()
	return &ToolConfig{
		Sandbox: ToolSandboxConfig{
			Enabled: &enabled,
		},
		ConcurrentMax: 10,
	}
}

// SandboxEnabled reports whether bash sandboxing is active.
func (c *ToolConfig) SandboxEnabled() bool {
	if c == nil || c.Sandbox.Enabled == nil {
		return toolSandboxEnabledDefault()
	}
	return *c.Sandbox.Enabled
}

// BuildToolConfig merges YAML values onto defaults.
func BuildToolConfig(fileCfg *ConfigFile) *ToolConfig {
	cfg := DefaultToolConfig()
	if fileCfg == nil {
		return cfg
	}

	if fileCfg.Tool.Sandbox.Enabled != nil {
		cfg.Sandbox.Enabled = fileCfg.Tool.Sandbox.Enabled
	}
	if len(fileCfg.Tool.Sandbox.AllowlistExtra) > 0 {
		cfg.Sandbox.AllowlistExtra = append([]string{}, fileCfg.Tool.Sandbox.AllowlistExtra...)
	}
	if len(fileCfg.Tool.Sandbox.DenyPatternsExtra) > 0 {
		cfg.Sandbox.DenyPatternsExtra = append([]string{}, fileCfg.Tool.Sandbox.DenyPatternsExtra...)
	}
	if fileCfg.Tool.ConcurrentMax > 0 {
		cfg.ConcurrentMax = fileCfg.Tool.ConcurrentMax
	}
	return cfg
}
