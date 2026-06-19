package configure_test

import (
	"os"
	"testing"
	"time"

	llmconfig "github.com/devrix/devrix/internal/layers/llmgateway/configure"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// T: D3-S6-A01-T01
//
// DSAFT: D3-S6-A03 (v2.x). The compiled default no longer ships model names
// or a default provider; callers must supply them via config (project or
// user-level). This test verifies that the *infrastructure* part of the
// default (provider catalog with BaseURL/APIKeyEnv, circuit breaker values)
// still loads cleanly when the file config provides the model metadata.
func TestLoader_should_load_default_provider_config(t *testing.T) {
	// Apply a file config that picks the active provider + a model.
	file := &sharedconfig.LLMGatewayFileConfig{
		DefaultProvider: "minimax",
		DefaultModel:    "MiniMax-M3",
		Providers: map[string]sharedconfig.LLMProviderConfig{
			"minimax": {DefaultModel: "MiniMax-M3"},
		},
	}
	resolved := sharedconfig.BuildLLMGatewayConfig(file)
	loader := llmconfig.NewLoader()
	cfg, err := loader.Load(resolved)
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

// T: D3-S6-A01-T01
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

// T: D3-S6-A01-T01
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
