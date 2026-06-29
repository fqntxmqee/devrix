package configure

import "time"

// LLMGatewayFileConfig is the YAML shape for llm_gateway.
type LLMGatewayFileConfig struct {
	DefaultProvider string                       `yaml:"default_provider"`
	DefaultModel    string                       `yaml:"default_model"`
	DefaultTier     string                       `yaml:"default_tier"`
	ModelTiers      map[string]string            `yaml:"model_tiers"`
	ModelRouting    map[string]string            `yaml:"model_routing"`
	CircuitBreaker  LLMCircuitBreakerFileConfig  `yaml:"circuit_breaker"`
	Providers       map[string]LLMProviderConfig `yaml:"providers"`
}

// LLMCircuitBreakerFileConfig holds circuit breaker YAML fields.
type LLMCircuitBreakerFileConfig struct {
	FailureThreshold  int           `yaml:"failure_threshold"`
	SuccessThreshold  int           `yaml:"success_threshold"`
	OpenDuration      time.Duration `yaml:"open_duration"`
	HalfOpenMaxProbes int           `yaml:"half_open_max_probes"`
	Scope             string        `yaml:"scope"`
}

// LLMProviderConfig holds per-provider YAML fields.
type LLMProviderConfig struct {
	Type          string             `yaml:"type"`
	BaseURL       string             `yaml:"base_url"`
	APIKeyEnv     string             `yaml:"api_key_env"`
	DefaultModel  string             `yaml:"default_model"`
	FallbackModel string             `yaml:"fallback_model"`
	Timeout       time.Duration      `yaml:"timeout"`
	MaxTokens     int                `yaml:"max_tokens"`
	Temperature   float64            `yaml:"temperature"`
	Retry         LLMRetryFileConfig `yaml:"retry"`
	Headers       map[string]string  `yaml:"headers"`
}

// LLMRetryFileConfig holds retry YAML fields.
type LLMRetryFileConfig struct {
	MaxAttempts  int           `yaml:"max_attempts"`
	InitialDelay time.Duration `yaml:"initial_delay"`
	MaxDelay     time.Duration `yaml:"max_delay"`
	Backoff      float64       `yaml:"backoff"`
}

// LLMGatewayConfig is the resolved Layer 3 configuration.
type LLMGatewayConfig struct {
	DefaultProvider string
	DefaultModel    string
	DefaultTier     string
	ModelTiers      map[string]string
	ModelRouting    map[string]string
	CircuitBreaker  LLMCircuitBreakerConfig
	Providers       map[string]LLMProviderRuntimeConfig
	// FeatureFlags — v1.1 D4-B 决议固化的运行时开关 (D3-S6-A01-F05)。
	// 默认值见 DefaultLLMGatewayConfig 与 DefaultFeatureFlags。
	FeatureFlags LLMFeatureFlags
}

// LLMFeatureFlags 是一组 v1.1 引入的运行时开关。
//
// DSAFT: D3-S6-A01-F05 FeatureFlagDefaults (v1.1 F9, D4-B 决议)。
// - ResilienceEmitEnabled: ON 时 attach breaker observer
//   (emit llm_breaker_state metric + flow.breaker.* EngineEvent)
// - SafetyLatencyEventEnabled: ON 时 emit span event safety.check.duration_ms
// - MetricEmitWarn: ON 时 emit 失败写 warn 日志（OFF 时走 D5 健康检查）
//
// 8 组合 (2^3) 单元测试见 llmgateway_features_test.go (D3-S6-A01-T02)。
type LLMFeatureFlags struct {
	ResilienceEmitEnabled      bool
	SafetyLatencyEventEnabled  bool
	MetricEmitWarn             bool
}

// LLMCircuitBreakerConfig holds resolved circuit breaker settings.
type LLMCircuitBreakerConfig struct {
	FailureThreshold  int
	SuccessThreshold  int
	OpenDuration      time.Duration
	HalfOpenMaxProbes int
	Scope             string
}

// LLMProviderRuntimeConfig holds resolved provider settings.
type LLMProviderRuntimeConfig struct {
	Type          string
	BaseURL       string
	APIKeyEnv     string
	DefaultModel  string
	FallbackModel string
	Timeout       time.Duration
	MaxTokens     int
	Temperature   float64
	Retry         LLMRetryConfig
	Headers       map[string]string
}

// LLMRetryConfig holds resolved retry settings.
type LLMRetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Backoff      float64
}

// BuildLLMGatewayConfig merges file config over defaults.
//
// DM-20260629-003 PR-3 (#1 god-fn-split pt2): was 80 LOC inline (in a
// 291 LOC file). The default registry now lives in defaults.go; this
// function only owns the file-overrides-defaults merge (the 3-way
// user-override entry point lives in BuildLLMGatewayConfigWithUser,
// delegating to merge_user.go).
func BuildLLMGatewayConfig(file *LLMGatewayFileConfig) *LLMGatewayConfig {
	cfg := DefaultLLMGatewayConfig()
	if file == nil {
		return cfg
	}
	if file.DefaultProvider != "" {
		cfg.DefaultProvider = file.DefaultProvider
	}
	if file.DefaultModel != "" {
		cfg.DefaultModel = file.DefaultModel
	}
	if file.DefaultTier != "" {
		cfg.DefaultTier = file.DefaultTier
	}
	if len(file.ModelTiers) > 0 {
		cfg.ModelTiers = file.ModelTiers
	}
	if len(file.ModelRouting) > 0 {
		cfg.ModelRouting = file.ModelRouting
	}
	if file.CircuitBreaker.FailureThreshold != 0 {
		cfg.CircuitBreaker.FailureThreshold = file.CircuitBreaker.FailureThreshold
	}
	if file.CircuitBreaker.SuccessThreshold != 0 {
		cfg.CircuitBreaker.SuccessThreshold = file.CircuitBreaker.SuccessThreshold
	}
	if file.CircuitBreaker.OpenDuration != 0 {
		cfg.CircuitBreaker.OpenDuration = file.CircuitBreaker.OpenDuration
	}
	if file.CircuitBreaker.Scope != "" {
		cfg.CircuitBreaker.Scope = file.CircuitBreaker.Scope
	}
	if file.CircuitBreaker.HalfOpenMaxProbes != 0 {
		cfg.CircuitBreaker.HalfOpenMaxProbes = file.CircuitBreaker.HalfOpenMaxProbes
	}
	for name, p := range file.Providers {
		existing := cfg.Providers[name]
		if p.Type != "" {
			existing.Type = p.Type
		}
		if p.BaseURL != "" {
			existing.BaseURL = p.BaseURL
		}
		if p.APIKeyEnv != "" {
			existing.APIKeyEnv = p.APIKeyEnv
		}
		if p.DefaultModel != "" {
			existing.DefaultModel = p.DefaultModel
		}
		if p.FallbackModel != "" {
			existing.FallbackModel = p.FallbackModel
		}
		if p.Timeout != 0 {
			existing.Timeout = p.Timeout
		}
		if p.MaxTokens != 0 {
			existing.MaxTokens = p.MaxTokens
		}
		if p.Temperature != 0 {
			existing.Temperature = p.Temperature
		}
		if p.Retry.MaxAttempts != 0 {
			existing.Retry.MaxAttempts = p.Retry.MaxAttempts
		}
		if p.Retry.InitialDelay != 0 {
			existing.Retry.InitialDelay = p.Retry.InitialDelay
		}
		if p.Retry.MaxDelay != 0 {
			existing.Retry.MaxDelay = p.Retry.MaxDelay
		}
		if p.Retry.Backoff != 0 {
			existing.Retry.Backoff = p.Retry.Backoff
		}
		if len(p.Headers) > 0 {
			existing.Headers = p.Headers
		}
		cfg.Providers[name] = existing
	}
	return cfg
}

// BuildLLMGatewayConfigWithUser is the full layered-config entry point.
//
// Merge order (highest priority wins):
//  1. user  — from internal/shared/config.UserConfig.LLMGateway
//  2. file  — from project config (devrix.yaml llm_gateway section)
//  3. defaults — from DefaultLLMGatewayConfig (infrastructure only, no model names)
//
// `user` may be nil (no user override declared). `file` may be nil
// (no project config found). The merged result is validated via
// ValidateLLMGatewayConfig; a nil DefaultModel / DefaultProvider fails
// fast with ErrLLMConfigMissing so the user sees a clear pointer to
// where to set it.
//
// DSAFT: D3-S6-A03 (model catalog, v2.x). The function is the single
// production entry point for resolving an LLMGatewayConfig — callers
// MUST go through this (or its sibling BuildLLMGatewayConfig) instead
// of stitching configs themselves.
//
// DM-20260629-003 PR-3: the 3-way user-override merge helpers now live
// in merge_user.go (extracted from this file's body to keep this
// function as a thin orchestration shim).
func BuildLLMGatewayConfigWithUser(file, user *LLMGatewayFileConfig) (*LLMGatewayConfig, error) {
	merged := MergeLLMGatewayFileConfig(file, user)
	resolved := BuildLLMGatewayConfig(merged)
	if err := ValidateLLMGatewayConfig(resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}