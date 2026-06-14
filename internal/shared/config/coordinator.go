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
	FastPathThreshold           int      `yaml:"fast_path_threshold"`
	CommandFirst                bool     `yaml:"command_first"`
	LLMFallback                 bool     `yaml:"llm_fallback"`
	AdvisoryValidationTimeoutMs int      `yaml:"d6_validation_timeout_ms"`
	PlanModeApproveGate         bool     `yaml:"plan_mode_approve_gate"`
	CommandWhitelist            []string `yaml:"command_whitelist"`
}

// DefaultCoordinatorConfig returns the v1.0 defaults — matches
// coordinator.DefaultConfig().
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		Enabled:                     false,
		FastPathThreshold:           90,
		CommandFirst:                true,
		LLMFallback:                 false,
		AdvisoryValidationTimeoutMs: 50,
		PlanModeApproveGate:         true,
		CommandWhitelist: []string{
			"/plan", "/stop", "/task", "/help",
		},
	}
}
