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
	}
}

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
	return cfg
}
