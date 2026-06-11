package config

import "time"

// OrchestrationConfig is the runtime configuration for the orchestration validator.
type OrchestrationConfig struct {
	Enabled        bool   `yaml:"enabled"`
	JudgeProvider  string `yaml:"judge_provider"`
	JudgeModel     string `yaml:"judge_model"`
	JudgeMaxTokens int    `yaml:"judge_max_tokens"`

	FallbackJudgeProvider string `yaml:"fallback_judge_provider"`
	FallbackJudgeModel    string `yaml:"fallback_judge_model"`

	PreFilterEnabled     bool     `yaml:"pre_filter_enabled"`
	TrustedToolAllowlist []string `yaml:"trusted_tool_allowlist"`
	AgentSelectionVerify bool     `yaml:"agent_selection_verify"`

	MinIntervalBetweenJudges time.Duration `yaml:"min_interval_between_judges"`
	MaxJudgeCallsPerMinute   int           `yaml:"max_judge_calls_per_minute"`

	AutoIntervene         bool    `yaml:"auto_intervene"`
	InterventionThreshold float64 `yaml:"intervention_threshold"`
}

// DefaultOrchestrationConfig returns sensible defaults.
func DefaultOrchestrationConfig() OrchestrationConfig {
	return OrchestrationConfig{
		Enabled:        true,
		JudgeProvider:  "minimax",
		JudgeModel:     "MiniMax-M2.7-highspeed",
		JudgeMaxTokens: 1024,

		FallbackJudgeProvider: "deepseek",
		FallbackJudgeModel:    "deepseek-v4-flash",

		PreFilterEnabled:     true,
		AgentSelectionVerify: true,

		MinIntervalBetweenJudges: 2 * time.Second,
		MaxJudgeCallsPerMinute:   10,

		AutoIntervene:         false,
		InterventionThreshold: 0.3,
	}
}

// BuildOrchestrationConfig merges OrchestrationFileConfig into OrchestrationConfig.
func BuildOrchestrationConfig(file *OrchestrationFileConfig) *OrchestrationConfig {
	cfg := DefaultOrchestrationConfig()

	cfg.Enabled = file.Enabled
	if file.JudgeProvider != "" {
		cfg.JudgeProvider = file.JudgeProvider
	}
	if file.JudgeModel != "" {
		cfg.JudgeModel = file.JudgeModel
	}
	if file.FallbackJudgeProvider != "" {
		cfg.FallbackJudgeProvider = file.FallbackJudgeProvider
	}
	if file.FallbackJudgeModel != "" {
		cfg.FallbackJudgeModel = file.FallbackJudgeModel
	}
	if file.PreFilterEnabled {
		cfg.PreFilterEnabled = true
	}
	if file.MinIntervalBetweenJudges != "" {
		if d, err := time.ParseDuration(file.MinIntervalBetweenJudges); err == nil {
			cfg.MinIntervalBetweenJudges = d
		}
	}
	if file.MaxJudgeCallsPerMinute > 0 {
		cfg.MaxJudgeCallsPerMinute = file.MaxJudgeCallsPerMinute
	}
	if len(file.TrustedToolAllowlist) > 0 {
		cfg.TrustedToolAllowlist = file.TrustedToolAllowlist
	}
	if file.InterventionThreshold > 0 {
		cfg.InterventionThreshold = file.InterventionThreshold
	}
	cfg.AutoIntervene = file.AutoIntervene

	return &cfg
}

// LoadOrchestrationConfig loads orchestration config from a YAML file path.
func LoadOrchestrationConfig(path string) (*OrchestrationConfig, error) {
	if path == "" {
		cfg := DefaultOrchestrationConfig()
		return &cfg, nil
	}
	fileCfg, err := LoadConfigFile(path)
	if err != nil {
		return nil, err
	}
	return BuildOrchestrationConfig(&fileCfg.Orchestration), nil
}
