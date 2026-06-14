package gateway_test

import (
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway/gateway"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// T: D3-S2-A01-T02
func TestRouter_should_resolve_deepseek_model(t *testing.T) {
	r := gateway.NewRouter(sharedconfig.DefaultLLMGatewayConfig())
	provider, model, err := r.Resolve("deepseek-v4-flash")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if provider != "deepseek" || model != "deepseek-v4-flash" {
		t.Errorf("got provider=%s model=%s", provider, model)
	}
}

// T: D3-S2-A01-T02
func TestRouter_should_resolve_minimax_model(t *testing.T) {
	r := gateway.NewRouter(sharedconfig.DefaultLLMGatewayConfig())
	provider, model, err := r.Resolve("minimax-3")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if provider != "minimax" || model != "minimax-3" {
		t.Errorf("got provider=%s model=%s", provider, model)
	}
}

// T: D3-S2-A01-T02
func TestRouter_should_use_provider_default_when_model_empty(t *testing.T) {
	r := gateway.NewRouter(sharedconfig.DefaultLLMGatewayConfig())
	provider, model, err := r.Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if provider != "minimax" {
		t.Errorf("provider: got %s", provider)
	}
	if model != "MiniMax-M2.7-highspeed" {
		t.Errorf("model: got %s", model)
	}
}

// T: D3-S2-A01-T02
func TestRouter_should_return_error_for_unknown_model(t *testing.T) {
	r := gateway.NewRouter(sharedconfig.DefaultLLMGatewayConfig())
	_, _, err := r.Resolve("gpt-4o")
	if err == nil {
		t.Fatal("expected error")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) {
		t.Fatalf("expected LLMError, got %T", err)
	}
	if llmErr.Code != sharederrors.CodeLLMUnsupportedModel {
		t.Errorf("code: got %s", llmErr.Code)
	}
}

func TestRouter_should_use_global_default_model_when_set(t *testing.T) {
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	cfg.DefaultModel = "custom-model"
	cfg.ModelRouting["custom-*"] = "minimax"
	r := gateway.NewRouter(cfg)
	provider, model, err := r.Resolve("custom-model")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if provider != "minimax" || model != "custom-model" {
		t.Errorf("got provider=%s model=%s", provider, model)
	}
}
