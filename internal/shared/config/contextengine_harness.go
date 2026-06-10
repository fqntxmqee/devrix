package config

import "fmt"

const (
	PreflightModeWarnOnly = "warn-only"
	PreflightModeBlock    = "block"

	PreflightToolFilterNone       = "none"
	PreflightToolFilterAutoRepair = "auto-repair"
)

// HarnessConfig holds harness bootstrap settings (V5).
type HarnessConfig struct {
	Enabled      bool               `yaml:"enabled"`
	Trusted      bool               `yaml:"trusted"`
	Prefetch     HarnessPrefetchConfig `yaml:"prefetch"`
	ToolPool     ToolPoolConfig     `yaml:"tool_pool"`
	Routing      RoutingConfig      `yaml:"routing"`
	DeferredInit DeferredInitConfig `yaml:"deferred_init"`
	Transcript   TranscriptConfig   `yaml:"transcript"`
}

// HarnessPrefetchConfig controls workspace prefetch during bootstrap.
type HarnessPrefetchConfig struct {
	Enabled      bool `yaml:"enabled"`
	MaxWalkDepth int  `yaml:"max_walk_depth"`
}

// ToolPoolConfig controls visible tool filtering.
type ToolPoolConfig struct {
	SimpleMode   bool     `yaml:"simple_mode"`
	IncludeMCP   bool     `yaml:"include_mcp"`
	DenyNames    []string `yaml:"deny_names"`
	DenyPrefixes []string `yaml:"deny_prefixes"`
}

// RoutingConfig controls advisory prompt routing hints.
type RoutingConfig struct {
	Enabled    bool `yaml:"enabled"`
	MaxMatches int  `yaml:"max_matches"`
}

// DeferredInitConfig controls trust-gated deferred initialization stage.
type DeferredInitConfig struct {
	Enabled bool `yaml:"enabled"`
}

// TranscriptConfig controls in-memory transcript behavior.
type TranscriptConfig struct {
	Enabled           bool `yaml:"enabled"`
	CompactAfterTurns int  `yaml:"compact_after_turns"`
	SessionLogEnabled bool `yaml:"session_log_enabled"`
}

// PreflightConfig holds pre-LLM context quality evaluation settings.
type PreflightConfig struct {
	Enabled     bool                  `yaml:"enabled"`
	Mode        string                `yaml:"mode"`
	TokenBudget int                   `yaml:"token_budget"`
	WarnRatio   float64               `yaml:"warn_ratio"`
	ToolFilter  PreflightToolFilterConfig `yaml:"tool_filter"`
}

// PreflightToolFilterConfig controls tool relevance filtering during preflight.
type PreflightToolFilterConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode"`
}

// WorkspacePromptConfig holds system prompt assembly workspace settings.
type WorkspacePromptConfig struct {
	MaxContextTokens       int      `yaml:"max_context_tokens"`
	AgentName              string   `yaml:"agent_name"`
	AdditionalContextFiles []string `yaml:"additional_context_files"`
	EmbedCoreTemplate      bool     `yaml:"embed_core_template"`
}

// DefaultHarnessConfig returns V5 harness defaults (disabled for safe rollout).
func DefaultHarnessConfig() HarnessConfig {
	return HarnessConfig{
		Enabled: false,
		Trusted: true,
		Prefetch: HarnessPrefetchConfig{
			Enabled:      true,
			MaxWalkDepth: 4,
		},
		ToolPool: ToolPoolConfig{
			SimpleMode: false,
			IncludeMCP: true,
		},
		Routing: RoutingConfig{
			Enabled:    false,
			MaxMatches: 5,
		},
		DeferredInit: DeferredInitConfig{
			Enabled: true,
		},
		Transcript: TranscriptConfig{
			Enabled:           true,
			CompactAfterTurns: 20,
			SessionLogEnabled: true,
		},
	}
}

// DefaultPreflightConfig returns V5 preflight defaults.
func DefaultPreflightConfig() PreflightConfig {
	return PreflightConfig{
		Enabled:     false,
		Mode:        PreflightModeWarnOnly,
		TokenBudget: 8000,
		WarnRatio:   0.85,
		ToolFilter: PreflightToolFilterConfig{
			Enabled: true,
			Mode:    PreflightToolFilterAutoRepair,
		},
	}
}

// DefaultWorkspacePromptConfig returns workspace prompt assembly defaults.
func DefaultWorkspacePromptConfig() WorkspacePromptConfig {
	return WorkspacePromptConfig{
		MaxContextTokens:  8000,
		AgentName:         "Devrix",
		EmbedCoreTemplate: true,
	}
}

// ValidateHarnessConfig validates harness-related configuration.
func ValidateHarnessConfig(h HarnessConfig, preflight PreflightConfig) error {
	if preflight.Enabled {
		switch preflight.Mode {
		case PreflightModeWarnOnly:
		case PreflightModeBlock:
			return fmt.Errorf("context_engine.preflight.mode: %q is not supported in V5a (use warn-only)", PreflightModeBlock)
		case "":
		default:
			return fmt.Errorf("context_engine.preflight.mode: invalid value %q", preflight.Mode)
		}
		if preflight.TokenBudget <= 0 {
			return fmt.Errorf("context_engine.preflight.token_budget: must be > 0 when preflight enabled")
		}
		if preflight.WarnRatio <= 0 || preflight.WarnRatio > 1 {
			return fmt.Errorf("context_engine.preflight.warn_ratio: must be in (0, 1]")
		}
	}
	if preflight.ToolFilter.Enabled {
		switch preflight.ToolFilter.Mode {
		case PreflightToolFilterNone, PreflightToolFilterAutoRepair, "":
		default:
			return fmt.Errorf("context_engine.preflight.tool_filter.mode: invalid value %q", preflight.ToolFilter.Mode)
		}
	}
	if h.Prefetch.MaxWalkDepth < 0 {
		return fmt.Errorf("context_engine.harness.prefetch.max_walk_depth: must be >= 0")
	}
	if h.Routing.MaxMatches < 0 {
		return fmt.Errorf("context_engine.harness.routing.max_matches: must be >= 0")
	}
	return nil
}
