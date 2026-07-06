package config

// CoordinatorConfig is the YAML-loaded, runtime configuration for the
// SessionCoordinator (D7 orchestration domain). The fields mirror
// coordinator.Config but live in the shared config package to avoid an
// import cycle (coordinator imports shared contracts/types; shared
// config is allowed to import coordinator).
//
// BuildCoordinatorConfig merges a file-loaded CoordinatorFileConfig into
// the v1.0 defaults. Caller passes the result to
// coordinator.NewSessionOrchestrator via coordinator.BuildConfig.
//
// Naming note: per the architecture naming policy, no D{N} identifier is
// used for package / file / type names — D{N} is reserved for DSAFT
// cross-team alignment only. The parent ConfigFile.D7 YAML section
// remains for backward compatibility with existing config files.
type CoordinatorConfig struct {
	Enabled                     bool     `yaml:"enabled"`
	RoutingMode                 string   `yaml:"routing_mode"`
	FastPathThreshold           int      `yaml:"fast_path_threshold"`
	CommandFirst                bool     `yaml:"command_first"`
	LLMFallback                 bool     `yaml:"llm_fallback"`
	AdvisoryValidationTimeoutMs int      `yaml:"d6_validation_timeout_ms"`
	CommandWhitelist            []string `yaml:"command_whitelist"`
	// PriorContextRounds (DM-20260628-003, D7-S15) controls how many
	// recent turns' finalText to inject into the next turn's directive
	// as a <prior-output-summary> block. 0 (default) disables injection
	// — equivalent to pre-D7-S15 behavior. >0 enables TurnState-based
	// serialization (one turn at a time per session) and transcript
	// reader wired in bootstrap.InitOrchestration.
	PriorContextRounds int `yaml:"prior_context_rounds"`
	// SemanticConvergence (D7-S16, DM-20260706-006) controls whether
	// the per-WorkItem ItemPipelineRunner consults an LLM SemanticVerifier
	// when the code-based verify rubber-stamps Pass but the artifact
	// looks structurally identical to a prior round (Jaccard ≥
	// MinSimilarity). Mirrors orchtypes.SemanticConvergenceConfig — the
	// bootstrap layer merges this into ItemPipelineRunner.SemanticConfig
	// and constructs a DefaultSemanticVerifier.
	SemanticConvergence SemanticConvergenceFileConfig `yaml:"semantic_convergence"`
}

// SemanticConvergenceFileConfig mirrors orchtypes.SemanticConvergenceConfig
// for YAML deserialization. Pointer fields let BuildCoordinatorConfig
// distinguish "absent in yaml" (keep default) from "explicitly false / 0".
type SemanticConvergenceFileConfig struct {
	Enabled       *bool    `yaml:"enabled"`
	MinSimilarity *float64 `yaml:"min_similarity"`
	LookbackN     *int     `yaml:"lookback_n"`
	TimeoutMs     *int     `yaml:"timeout_ms"`
	ModelTier     *string  `yaml:"model_tier"`
}

// DefaultCoordinatorConfig returns the v1.0 defaults — matches
// coordinator.DefaultConfig().
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		Enabled:                     true,
		RoutingMode:                 "loop_first",
		FastPathThreshold:           90,
		CommandFirst:                true,
		LLMFallback:                 false,
		AdvisoryValidationTimeoutMs: 50,
		CommandWhitelist: []string{
			"/plan", "/stop", "/task", "/help",
		},
		// SemanticConvergence production default = Enabled=true
		// (DM-20260706-006). The Jaccard pre-check is cheap; the LLM
		// call only fires on stagnation-suspect rounds.
		SemanticConvergence: SemanticConvergenceFileConfig{
			Enabled:       boolPtr(true),
			MinSimilarity: coordFloat64Ptr(0.85),
			LookbackN:     coordIntPtr(5),
			TimeoutMs:     coordIntPtr(8000),
			ModelTier:     coordStringPtr(""),
		},
	}
}

// boolPtr is defined in contextengine.go (shared by all config types).
// These three are the coordinator.go-local numeric / string helpers.
func coordIntPtr(n int) *int             { return &n }
func coordFloat64Ptr(f float64) *float64 { return &f }
func coordStringPtr(s string) *string    { return &s }

// BuildCoordinatorConfig merges file over defaults. Nil fields in file keep defaults.
func BuildCoordinatorConfig(file *CoordinatorFileConfig) CoordinatorConfig {
	cfg := DefaultCoordinatorConfig()
	if file == nil {
		return cfg
	}
	if file.Enabled != nil {
		cfg.Enabled = *file.Enabled
	}
	if file.RoutingMode != nil && *file.RoutingMode != "" {
		cfg.RoutingMode = *file.RoutingMode
	}
	if file.FastPathThreshold != nil {
		cfg.FastPathThreshold = *file.FastPathThreshold
	}
	if file.CommandFirst != nil {
		cfg.CommandFirst = *file.CommandFirst
	}
	if file.LLMFallback != nil {
		cfg.LLMFallback = *file.LLMFallback
	}
	if file.AdvisoryValidationTimeoutMs != nil {
		cfg.AdvisoryValidationTimeoutMs = *file.AdvisoryValidationTimeoutMs
	}
	if len(file.CommandWhitelist) > 0 {
		cfg.CommandWhitelist = file.CommandWhitelist
	}
	if file.PriorContextRounds != nil {
		cfg.PriorContextRounds = *file.PriorContextRounds
	}
	if file.SemanticConvergence != nil {
		// inner SemanticConvergence is a value (CoordinatorFileConfig
		// already nil-checked), so merge per-field.
		if file.SemanticConvergence.Enabled != nil {
			cfg.SemanticConvergence.Enabled = file.SemanticConvergence.Enabled
		}
		if file.SemanticConvergence.MinSimilarity != nil {
			cfg.SemanticConvergence.MinSimilarity = file.SemanticConvergence.MinSimilarity
		}
		if file.SemanticConvergence.LookbackN != nil {
			cfg.SemanticConvergence.LookbackN = file.SemanticConvergence.LookbackN
		}
		if file.SemanticConvergence.TimeoutMs != nil {
			cfg.SemanticConvergence.TimeoutMs = file.SemanticConvergence.TimeoutMs
		}
		if file.SemanticConvergence.ModelTier != nil {
			cfg.SemanticConvergence.ModelTier = file.SemanticConvergence.ModelTier
		}
	}
	return cfg
}
