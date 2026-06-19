package configure

import (
	"errors"
	"testing"
	"time"
)

func validCfg() *LLMGatewayConfig {
	return &LLMGatewayConfig{
		DefaultProvider: "minimax",
		DefaultModel:    "any-model",
		DefaultTier:     "default",
		Providers: map[string]LLMProviderRuntimeConfig{
			"minimax": {
				Type:         "minimax",
				BaseURL:      "https://api.example.com",
				APIKeyEnv:    "MINIMAX_API_KEY",
				DefaultModel: "any-model",
				Timeout:      60 * time.Second,
			},
		},
	}
}

func TestValidateLLMGatewayConfig_OK(t *testing.T) {
	if err := ValidateLLMGatewayConfig(validCfg()); err != nil {
		t.Fatalf("valid cfg should pass, got %v", err)
	}
}

func TestValidateLLMGatewayConfig_Nil(t *testing.T) {
	err := ValidateLLMGatewayConfig(nil)
	if err == nil {
		t.Fatal("nil cfg should fail")
	}
	if !errors.Is(err, ErrLLMConfigMissing) {
		t.Errorf("expected ErrLLMConfigMissing sentinel, got %v", err)
	}
}

func TestValidateLLMGatewayConfig_MissingDefaultModel(t *testing.T) {
	cfg := validCfg()
	cfg.DefaultModel = ""
	err := ValidateLLMGatewayConfig(cfg)
	if err == nil {
		t.Fatal("missing default_model should fail")
	}
	if !errors.Is(err, ErrLLMConfigMissing) {
		t.Errorf("expected ErrLLMConfigMissing, got %v", err)
	}
	if !contains(err.Error(), "default_model") {
		t.Errorf("error should name default_model, got: %v", err)
	}
}

func TestValidateLLMGatewayConfig_MissingDefaultProvider(t *testing.T) {
	cfg := validCfg()
	cfg.DefaultProvider = ""
	err := ValidateLLMGatewayConfig(cfg)
	if err == nil {
		t.Fatal("missing default_provider should fail")
	}
	if !errors.Is(err, ErrLLMConfigMissing) {
		t.Errorf("expected ErrLLMConfigMissing, got %v", err)
	}
	if !contains(err.Error(), "default_provider") {
		t.Errorf("error should name default_provider, got: %v", err)
	}
}

func TestValidateLLMGatewayConfig_ProviderEntryMissing(t *testing.T) {
	cfg := validCfg()
	cfg.DefaultProvider = "anthropic" // not in Providers map
	err := ValidateLLMGatewayConfig(cfg)
	if err == nil {
		t.Fatal("unknown default_provider should fail")
	}
	if contains(err.Error(), "ErrLLMConfigMissing") || !contains(err.Error(), "anthropic") {
		t.Errorf("error should mention anthropic, got: %v", err)
	}
}

func TestValidateLLMGatewayConfig_ProviderDefaultModelMissing(t *testing.T) {
	cfg := validCfg()
	cfg.Providers["minimax"] = LLMProviderRuntimeConfig{
		Type:    "minimax",
		BaseURL: "https://api.example.com",
		// DefaultModel intentionally empty
	}
	err := ValidateLLMGatewayConfig(cfg)
	if err == nil {
		t.Fatal("missing providers.minimax.default_model should fail")
	}
	if !errors.Is(err, ErrLLMConfigMissing) {
		t.Errorf("expected ErrLLMConfigMissing, got %v", err)
	}
	if !contains(err.Error(), "providers.minimax.default_model") {
		t.Errorf("error should name providers.minimax.default_model, got: %v", err)
	}
}

func TestValidateLLMGatewayConfig_ErrorHintsUserConfig(t *testing.T) {
	err := ValidateLLMGatewayConfig(nil)
	if !contains(err.Error(), ".devrix/config.yaml") {
		t.Errorf("error should point to ~/.devrix/config.yaml, got: %v", err)
	}
	if !contains(err.Error(), "DEVRIX_LLM_DEFAULT_MODEL") {
		t.Errorf("error should name DEVRIX_LLM_DEFAULT_MODEL env var, got: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}