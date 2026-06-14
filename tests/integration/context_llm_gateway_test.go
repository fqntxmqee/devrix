//go:build integration && cross

package integration

import (
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	llmtoken "github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/shared/config"
)

// T: D2-S0-A01-T02
func TestIntegration_ContextEngineUsesGatewayTokenCounterWhenWired(t *testing.T) {
	cfg := config.DefaultContextEngineConfig()
	cfg.TokenCounter.Source = config.TokenCounterSourceGateway

	counter, err := llmtoken.NewCounter()
	if err != nil {
		t.Skip("tiktoken not available:", err)
	}

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:          &mockctx.LLMGateway{},
		TokenCounter: counter,
		Config:       cfg,
	})
	if engine == nil {
		t.Fatal("expected engine")
	}
}

// T: D2-S0-A01-T02
func TestIntegration_WireContextLLMFallsBackToMock(t *testing.T) {
	stack, err := llmbridge.WireContextLLM("/nonexistent/devrix.yaml", nil)
	if err != nil {
		t.Fatalf("WireContextLLM: %v", err)
	}
	if stack.Gateway == nil {
		t.Fatal("expected gateway")
	}
}
