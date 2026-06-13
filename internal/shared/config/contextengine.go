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
	MainTranscript     MainTranscriptConfig `yaml:"main_transcript"`
}

// MainTranscriptConfig controls append-only JSONL persistence for main sessions.
type MainTranscriptConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseDir string `yaml:"base_dir"`
}

// DefaultMainTranscriptConfig returns main-thread transcript defaults.
func DefaultMainTranscriptConfig() MainTranscriptConfig {
	return MainTranscriptConfig{
		Enabled: true,
		BaseDir: "~/.devrix/sessions",
	}
}

// SnapshotConfig holds snapshot persistence settings.
type SnapshotConfig struct {
	Enabled              bool   `yaml:"enabled"`
	BackupDir            string `yaml:"backup_dir"`
	Compression          bool   `yaml:"compression"`
	CompressionThreshold int    `yaml:"compression_threshold"`
}

// SystemPromptConfig holds system prompt source paths and AGENTS.md discovery.
type SystemPromptConfig struct {
	Sources       []string `yaml:"sources"`
	Fallback      string   `yaml:"fallback"`
	WalkUp        *bool    `yaml:"walk_up"`
	UserGlobal    string   `yaml:"user_global"`
	RulesGlob     string   `yaml:"rules_glob"`
	MaxChars      int      `yaml:"max_chars"`
	EnableInclude *bool    `yaml:"enable_include"`
}

// DefaultSystemPromptConfig returns AGENTS.md discovery defaults (ClawCode claudemd aligned).
func DefaultSystemPromptConfig() SystemPromptConfig {
	return SystemPromptConfig{
		Sources:       []string{".devrix/AGENTS.md", "AGENTS.md"},
		Fallback:      "You are Devrix, a multi-agent development assistant.",
		WalkUp:        boolPtr(true),
		UserGlobal:    "~/.devrix/AGENTS.md",
		RulesGlob:     ".devrix/rules/*.md",
		MaxChars:      40000,
		EnableInclude: boolPtr(true),
	}
}

func boolPtr(v bool) *bool { return &v }

// Normalized returns a copy with defaults applied for unset fields.
func (c SystemPromptConfig) Normalized() SystemPromptConfig {
	def := DefaultSystemPromptConfig()
	out := c
	if len(out.Sources) == 0 {
		out.Sources = def.Sources
	}
	if out.Fallback == "" {
		out.Fallback = def.Fallback
	}
	if out.WalkUp == nil {
		out.WalkUp = def.WalkUp
	}
	if out.UserGlobal == "" {
		out.UserGlobal = def.UserGlobal
	}
	if out.RulesGlob == "" {
		out.RulesGlob = def.RulesGlob
	}
	if out.MaxChars <= 0 {
		out.MaxChars = def.MaxChars
	}
	if out.EnableInclude == nil {
		out.EnableInclude = def.EnableInclude
	}
	return out
}

func (c SystemPromptConfig) WalkUpEnabled() bool {
	if c.WalkUp == nil {
		return true
	}
	return *c.WalkUp
}

func (c SystemPromptConfig) IncludeEnabled() bool {
	if c.EnableInclude == nil {
		return true
	}
	return *c.EnableInclude
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
		SystemPrompt: DefaultSystemPromptConfig(),
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
		MainTranscript: DefaultMainTranscriptConfig(),
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
