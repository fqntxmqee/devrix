package orchtypes

// SemanticConvergenceConfig is the runtime view of d7.semantic_convergence
// (DM-20260706-006). It controls when the per-WorkItem ItemPipelineRunner
// consults the LLM SemanticVerifier after a code-based Pass verdict.
//
// Production default is Enabled=true. The cheap Jaccard pre-check
// (MinSimilarity) gates the LLM call so the cost is bounded to
// stagnation-suspect rounds only. Healthy rounds (1-3 in a fresh
// session) skip the verify entirely — zero overhead.
type SemanticConvergenceConfig struct {
	Enabled         bool
	MinSimilarity   float64
	LookbackN       int
	TimeoutMs       int
	ModelTier       string
}

// DefaultSemanticConvergenceConfig returns the production default.
// Mirrors sessionorchestrator.DefaultSemanticSimilarityConfig — keep in sync.
func DefaultSemanticConvergenceConfig() SemanticConvergenceConfig {
	return SemanticConvergenceConfig{
		Enabled:       true, // production default ON
		MinSimilarity: 0.85,
		LookbackN:     5,
		TimeoutMs:     8000,
		ModelTier:     "",
	}
}

// Config is the runtime configuration for the D7 orchestration domain (v2.6.0).
//
// Active fields:
//   - Enabled:                     feature flag (orchestration.d7_enabled). When false,
//     D1 routes directly to D2.Process (legacy).
//   - RoutingMode:                 loop_first (default only; rule_orchestrate retired in v6.0.0).
//   - CommandFirst:                if true, recognized commands short-circuit Classify
//     and bypass the LLM (v1.0 default true).
//   - AdvisoryValidationTimeoutMs: advisory D6 validation budget. Timeout → pass.
//   - CommandWhitelist:            recognized command prefixes.
//   - PriorContextRounds (D7-S15, DM-20260628-003): see shared/config.CoordinatorConfig
//     for semantics. 0 (default) disables TurnState + transcript injection.
//   - SemanticConvergence (D7-S16, DM-20260706-006): LLM-driven semantic verify
//     for the MUPS Verify node. See SemanticConvergenceConfig docs.
//
// Retired fields (DM-20260629-001): FastPathThreshold / LLMFallback / ShadowLLMClassify /
// ShadowLLMTimeoutMs + RuleOrchestrateConfig() removed in v2.6.0; FastPath retired in
// PR #239 (DM-20260626-009), rule_orchestrate ingress retired in v6.0.0.
type Config struct {
	Enabled                     bool
	RoutingMode                 RoutingMode
	CommandFirst                bool
	AdvisoryValidationTimeoutMs int
	CommandWhitelist            []string
	PriorContextRounds          int
	SemanticConvergence         SemanticConvergenceConfig
}

// DefaultConfig returns the v2.6.0 default. This is what D1 routes against
// when orchestration.d7_enabled = true.
func DefaultConfig() *Config {
	return &Config{
		Enabled:                     false,
		RoutingMode:                 RoutingModeLoopFirst,
		CommandFirst:                true,
		AdvisoryValidationTimeoutMs: 50,
		CommandWhitelist: []string{
			"/plan", "/stop", "/task", "/help",
		},
		SemanticConvergence: DefaultSemanticConvergenceConfig(),
	}
}

// FileConfig is the YAML deserialization target. BuildConfig merges file
// overrides on top of DefaultConfig (per coding.md §4.2).
type FileConfig struct {
	Enabled                     *bool                       `yaml:"enabled"`
	RoutingMode                 *string                     `yaml:"routing_mode"`
	CommandFirst                *bool                       `yaml:"command_first"`
	AdvisoryValidationTimeoutMs *int                        `yaml:"d6_validation_timeout_ms"`
	CommandWhitelist            []string                    `yaml:"command_whitelist"`
	PriorContextRounds          *int                        `yaml:"prior_context_rounds"`
	SemanticConvergence         *SemanticConvergenceFileConfig `yaml:"semantic_convergence"`
}

// SemanticConvergenceFileConfig mirrors SemanticConvergenceConfig with
// pointer fields so BuildConfig can distinguish "absent in yaml" (keep
// default) from "explicitly false / 0".
type SemanticConvergenceFileConfig struct {
	Enabled       *bool    `yaml:"enabled"`
	MinSimilarity *float64 `yaml:"min_similarity"`
	LookbackN     *int     `yaml:"lookback_n"`
	TimeoutMs     *int     `yaml:"timeout_ms"`
	ModelTier     *string  `yaml:"model_tier"`
}

// BuildConfig merges file over default. nil fields in file keep default.
func BuildConfig(file *FileConfig) *Config {
	cfg := DefaultConfig()
	if file == nil {
		return cfg
	}
	if file.Enabled != nil {
		cfg.Enabled = *file.Enabled
	}
	if file.RoutingMode != nil {
		cfg.RoutingMode = normalizeRoutingMode(RoutingMode(*file.RoutingMode))
	}
	if file.CommandFirst != nil {
		cfg.CommandFirst = *file.CommandFirst
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
		cfg.SemanticConvergence = BuildSemanticConvergenceConfig(file.SemanticConvergence)
	}
	return cfg
}

// BuildSemanticConvergenceConfig merges file over default.
func BuildSemanticConvergenceConfig(file *SemanticConvergenceFileConfig) SemanticConvergenceConfig {
	cfg := DefaultSemanticConvergenceConfig()
	if file == nil {
		return cfg
	}
	if file.Enabled != nil {
		cfg.Enabled = *file.Enabled
	}
	if file.MinSimilarity != nil {
		cfg.MinSimilarity = *file.MinSimilarity
	}
	if file.LookbackN != nil {
		cfg.LookbackN = *file.LookbackN
	}
	if file.TimeoutMs != nil {
		cfg.TimeoutMs = *file.TimeoutMs
	}
	if file.ModelTier != nil {
		cfg.ModelTier = *file.ModelTier
	}
	return cfg
}
