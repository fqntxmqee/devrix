package config

// QueryLoopConfig holds QueryLoop runtime settings (Claude Code queryLoop aligned).
type QueryLoopConfig struct {
	Enabled         bool `yaml:"enabled"`
	MaxTurns        int  `yaml:"max_turns"`
	CompressPerTurn bool `yaml:"compress_per_turn"`
	StreamingTools  bool `yaml:"streaming_tools"`
}

// UserContextConfig controls AGENTS.md injection strategy.
type UserContextConfig struct {
	Mode string `yaml:"mode"` // prepend | system | both
}

// AttachmentsConfig controls meta-user attachment injection.
type AttachmentsConfig struct {
	Enabled           bool `yaml:"enabled"`
	PlanModeFullEvery int  `yaml:"plan_mode_full_every"`
}

// ContextPermissionConfig holds permission mode defaults for QueryLoop.
type ContextPermissionConfig struct {
	DefaultMode string           `yaml:"default_mode"`
	Plan        PlanModeConfig   `yaml:"plan"`
}

// PlanModeConfig holds plan-mode-specific settings.
type PlanModeConfig struct {
	ExploreAgentCount int    `yaml:"explore_agent_count"`
	PlanAgentCount    int    `yaml:"plan_agent_count"`
	PlanFileDir       string `yaml:"plan_file_dir"`
}

// TasksConfig selects todo v1 vs task v2 tools.
type TasksConfig struct {
	Mode     string `yaml:"mode"` // v1 | v2
	StoreDir string `yaml:"store_dir"`
}

// SubQueryConfig holds sub-agent runtime settings.
type SubQueryConfig struct {
	ForkSubagentEnabled    bool `yaml:"fork_subagent_enabled"`
	SidechainTranscript    bool `yaml:"sidechain_transcript"`
	DefaultSubagentMaxTurns int `yaml:"default_subagent_max_turns"`
}

// DefaultQueryLoopConfig returns v1.0 defaults.
func DefaultQueryLoopConfig() QueryLoopConfig {
	return QueryLoopConfig{
		Enabled:         false,
		MaxTurns:        0,
		CompressPerTurn: true,
	}
}

func DefaultUserContextConfig() UserContextConfig {
	return UserContextConfig{Mode: "prepend"}
}

func DefaultAttachmentsConfig() AttachmentsConfig {
	return AttachmentsConfig{
		Enabled:           true,
		PlanModeFullEvery: 5,
	}
}

func DefaultContextPermissionConfig() ContextPermissionConfig {
	return ContextPermissionConfig{
		DefaultMode: "default",
		Plan: PlanModeConfig{
			ExploreAgentCount: 3,
			PlanAgentCount:    1,
			PlanFileDir:       "~/.devrix/plans",
		},
	}
}

func DefaultTasksConfig() TasksConfig {
	return TasksConfig{
		Mode:     "v2",
		StoreDir: "~/.devrix/tasks",
	}
}

func DefaultSubQueryConfig() SubQueryConfig {
	return SubQueryConfig{
		SidechainTranscript:     true,
		DefaultSubagentMaxTurns: 50,
	}
}
