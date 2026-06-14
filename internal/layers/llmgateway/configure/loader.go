package configure

import (
	"fmt"
	"os"
)

// Loader validates and normalizes LLM gateway configuration.
type Loader struct{}

// NewLoader creates a config loader.
func NewLoader() *Loader {
	return &Loader{}
}

// Load merges defaults with the given config and validates providers.
func (l *Loader) Load(cfg *LLMGatewayConfig) (*LLMGatewayConfig, error) {
	if cfg == nil {
		cfg = DefaultLLMGatewayConfig()
	}
	if err := l.validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadFromFileConfig builds and validates from YAML file section.
func (l *Loader) LoadFromFileConfig(file *LLMGatewayFileConfig) (*LLMGatewayConfig, error) {
	return l.Load(BuildLLMGatewayConfig(file))
}

func (l *Loader) validate(cfg *LLMGatewayConfig) error {
	if cfg.DefaultProvider == "" {
		return fmt.Errorf("llm_gateway.default_provider is required")
	}
	if _, ok := cfg.Providers[cfg.DefaultProvider]; !ok {
		return fmt.Errorf("llm_gateway.default_provider %q not found in providers", cfg.DefaultProvider)
	}
	for name, p := range cfg.Providers {
		if p.BaseURL == "" {
			return fmt.Errorf("provider %q: base_url is required", name)
		}
		if p.APIKeyEnv == "" {
			return fmt.Errorf("provider %q: api_key_env is required", name)
		}
		if p.DefaultModel == "" && cfg.DefaultModel == "" {
			return fmt.Errorf("provider %q: default_model or global default_model required", name)
		}
	}
	return nil
}

// APIKey returns the API key for a provider from its configured env var.
func APIKey(provider LLMProviderRuntimeConfig) (string, bool) {
	if provider.APIKeyEnv == "" {
		return "", false
	}
	val := os.Getenv(provider.APIKeyEnv)
	return val, val != ""
}
