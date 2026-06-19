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
	active, ok := cfg.Providers[cfg.DefaultProvider]
	if !ok {
		return fmt.Errorf("llm_gateway.default_provider %q not found in providers", cfg.DefaultProvider)
	}
	// Only the ACTIVE default provider needs a model — other providers in
	// the map may be configured lazily (e.g. fallback path activated on
	// first 5xx). DSAFT: D3-S6-A03 (v2.x) — the compiled default no longer
	// ships model names, so we can't require every entry to have one.
	if active.BaseURL == "" {
		return fmt.Errorf("provider %q: base_url is required", cfg.DefaultProvider)
	}
	if active.APIKeyEnv == "" {
		return fmt.Errorf("provider %q: api_key_env is required", cfg.DefaultProvider)
	}
	if active.DefaultModel == "" && cfg.DefaultModel == "" {
		return fmt.Errorf("provider %q: default_model or global default_model required", cfg.DefaultProvider)
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
