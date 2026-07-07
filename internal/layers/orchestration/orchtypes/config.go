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

// DAGExecutorConfig is the runtime view of the `dag_executor` YAML
// sub-config under the d7 block (DM-20260707-001 PR-D, T29). The YAML
// key is `dag_executor` (see DAGExecutorFileConfig below). When
// `Enabled` is true, ItemPipelineRunner forks into the multi-intent DAG
// execution path (executePlanDAG) whenever a Plan emits pl.DAG +
// pl.IntentSegmentSet. When false (default), the fork gate at
// item_pipeline.go:414 short-circuits and the legacy single-WorkItem
// path runs unchanged — keeping the flag a zero-impact gate for staged
// rollout (DM-20260707-001 §3 Backward Compatibility).
//
// Why a separate config rather than reusing Enabled:
//   - the legacy D7 enabled flag gates the entire D7 layer (D1 routing);
//     this flag gates a *sub-feature inside* D7 (multi-intent DAG).
//   - keeping them independent lets ops flip DAG off without disabling
//     the rest of D7 (e.g. semantic convergence, observation fast-path).
//
// Devrix.yaml example:
//   d7:
//     enabled: true
//     dag_executor:
//       enabled: true           # default false
//       max_fan_out: 8          # default 8
//       max_retry_on_partial_fail: 1   # default 1
type DAGExecutorConfig struct {
	Enabled bool
	// MaxFanOut caps the validated PlanDAG node count. Default 8 matches
	// DAGValidator's MaxFanOut sentinel; the WaveScheduler hard 4-worker
	// cap is unchanged.
	MaxFanOut int
	// MaxRetryOnPartialFail caps the auto-retry loop on VerdictPartial.
	// Matches design.md §2.12 MaxRetry=1 (i.e. only one retry before the
	// next segment), but exposed here so prod can dial it down for
	// latency-sensitive workloads.
	MaxRetryOnPartialFail int
}

// DefaultDAGExecutorConfig returns the production default (flag OFF).
// Mirrors proposal.md §6: "first 5% → 100% across 2 weeks" — until ops
// flips the flag, the multi-intent path is dormant.
func DefaultDAGExecutorConfig() DAGExecutorConfig {
	return DAGExecutorConfig{
		Enabled:               false,
		MaxFanOut:             8,
		MaxRetryOnPartialFail: 1,
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
//   - DAGExecutor (DM-20260707-001 PR-D, T29): multi-intent DAG routing.
//     When Enabled is true, ItemPipelineRunner forks into the DAG path
//     when Plan emits pl.DAG + pl.IntentSegmentSet. Default false; see
//     DAGExecutorConfig docs.
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
	DAGExecutor                 DAGExecutorConfig
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
		DAGExecutor:         DefaultDAGExecutorConfig(),
	}
}

// FileConfig is the YAML deserialization target. BuildConfig merges file
// overrides on top of DefaultConfig (per coding.md §4.2).
type FileConfig struct {
	Enabled                     *bool                          `yaml:"enabled"`
	RoutingMode                 *string                        `yaml:"routing_mode"`
	CommandFirst                *bool                          `yaml:"command_first"`
	AdvisoryValidationTimeoutMs *int                           `yaml:"d6_validation_timeout_ms"`
	CommandWhitelist            []string                       `yaml:"command_whitelist"`
	PriorContextRounds          *int                           `yaml:"prior_context_rounds"`
	SemanticConvergence         *SemanticConvergenceFileConfig `yaml:"semantic_convergence"`
	DAGExecutor                 *DAGExecutorFileConfig         `yaml:"dag_executor"`
}

// DAGExecutorFileConfig mirrors DAGExecutorConfig with pointer fields so
// BuildConfig can distinguish "absent in yaml" (keep default) from
// "explicitly false / 0". D7 PR-D concern: YAML underscore is the wire
// format, so devrix.yaml uses `dag_executor.enabled`, not `dagExecutor`.
type DAGExecutorFileConfig struct {
	Enabled               *bool `yaml:"enabled"`
	MaxFanOut             *int  `yaml:"max_fan_out"`
	MaxRetryOnPartialFail *int  `yaml:"max_retry_on_partial_fail"`
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
	if file.DAGExecutor != nil {
		cfg.DAGExecutor = BuildDAGExecutorConfig(file.DAGExecutor)
	}
	return cfg
}

// BuildDAGExecutorConfig merges file over default. nil fields preserve
// the default; an explicitly-set Enabled=false is honored (rollout OFF).
func BuildDAGExecutorConfig(file *DAGExecutorFileConfig) DAGExecutorConfig {
	cfg := DefaultDAGExecutorConfig()
	if file == nil {
		return cfg
	}
	if file.Enabled != nil {
		cfg.Enabled = *file.Enabled
	}
	if file.MaxFanOut != nil {
		cfg.MaxFanOut = *file.MaxFanOut
	}
	if file.MaxRetryOnPartialFail != nil {
		cfg.MaxRetryOnPartialFail = *file.MaxRetryOnPartialFail
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
