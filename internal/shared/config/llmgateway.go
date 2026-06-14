// Package config provides shared configuration types for devrix.
//
// LLM Gateway types below are re-exports from llmgateway/configure.
// Deprecated: import llmgateway/configure directly. These aliases will be
// removed in the next release cycle.
package config

import "github.com/devrix/devrix/internal/layers/llmgateway/configure"

// LLMGatewayFileConfig is the YAML shape for llm_gateway.
type LLMGatewayFileConfig = configure.LLMGatewayFileConfig

// LLMGatewayConfig is the resolved Layer 3 configuration.
type LLMGatewayConfig = configure.LLMGatewayConfig

// LLMFeatureFlags is a set of v1.1 runtime feature switches.
type LLMFeatureFlags = configure.LLMFeatureFlags

// LLMCircuitBreakerConfig holds resolved circuit breaker settings.
type LLMCircuitBreakerConfig = configure.LLMCircuitBreakerConfig

// LLMProviderRuntimeConfig holds resolved provider settings.
type LLMProviderRuntimeConfig = configure.LLMProviderRuntimeConfig

// LLMRetryConfig holds resolved retry settings.
type LLMRetryConfig = configure.LLMRetryConfig

// LLMCircuitBreakerFileConfig holds circuit breaker YAML fields.
type LLMCircuitBreakerFileConfig = configure.LLMCircuitBreakerFileConfig

// LLMProviderConfig holds per-provider YAML fields.
type LLMProviderConfig = configure.LLMProviderConfig

// LLMRetryFileConfig holds retry YAML fields.
type LLMRetryFileConfig = configure.LLMRetryFileConfig

// DefaultLLMGatewayConfig returns V1 defaults.
//
// Deprecated: use configure.DefaultLLMGatewayConfig directly.
func DefaultLLMGatewayConfig() *LLMGatewayConfig {
	return configure.DefaultLLMGatewayConfig()
}

// DefaultFeatureFlags returns v1.1 D4-B defaults.
//
// Deprecated: use configure.DefaultFeatureFlags directly.
func DefaultFeatureFlags() LLMFeatureFlags {
	return configure.DefaultFeatureFlags()
}

// BuildLLMGatewayConfig merges file config over defaults.
//
// Deprecated: use configure.BuildLLMGatewayConfig directly.
func BuildLLMGatewayConfig(file *LLMGatewayFileConfig) *LLMGatewayConfig {
	return configure.BuildLLMGatewayConfig(file)
}

// LoadLLMGatewayConfig is defined in loader.go (this package).
