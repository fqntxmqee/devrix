package config

// D7Config is the YAML-loaded, runtime configuration for the D7
// orchestration domain. The fields mirror d7.Config but live in the
// shared config package to avoid an import cycle (d7 imports shared
// contracts/types; shared config is allowed to import d7).
//
// BuildD7Config merges a file-loaded D7FileConfig into the v1.0 defaults.
// Caller passes the result to d7.NewSessionOrchestrator via d7.BuildConfig.
type D7Config struct {
	Enabled               bool     `yaml:"enabled"`
	FastPathThreshold     int      `yaml:"fast_path_threshold"`
	CommandFirst          bool     `yaml:"command_first"`
	LLMFallback           bool     `yaml:"llm_fallback"`
	D6ValidationTimeoutMs int      `yaml:"d6_validation_timeout_ms"`
	PlanModeApproveGate   bool     `yaml:"plan_mode_approve_gate"`
	CommandWhitelist      []string `yaml:"command_whitelist"`
}

// DefaultD7Config returns the v1.0 defaults — matches d7.DefaultConfig().
func DefaultD7Config() D7Config {
	return D7Config{
		Enabled:               false,
		FastPathThreshold:     90,
		CommandFirst:          true,
		LLMFallback:           false,
		D6ValidationTimeoutMs: 50,
		PlanModeApproveGate:   true,
		CommandWhitelist: []string{
			"/plan", "/stop", "/task", "/help",
		},
	}
}
