package config

// ContextEngineConfig holds Layer 2 configuration.
type ContextEngineConfig struct {
	MaxContextTokens   int                `yaml:"max_context_tokens"`
	ReservedOutput     int                `yaml:"reserved_output_tokens"`
	ToolResultBudget   int                `yaml:"tool_result_budget"`
	CompressionEnabled bool               `yaml:"compression_enabled"`
	Compression        CompressionConfig  `yaml:"compression"`
	TokenCounter       TokenCounterConfig `yaml:"token_counter"`
	PEV                PEVConfig          `yaml:"pev"`
	Snapshot           SnapshotConfig     `yaml:"snapshot"`
	SystemPrompt       SystemPromptConfig `yaml:"system_prompt"`
	Plan               PlanConfig         `yaml:"plan"`
	LongTerm           LongTermConfig     `yaml:"longterm"`
	Harness            HarnessConfig      `yaml:"harness"`
	Preflight          PreflightConfig    `yaml:"preflight"`
	Workspace          WorkspacePromptConfig `yaml:"workspace"`
}

// PEVConfig holds PEV loop settings.
type PEVConfig struct {
	MaxIterations  int                   `yaml:"max_iterations"`
	VerifyMode     string                `yaml:"verify_mode"`
	VerifyPolicy   string                `yaml:"verify_policy"`
	VerifyCommands []VerifyCommandConfig `yaml:"verify_commands"`
}

// SnapshotConfig holds snapshot persistence settings.
type SnapshotConfig struct {
	Enabled              bool   `yaml:"enabled"`
	BackupDir            string `yaml:"backup_dir"`
	Compression          bool   `yaml:"compression"`
	CompressionThreshold int    `yaml:"compression_threshold"`
}

// SystemPromptConfig holds system prompt source paths.
type SystemPromptConfig struct {
	Sources  []string `yaml:"sources"`
	Fallback string   `yaml:"fallback"`
}

// DefaultContextEngineConfig returns V1 defaults.
func DefaultContextEngineConfig() *ContextEngineConfig {
	return &ContextEngineConfig{
		MaxContextTokens:   128000,
		ReservedOutput:     8192,
		ToolResultBudget:   800,
		CompressionEnabled: true,
		Compression: CompressionConfig{
			Autocompact: DefaultAutocompactConfig(),
		},
		TokenCounter: TokenCounterConfig{
			Source: TokenCounterSourceGateway,
		},
		PEV: PEVConfig{
			MaxIterations: 3,
			VerifyMode:    VerifyModeBasic,
			VerifyPolicy:  VerifyPolicyAllPass,
		},
		Snapshot: SnapshotConfig{
			Enabled:   true,
			BackupDir: "~/.devrix/context",
		},
		SystemPrompt: SystemPromptConfig{
			Sources:  []string{"AGENTS.md", ".devrix/AGENTS.md"},
			Fallback: "You are Devrix, a multi-agent development assistant.",
		},
		Plan:      DefaultPlanConfig(),
		LongTerm:  DefaultLongTermConfig(),
		Harness:   DefaultHarnessConfig(),
		Preflight: DefaultPreflightConfig(),
		Workspace: DefaultWorkspacePromptConfig(),
	}
}

// ToTokenBudget converts config to types.TokenBudget.
func (c *ContextEngineConfig) ToTokenBudget() (max, reserved, toolResult, target int) {
	max = c.MaxContextTokens
	reserved = c.ReservedOutput
	toolResult = c.ToolResultBudget
	if max <= 0 {
		max = 128000
	}
	if reserved <= 0 {
		reserved = 8192
	}
	if toolResult <= 0 {
		toolResult = 800
	}
	target = int(float64(max-reserved) * 0.9)
	return max, reserved, toolResult, target
}
