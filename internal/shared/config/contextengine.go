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
	// Diagnostics DM-20260617-002 W13 (AC14) — 诊断 / 通知 / LSP / transcript 集中配置。
	Diagnostics        DiagnosticsConfig  `yaml:"diagnostics"`
	// Tools DM-20260617-007 W12 (AC12) — tool surface 可见性 / 风险阈值配置。
	// 空 map / 未配置 = 所有 surface 全开, 风险阈值由每个 tool 自己声明。
	Tools              ToolsConfig        `yaml:"tools"`
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
		Diagnostics:    DefaultDiagnosticsConfig(),
		Tools:          DefaultToolsConfig(),
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

// DiagnosticsConfig DM-20260617-002 W13 (AC14) — 集中配置所有诊断 / 通知 /
// LSP / transcript 子系统参数; 各 bootstrap / CLI / tool 入口从这里读取。
type DiagnosticsConfig struct {
	// TrackerLRUCapacity 文件诊断追踪器 LRU 上限。0 走 default (500)。
	TrackerLRUCapacity int `yaml:"tracker_lru_capacity"`
	// TrackerTickIntervalMs 周期 tick 间隔 (毫秒)。<=0 走 default (1000ms = 1s)。
	TrackerTickIntervalMs int `yaml:"tracker_tick_interval_ms"`
	// LSPEnabled 是否启用 LSP tool。默认 false,需配置 servers 才生效。
	LSPEnabled bool `yaml:"lsp_enabled"`
	// LSPServers 启用的 LSP server 列表。
	LSPServers []LSPServerConfig `yaml:"lsp_servers"`
	// LSPMaxServers DM-20260618-007 W0 (D2-S4-A01) — LSP server 进程池上限。
	// 默认 4; invariant `lsp_servers <= 4` 在 runtime check 中强约束 (LTL-Lite)。
	// 0 走 default。
	LSPMaxServers int `yaml:"lsp_max_servers"`
	// LSPMethodTimeoutMs DM-20260618-007 W0 (SUG-2) — 单个 LSP method 调用超时 (ms)。
	// 默认 2000ms; p99 告警阈值 lsp.latency_alert_ms (默认 1500ms)。
	LSPMethodTimeoutMs int `yaml:"lsp_method_timeout_ms"`
	// LSPLatencyAlertMs DM-20260618-007 W0 (SUG-2) — D5 metrics p99 告警阈值 (ms)。
	// 0 走 default (1500ms)。
	LSPLatencyAlertMs int `yaml:"lsp_latency_alert_ms"`
	// DebugCategories 默认注入到 DebugFilter 的 category 列表 (与 --debug 等价)。
	DebugCategories []string `yaml:"debug_categories"`
	// TranscriptDir transcript .jsonl 落盘目录。空则按 $DEVRIX_TRANSCRIPT_DIR
	// → ~/.devrix/transcripts fallback。
	TranscriptDir string `yaml:"transcript_dir"`
}

// LSPServerConfig 描述一个 LSP server 启动命令 (D2-S7-A02 / G1)。
type LSPServerConfig struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

// DefaultDiagnosticsConfig returns sensible defaults; 0 值字段走 internal default。
func DefaultDiagnosticsConfig() DiagnosticsConfig {
	return DiagnosticsConfig{
		TrackerLRUCapacity:   500,
		TrackerTickIntervalMs: 1000,
		LSPEnabled:           false,
		LSPMaxServers:        4,
		LSPMethodTimeoutMs:   2000,
		LSPLatencyAlertMs:    1500,
	}
}

// Normalized 填充 0 值字段为 default; 保留显式设置。
func (c DiagnosticsConfig) Normalized() DiagnosticsConfig {
	def := DefaultDiagnosticsConfig()
	out := c
	if out.TrackerLRUCapacity <= 0 {
		out.TrackerLRUCapacity = def.TrackerLRUCapacity
	}
	if out.TrackerTickIntervalMs <= 0 {
		out.TrackerTickIntervalMs = def.TrackerTickIntervalMs
	}
	if out.LSPMaxServers <= 0 {
		out.LSPMaxServers = def.LSPMaxServers
	}
	if out.LSPMethodTimeoutMs <= 0 {
		out.LSPMethodTimeoutMs = def.LSPMethodTimeoutMs
	}
	if out.LSPLatencyAlertMs <= 0 {
		out.LSPLatencyAlertMs = def.LSPLatencyAlertMs
	}
	return out
}

// ToolsConfig DM-20260617-007 W12 (AC12) — tool surface 可见性 / 风险阈值配置。
// 按 surface name 索引, key 与 surface.Name() 一致 ("builtin" / "lsp" /
// "free_fork" / "tracker" / "verify" / "delegate_*" / "background_task" / ...)。
type ToolsConfig struct {
	// Surfaces 显式 surface 级覆盖; 空 = 全开 (各 surface 暴露所有 tool)。
	Surfaces map[string]SurfaceConfig `yaml:"surfaces"`
	// RiskThreshold 全局风险阈值; tool.Risk 超过此值时被 per-risk filter 隐藏。
	// 留空 "" = 不过滤, 暴露所有 risk 等级。
	RiskThreshold string `yaml:"risk_threshold"`
	// FreeForkMaxConcurrent DM-20260618-007 W0 (D4-S11-A02) — free_fork 最大并发子代理数。
	// 默认 8 (硬约束 by invariant `concurrent_forks <= 8`)。0 走 default。
	FreeForkMaxConcurrent int `yaml:"free_fork_max_concurrent"`
	// VerifyTimeoutSec DM-20260618-007 W0 (D6-S11-A02) — verify 单项超时 (秒)。
	// 默认 300s (5 分钟); per-verify-type 可覆盖。
	VerifyTimeoutSec int `yaml:"verify_timeout_sec"`
	// TrackerSamplingThresholdHz DM-20260618-007 W0 (D5-S23-A02) — 编辑频率
	// 高于此值时降级为采样追踪 (fire-and-forget 丢失误接受)。默认 10/s。
	TrackerSamplingThresholdHz int `yaml:"tracker_sampling_threshold_hz"`
}

// SurfaceConfig 单个 surface 的可见性 / 风险阈值覆盖。
type SurfaceConfig struct {
	// Enabled false = 整 surface 隐藏 (Tools() 返回空); 缺省 / nil = true。
	Enabled *bool `yaml:"enabled"`
	// RiskThreshold 覆盖全局阈值, 仅作用于本 surface。
	RiskThreshold string `yaml:"risk_threshold"`
}

// DefaultToolsConfig returns sane defaults (all surfaces enabled, no threshold).
func DefaultToolsConfig() ToolsConfig {
	return ToolsConfig{
		Surfaces:                  map[string]SurfaceConfig{},
		FreeForkMaxConcurrent:     8,
		VerifyTimeoutSec:          300,
		TrackerSamplingThresholdHz: 10,
	}
}

// Normalized fills in nil Enabled pointers to point at true (no behavior change
// for explicit false; the default action is "enabled unless explicitly disabled").
func (c ToolsConfig) Normalized() ToolsConfig {
	out := c
	if out.Surfaces == nil {
		out.Surfaces = map[string]SurfaceConfig{}
	}
	if out.FreeForkMaxConcurrent <= 0 {
		out.FreeForkMaxConcurrent = 8
	}
	if out.VerifyTimeoutSec <= 0 {
		out.VerifyTimeoutSec = 300
	}
	if out.TrackerSamplingThresholdHz <= 0 {
		out.TrackerSamplingThresholdHz = 10
	}
	return out
}

// IsEnabled returns true if the surface is not explicitly disabled in config.
// Returns true when the surface has no entry in the Surfaces map (default-on).
func (c ToolsConfig) IsEnabled(surfaceName string) bool {
	if c.Surfaces == nil {
		return true
	}
	sc, ok := c.Surfaces[surfaceName]
	if !ok || sc.Enabled == nil {
		return true
	}
	return *sc.Enabled
}
