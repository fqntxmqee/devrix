package config

// ContextEngineConfig holds Layer 2 configuration.
type ContextEngineConfig struct {
	MaxContextTokens   int                `yaml:"max_context_tokens"`
	ReservedOutput     int                `yaml:"reserved_output_tokens"`
	ToolResultBudget   int                `yaml:"tool_result_budget"`
	CompressionEnabled bool               `yaml:"compression_enabled"`
	CompressionRatio   float64            `yaml:"compression_ratio"`    // autocompact 触发阈值比率, 默认 0.6
	SnipRatio          float64            `yaml:"snip_ratio"`           // snip 触发阈值比率, 默认 0.8
	Compression        CompressionConfig  `yaml:"compression"`
	TokenCounter       TokenCounterConfig `yaml:"token_counter"`
	Snapshot           SnapshotConfig     `yaml:"snapshot"`
	SystemPrompt       SystemPromptConfig `yaml:"system_prompt"`
	Prompt            *PromptConfig       `yaml:"prompt"`
	LongTerm           LongTermConfig     `yaml:"longterm"`
	Harness            HarnessConfig      `yaml:"harness"`
	Preflight          PreflightConfig    `yaml:"preflight"`
	Workspace          WorkspacePromptConfig `yaml:"workspace"`
	QueryLoop          QueryLoopConfig    `yaml:"query_loop"`
	UserContext        UserContextConfig  `yaml:"user_context"`
	Attachments        AttachmentsConfig  `yaml:"attachments"`
	Permission         ContextPermissionConfig `yaml:"permission"`
	Tasks              TasksConfig        `yaml:"tasks"`
	SubQuery           SubQueryConfig     `yaml:"subquery"`
	ExecutionFlow      ExecutionFlowConfig `yaml:"execution_flow"`
	Worktree           WorktreeConfig     `yaml:"worktree"`
	TodoWrite          TodoWriteConfig    `yaml:"todo_write"`
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
		CompressionRatio:   0.6,
		SnipRatio:          0.8,
		Compression: DefaultCompressionConfig(),
		TokenCounter: TokenCounterConfig{
			Source: TokenCounterSourceGateway,
		},
		Snapshot: SnapshotConfig{
			Enabled:   true,
			BackupDir: "~/.devrix/context",
		},
		SystemPrompt: SystemPromptConfig{
			Sources:  []string{"AGENTS.md", ".devrix/AGENTS.md"},
			Fallback: "You are Devrix, a multi-agent development assistant.",
		},
		Prompt:     DefaultPromptConfig(),
		LongTerm:   DefaultLongTermConfig(),
		Harness:    DefaultHarnessConfig(),
		Preflight:  DefaultPreflightConfig(),
		Workspace:   DefaultWorkspacePromptConfig(),
		QueryLoop:   DefaultQueryLoopConfig(),
		UserContext: DefaultUserContextConfig(),
		Attachments: DefaultAttachmentsConfig(),
		Permission:  DefaultContextPermissionConfig(),
		Tasks:       DefaultTasksConfig(),
		SubQuery:    DefaultSubQueryConfig(),
		ExecutionFlow: DefaultExecutionFlowConfig(),
		Worktree:      DefaultWorktreeConfig(),
		TodoWrite:      DefaultTodoWriteConfig(),
	}
}

// ToTokenBudget converts config to types.TokenBudget.
func (c *ContextEngineConfig) ToTokenBudget() (max, reserved, toolResult, target, snipTarget int) {
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
	ratio := c.CompressionRatio
	if ratio <= 0 || ratio > 1.0 {
		ratio = 0.6
	}
	target = int(float64(max-reserved) * ratio)

	snipRatio := c.SnipRatio
	if snipRatio <= 0 || snipRatio > 1.0 {
		snipRatio = 0.8
	}
	snipTarget = int(float64(max-reserved) * snipRatio)
	return max, reserved, toolResult, target, snipTarget
}

// TodoWriteConfig holds todo_write tool settings.
type TodoWriteConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DefaultTodoWriteConfig returns default todo_write settings.
func DefaultTodoWriteConfig() TodoWriteConfig {
	return TodoWriteConfig{Enabled: true}
}
