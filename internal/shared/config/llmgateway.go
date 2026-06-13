package config

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

// DefaultLLMGatewayConfig returns V1 defaults.
func DefaultLLMGatewayConfig() *LLMGatewayConfig {
	return &LLMGatewayConfig{
		DefaultProvider: "minimax",
		DefaultModel:    "MiniMax-M2.7-highspeed",
		DefaultTier:     "default",
		ModelTiers: map[string]string{
			"fast":     "MiniMax-M2.7-highspeed",
			"default":  "MiniMax-M2.7-highspeed",
			"powerful": "deepseek-v4-latest",
		},
		ModelRouting: map[string]string{
			"deepseek-*": "deepseek",
			"minimax-*":  "minimax",
			"MiniMax-*":  "minimax",
		},
		CircuitBreaker: LLMCircuitBreakerConfig{
			FailureThreshold:  5,
			SuccessThreshold:  2,
			OpenDuration:      30 * time.Second,
			HalfOpenMaxProbes: 1,
			Scope:             "provider",
		},
		Providers: map[string]LLMProviderRuntimeConfig{
			"deepseek": {
				Type:          "deepseek",
				BaseURL:       "https://api.deepseek.com/v1",
				APIKeyEnv:     "DEEPSEEK_API_KEY",
				DefaultModel:  "deepseek-v4-flash",
				FallbackModel: "deepseek-v4-pro",
				Timeout:       60 * time.Second,
				MaxTokens:     8192,
				Temperature:   0.7,
				Retry: LLMRetryConfig{
					MaxAttempts:  3,
					InitialDelay: time.Second,
					MaxDelay:     10 * time.Second,
					Backoff:      2.0,
				},
			},
			"minimax": {
				Type:          "minimax",
				BaseURL:       "https://api.minimaxi.com/v1",
				APIKeyEnv:     "MINIMAX_API_KEY",
				DefaultModel:  "MiniMax-M2.7-highspeed",
				FallbackModel: "MiniMax-M2.5-highspeed",
				Timeout:       60 * time.Second,
				MaxTokens:     8192,
				Temperature:   0.7,
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

// BuildLLMGatewayConfig merges file config over defaults.
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
