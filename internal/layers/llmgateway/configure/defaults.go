package configure

import "time"

// DefaultFeatureFlags 返回 v1.1 D4-B 决议固化的默认值。
//
// ON  : ResilienceEmitEnabled, SafetyLatencyEventEnabled (cardinality 受控)
// OFF : MetricEmitWarn (避免污染日志；走 D5 健康检查)
func DefaultFeatureFlags() LLMFeatureFlags {
	return LLMFeatureFlags{
		ResilienceEmitEnabled:     true,
		SafetyLatencyEventEnabled: true,
		MetricEmitWarn:            false,
	}
}

// DefaultLLMGatewayConfig returns infrastructure-only defaults.
//
// DSAFT: D3-S6-A03 (model catalog, v2.x). The defaults deliberately contain
// NO model-name strings. Provider INFRASTRUCTURE (base_url, api_key_env,
// timeout, retry, headers) is shipped because it is not a "model choice" —
// it describes the provider's transport contract. Model *names* are
// resolved exclusively from the layered config:
//
//	compiled defaults (this fn, empty)
//	  ← project config (devrix.yaml llm_gateway section, optional)
//	  ← user config  ( ~/.devrix/config.yaml llm_gateway section, optional)
//	  ← environment  (DEVRIX_LLM_* vars, optional)
//
// If the resolved config still has empty default_model / default_provider
// after all layers, ValidateLLMGatewayConfig fails fast at startup and
// points the user to devrix.yaml or ~/.devrix/config.yaml.
//
// DM-20260629-003 PR-3 (#1 god-fn-split pt2): extracted from shared_config.go
// (originally 50 LOC inline). shared_config.go now only carries type defs
// + the BuildLLMGatewayConfig merge function.
func DefaultLLMGatewayConfig() *LLMGatewayConfig {
	return &LLMGatewayConfig{
		DefaultProvider: "",
		DefaultModel:    "",
		DefaultTier:     "default",
		FeatureFlags:    DefaultFeatureFlags(),
		ModelTiers:      map[string]string{},
		ModelRouting:    map[string]string{},
		CircuitBreaker: LLMCircuitBreakerConfig{
			FailureThreshold:  5,
			SuccessThreshold:  2,
			OpenDuration:      30 * time.Second,
			HalfOpenMaxProbes: 1,
			Scope:             "provider",
		},
		Providers: map[string]LLMProviderRuntimeConfig{
			"deepseek": {
				Type:         "deepseek",
				BaseURL:      "https://api.deepseek.com/v1",
				APIKeyEnv:    "DEEPSEEK_API_KEY",
				DefaultModel: "",
				Timeout:      60 * time.Second,
				MaxTokens:    8192,
				Temperature:  0.7,
				Retry: LLMRetryConfig{
					MaxAttempts:  3,
					InitialDelay: time.Second,
					MaxDelay:     10 * time.Second,
					Backoff:      2.0,
				},
			},
			"minimax": {
				Type:         "minimax",
				BaseURL:      "https://api.minimaxi.com/v1",
				APIKeyEnv:    "MINIMAX_API_KEY",
				DefaultModel: "",
				Timeout:      60 * time.Second,
				MaxTokens:    8192,
				Temperature:  0.7,
				Retry: LLMRetryConfig{
					MaxAttempts:  3,
					InitialDelay: time.Second,
					MaxDelay:     10 * time.Second,
					Backoff:      2.0,
				},
			},
		},
	}
}