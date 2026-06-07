package config_test

import (
	"os"
	"testing"
	"time"

	llmconfig "github.com/devrix/devrix/internal/layers/llmgateway/config"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// Covers: L5-LLM-09
func TestLoader_should_load_default_provider_config(t *testing.T) {
	loader := llmconfig.NewLoader()
	cfg, err := loader.Load(sharedconfig.DefaultLLMGatewayConfig())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultProvider != "minimax" {
		t.Errorf("DefaultProvider: got %s", cfg.DefaultProvider)
	}
	deepseek, ok := cfg.Providers["deepseek"]
	if !ok {
		t.Fatal("expected deepseek provider")
	}
	if deepseek.BaseURL == "" || deepseek.APIKeyEnv != "DEEPSEEK_API_KEY" {
		t.Errorf("deepseek config: %+v", deepseek)
	}
	if cfg.CircuitBreaker.FailureThreshold != 5 {
		t.Errorf("FailureThreshold: got %d", cfg.CircuitBreaker.FailureThreshold)
	}
}

// Covers: L5-LLM-09
func TestLoader_should_merge_file_config(t *testing.T) {
	file := &sharedconfig.LLMGatewayFileConfig{
		DefaultProvider: "deepseek",
		Providers: map[string]sharedconfig.LLMProviderConfig{
			"deepseek": {
				DefaultModel: "deepseek-v4-pro",
				Timeout:      90 * time.Second,
			},
		},
	}
	loader := llmconfig.NewLoader()
	cfg, err := loader.LoadFromFileConfig(file)
	if err != nil {
		t.Fatalf("LoadFromFileConfig: %v", err)
	}
	if cfg.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider: got %s", cfg.DefaultProvider)
	}
	if cfg.Providers["deepseek"].DefaultModel != "deepseek-v4-pro" {
		t.Errorf("DefaultModel: got %s", cfg.Providers["deepseek"].DefaultModel)
	}
	if cfg.Providers["deepseek"].Timeout != 90*time.Second {
		t.Errorf("Timeout: got %v", cfg.Providers["deepseek"].Timeout)
	}
}

// Covers: L5-LLM-09
func TestLoader_should_reject_unknown_default_provider(t *testing.T) {
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	cfg.DefaultProvider = "unknown"
	loader := llmconfig.NewLoader()
	_, err := loader.Load(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAPIKey_should_read_from_env(t *testing.T) {
	const envName = "DEVRIX_TEST_LLM_KEY"
	_ = os.Setenv(envName, "secret-key")
	defer os.Unsetenv(envName)

	p := sharedconfig.LLMProviderRuntimeConfig{APIKeyEnv: envName}
	key, ok := llmconfig.APIKey(p)
	if !ok || key != "secret-key" {
		t.Errorf("APIKey: got %q ok=%v", key, ok)
	}
}
