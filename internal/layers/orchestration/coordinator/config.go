package coordinator

// Config is the runtime configuration for the D7 orchestration domain.
//
// v1.0 fields:
//   - Enabled:           feature flag (orchestration.d7_enabled). When false,
//     D1 routes directly to D2.Process (legacy).
//   - RoutingMode:       loop_first (default) or rule_orchestrate (legacy ingress).
//   - FastPathThreshold: minimum ClassifyIntent confidence for the FastPath.
//     Below this, the message is routed to OrchestratePath (rule_orchestrate only).
//   - CommandFirst:      if true, recognized commands short-circuit Classify
//     and bypass the LLM (v1.0 default true).
//   - LLMFallback:       v1.0 always false (LLM fallback deferred to v1.1).
//   - AdvisoryValidationTimeoutMs: advisory D6 validation budget. Timeout → pass.
type Config struct {
	Enabled                     bool
	RoutingMode                 RoutingMode
	FastPathThreshold           int
	CommandFirst                bool
	LLMFallback                 bool
	AdvisoryValidationTimeoutMs int
	CommandWhitelist            []string
	// ShadowLLMClassify enables the async LLM classify shadow on the
	// IntentOrchestrate tail (R2 §5 命题 C). Default false; enable per
	// deployment to gather v1.1 cold-start samples.
	ShadowLLMClassify bool
	// ShadowLLMTimeoutMs is the LLM call timeout for the shadow in
	// milliseconds. Default 500. Only used when ShadowLLMClassify=true.
	ShadowLLMTimeoutMs int
}

// DefaultConfig returns the v1.0 default. This is what D1 routes against
// when orchestration.d7_enabled = true.
func DefaultConfig() *Config {
	return &Config{
		Enabled:                     false,
		RoutingMode:                 RoutingModeLoopFirst,
		FastPathThreshold:           90,
		CommandFirst:                true,
		LLMFallback:                 false,
		AdvisoryValidationTimeoutMs: 50,
		CommandWhitelist: []string{
			"/plan", "/stop", "/task", "/help",
		},
		ShadowLLMClassify:  false,
		ShadowLLMTimeoutMs: 500,
	}
}

// RuleOrchestrateConfig returns defaults with legacy ingress routing (DM-20260615-004).
func RuleOrchestrateConfig() *Config {
	cfg := DefaultConfig()
	cfg.RoutingMode = RoutingModeRuleOrchestrate
	return cfg
}

// FileConfig is the YAML deserialization target. BuildConfig merges file
// overrides on top of DefaultConfig (per coding.md §4.2).
type FileConfig struct {
	Enabled                     *bool    `yaml:"enabled"`
	RoutingMode                 *string  `yaml:"routing_mode"`
	FastPathThreshold           *int     `yaml:"fast_path_threshold"`
	CommandFirst                *bool    `yaml:"command_first"`
	LLMFallback                 *bool    `yaml:"llm_fallback"`
	AdvisoryValidationTimeoutMs *int     `yaml:"d6_validation_timeout_ms"`
	CommandWhitelist            []string `yaml:"command_whitelist"`
	ShadowLLMClassify           *bool    `yaml:"shadow_llm_classify"`
	ShadowLLMTimeoutMs          *int     `yaml:"shadow_llm_timeout_ms"`
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
	if file.ShadowLLMClassify != nil {
		cfg.ShadowLLMClassify = *file.ShadowLLMClassify
	}
	if file.ShadowLLMTimeoutMs != nil {
		cfg.ShadowLLMTimeoutMs = *file.ShadowLLMTimeoutMs
	}
	return cfg
}
