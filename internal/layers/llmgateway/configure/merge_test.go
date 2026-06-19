package configure

import (
	"testing"
	"time"
)

func TestMergeLLMGatewayFileConfig_NilPaths(t *testing.T) {
	base := &LLMGatewayFileConfig{DefaultProvider: "minimax", DefaultModel: "M-x"}
	if got := MergeLLMGatewayFileConfig(base, nil); got != base {
		t.Fatalf("override=nil should return base pointer unchanged")
	}
	if got := MergeLLMGatewayFileConfig(nil, base); got == nil || got.DefaultProvider != "minimax" {
		t.Fatalf("base=nil should clone override, got %+v", got)
	}
}

func TestMergeLLMGatewayFileConfig_ScalarOverrideWins(t *testing.T) {
	base := &LLMGatewayFileConfig{
		DefaultProvider: "minimax",
		DefaultModel:    "M-2.7",
		DefaultTier:     "default",
	}
	override := &LLMGatewayFileConfig{
		DefaultProvider: "deepseek",
		DefaultModel:    "M-3",
	}
	got := MergeLLMGatewayFileConfig(base, override)
	if got.DefaultProvider != "deepseek" {
		t.Errorf("DefaultProvider = %q, want deepseek", got.DefaultProvider)
	}
	if got.DefaultModel != "M-3" {
		t.Errorf("DefaultModel = %q, want M-3", got.DefaultModel)
	}
	if got.DefaultTier != "default" {
		t.Errorf("DefaultTier = %q, want default (preserved from base)", got.DefaultTier)
	}
}

func TestMergeLLMGatewayFileConfig_EmptyOverridePreservesBase(t *testing.T) {
	base := &LLMGatewayFileConfig{
		DefaultProvider: "minimax",
		DefaultModel:    "M-2.7",
	}
	override := &LLMGatewayFileConfig{
		// All scalars empty — base should be preserved.
		ModelTiers: map[string]string{"fast": "M-fast"},
	}
	got := MergeLLMGatewayFileConfig(base, override)
	if got.DefaultProvider != "minimax" || got.DefaultModel != "M-2.7" {
		t.Errorf("empty override should not clobber scalars, got %+v", got)
	}
	if got.ModelTiers["fast"] != "M-fast" {
		t.Errorf("override ModelTiers should be merged in, got %+v", got.ModelTiers)
	}
}

func TestMergeLLMGatewayFileConfig_ModelTiersDeepMerge(t *testing.T) {
	base := &LLMGatewayFileConfig{
		ModelTiers: map[string]string{
			"fast":     "M-fast-old",
			"default":  "M-default",
			"powerful": "M-powerful",
		},
	}
	override := &LLMGatewayFileConfig{
		ModelTiers: map[string]string{
			"fast": "M-fast-new", // override
			// default + powerful preserved
		},
	}
	got := MergeLLMGatewayFileConfig(base, override)
	if got.ModelTiers["fast"] != "M-fast-new" {
		t.Errorf("fast = %q, want M-fast-new", got.ModelTiers["fast"])
	}
	if got.ModelTiers["default"] != "M-default" {
		t.Errorf("default = %q, want M-default (preserved)", got.ModelTiers["default"])
	}
	if got.ModelTiers["powerful"] != "M-powerful" {
		t.Errorf("powerful = %q, want M-powerful (preserved)", got.ModelTiers["powerful"])
	}
}

func TestMergeLLMGatewayFileConfig_ProviderPartialOverride(t *testing.T) {
	base := &LLMGatewayFileConfig{
		Providers: map[string]LLMProviderConfig{
			"minimax": {
				Type:         "minimax",
				BaseURL:      "https://api.minimaxi.com/v1",
				APIKeyEnv:    "MINIMAX_API_KEY",
				DefaultModel: "M-2.7",
				Timeout:      60 * time.Second,
				MaxTokens:    8192,
				Temperature:  0.7,
				Retry: LLMRetryFileConfig{
					MaxAttempts:  3,
					InitialDelay: time.Second,
					MaxDelay:     10 * time.Second,
					Backoff:      2.0,
				},
			},
		},
	}
	override := &LLMGatewayFileConfig{
		Providers: map[string]LLMProviderConfig{
			"minimax": {
				// User only overrides default_model — everything else should remain from base.
				DefaultModel: "M-3",
			},
		},
	}
	got := MergeLLMGatewayFileConfig(base, override)
	p := got.Providers["minimax"]
	if p.DefaultModel != "M-3" {
		t.Errorf("DefaultModel = %q, want M-3", p.DefaultModel)
	}
	if p.Type != "minimax" || p.BaseURL != "https://api.minimaxi.com/v1" || p.APIKeyEnv != "MINIMAX_API_KEY" {
		t.Errorf("infrastructure fields should be preserved, got %+v", p)
	}
	if p.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s (preserved)", p.Timeout)
	}
	if p.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want 8192 (preserved)", p.MaxTokens)
	}
	if p.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7 (preserved)", p.Temperature)
	}
	if p.Retry.MaxAttempts != 3 {
		t.Errorf("Retry.MaxAttempts = %d, want 3 (preserved)", p.Retry.MaxAttempts)
	}
}

func TestMergeLLMGatewayFileConfig_NewProviderAddedByUser(t *testing.T) {
	base := &LLMGatewayFileConfig{
		Providers: map[string]LLMProviderConfig{
			"minimax": {Type: "minimax"},
		},
	}
	override := &LLMGatewayFileConfig{
		Providers: map[string]LLMProviderConfig{
			"anthropic": {Type: "anthropic", BaseURL: "https://api.anthropic.com"},
		},
	}
	got := MergeLLMGatewayFileConfig(base, override)
	if got.Providers["minimax"].Type != "minimax" {
		t.Errorf("base provider should be preserved")
	}
	if got.Providers["anthropic"].Type != "anthropic" {
		t.Errorf("override provider should be added, got %+v", got.Providers["anthropic"])
	}
}

func TestMergeLLMGatewayFileConfig_HeadersDeepMerge(t *testing.T) {
	base := &LLMGatewayFileConfig{
		Providers: map[string]LLMProviderConfig{
			"minimax": {
				Headers: map[string]string{"X-Source": "base", "X-Region": "cn"},
			},
		},
	}
	override := &LLMGatewayFileConfig{
		Providers: map[string]LLMProviderConfig{
			"minimax": {
				Headers: map[string]string{"X-Source": "user", "X-Trace": "1"},
			},
		},
	}
	got := MergeLLMGatewayFileConfig(base, override)
	h := got.Providers["minimax"].Headers
	if h["X-Source"] != "user" {
		t.Errorf("X-Source = %q, want user (override)", h["X-Source"])
	}
	if h["X-Region"] != "cn" {
		t.Errorf("X-Region = %q, want cn (preserved)", h["X-Region"])
	}
	if h["X-Trace"] != "1" {
		t.Errorf("X-Trace = %q, want 1 (new from override)", h["X-Trace"])
	}
}

func TestMergeLLMGatewayFileConfig_CircuitBreakerPartial(t *testing.T) {
	base := &LLMGatewayFileConfig{
		CircuitBreaker: LLMCircuitBreakerFileConfig{
			FailureThreshold:  5,
			SuccessThreshold:  2,
			OpenDuration:      30 * time.Second,
			HalfOpenMaxProbes: 1,
			Scope:             "provider",
		},
	}
	override := &LLMGatewayFileConfig{
		CircuitBreaker: LLMCircuitBreakerFileConfig{
			FailureThreshold: 10, // only this overridden
		},
	}
	got := MergeLLMGatewayFileConfig(base, override)
	cb := got.CircuitBreaker
	if cb.FailureThreshold != 10 {
		t.Errorf("FailureThreshold = %d, want 10", cb.FailureThreshold)
	}
	if cb.SuccessThreshold != 2 {
		t.Errorf("SuccessThreshold = %d, want 2 (preserved)", cb.SuccessThreshold)
	}
	if cb.OpenDuration != 30*time.Second {
		t.Errorf("OpenDuration = %v, want 30s (preserved)", cb.OpenDuration)
	}
	if cb.Scope != "provider" {
		t.Errorf("Scope = %q, want provider (preserved)", cb.Scope)
	}
}

func TestMergeLLMGatewayFileConfig_DoesNotMutateInputs(t *testing.T) {
	baseTiers := map[string]string{"fast": "M-fast-base"}
	overrideTiers := map[string]string{"fast": "M-fast-user", "default": "M-default-user"}
	base := &LLMGatewayFileConfig{ModelTiers: baseTiers}
	override := &LLMGatewayFileConfig{ModelTiers: overrideTiers}

	got := MergeLLMGatewayFileConfig(base, override)
	got.ModelTiers["fast"] = "MUTATED"

	if baseTiers["fast"] != "M-fast-base" {
		t.Errorf("base mutated: %q", baseTiers["fast"])
	}
	if overrideTiers["fast"] != "M-fast-user" {
		t.Errorf("override mutated: %q", overrideTiers["fast"])
	}
}