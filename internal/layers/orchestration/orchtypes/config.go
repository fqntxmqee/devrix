package orchtypes

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
	}
}

// FileConfig is the YAML deserialization target. BuildConfig merges file
// overrides on top of DefaultConfig (per coding.md §4.2).
type FileConfig struct {
	Enabled                     *bool    `yaml:"enabled"`
	RoutingMode                 *string  `yaml:"routing_mode"`
	CommandFirst                *bool    `yaml:"command_first"`
	AdvisoryValidationTimeoutMs *int     `yaml:"d6_validation_timeout_ms"`
	CommandWhitelist            []string `yaml:"command_whitelist"`
	PriorContextRounds          *int     `yaml:"prior_context_rounds"`
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
	return cfg
}
