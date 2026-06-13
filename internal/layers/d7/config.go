package d7

// Config is the runtime configuration for the D7 orchestration domain.
//
// v1.0 fields:
//   - Enabled:           feature flag (orchestration.d7_enabled). When false,
//                        D1 routes directly to D2.Process (legacy).
//   - FastPathThreshold: minimum ClassifyIntent confidence for the FastPath.
//                        Below this, the message is routed to OrchestratePath.
//   - CommandFirst:      if true, recognized commands short-circuit Classify
//                        and bypass the LLM (v1.0 default true).
//   - LLMFallback:       v1.0 always false (LLM fallback deferred to v1.1).
//   - D6ValidationTimeoutMs: advisory D6 validation budget. Timeout → pass.
//   - PlanModeApproveGate: when true, Wave triggers require an explicit
//                          Plan approve (per R2 OQ-1 resolution A).
type Config struct {
	Enabled               bool
	FastPathThreshold     int
	CommandFirst          bool
	LLMFallback           bool
	D6ValidationTimeoutMs int
	PlanModeApproveGate   bool
	CommandWhitelist      []string
}

// DefaultConfig returns the v1.0 default. This is what D1 routes against
// when orchestration.d7_enabled = true.
func DefaultConfig() *Config {
	return &Config{
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

// FileConfig is the YAML deserialization target. BuildConfig merges file
// overrides on top of DefaultConfig (per coding.md §4.2).
type FileConfig struct {
	Enabled               *bool    `yaml:"enabled"`
	FastPathThreshold     *int     `yaml:"fast_path_threshold"`
	CommandFirst          *bool    `yaml:"command_first"`
	LLMFallback           *bool    `yaml:"llm_fallback"`
	D6ValidationTimeoutMs *int     `yaml:"d6_validation_timeout_ms"`
	PlanModeApproveGate   *bool    `yaml:"plan_mode_approve_gate"`
	CommandWhitelist      []string `yaml:"command_whitelist"`
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
	if file.FastPathThreshold != nil {
		cfg.FastPathThreshold = *file.FastPathThreshold
	}
	if file.CommandFirst != nil {
		cfg.CommandFirst = *file.CommandFirst
	}
	if file.LLMFallback != nil {
		cfg.LLMFallback = *file.LLMFallback
	}
	if file.D6ValidationTimeoutMs != nil {
		cfg.D6ValidationTimeoutMs = *file.D6ValidationTimeoutMs
	}
	if file.PlanModeApproveGate != nil {
		cfg.PlanModeApproveGate = *file.PlanModeApproveGate
	}
	if len(file.CommandWhitelist) > 0 {
		cfg.CommandWhitelist = file.CommandWhitelist
	}
	return cfg
}
