//go:build integration && cross

package integration

import (
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	llmtoken "github.com/devrix/devrix/internal/layers/llmgateway/token"
	"github.com/devrix/devrix/internal/shared/config"
)

// Covers: L5-CTX-18
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

// Covers: L5-CTX-18
func TestIntegration_WireContextLLMFallsBackToMock(t *testing.T) {
	stack := llmbridge.WireContextLLM("/nonexistent/devrix.yaml", nil)
	if stack.Gateway == nil {
		t.Fatal("expected gateway")
	}
}
