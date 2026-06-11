package llmbridge

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ContextLLMStack holds LLM gateway wiring for the context engine.
type ContextLLMStack struct {
	Gateway      contextengine.ILLMGateway
	RawGateway   llmgateway.IGateway
	TokenCounter contracts.ITokenCounter
}

// WireContextLLM loads and wires the LLM stack; falls back to mock on error.
func WireContextLLM(configFile string, obsBridge *observability.Bridge) ContextLLMStack {
	llmCfg, err := config.LoadLLMGatewayConfig(configFile)
	if err != nil {
		slog.Warn("failed to load llm gateway config, using mock", "error", err)
		return ContextLLMStack{Gateway: &mockctx.LLMGateway{}, RawGateway: nil, TokenCounter: nil}
	}
	wired, err := WireFromConfig(llmCfg, obsBridge)
	if err != nil {
		slog.Warn("failed to wire llm gateway, using mock", "error", err)
		return ContextLLMStack{Gateway: &mockctx.LLMGateway{}, RawGateway: nil, TokenCounter: nil}
	}
	slog.Info("llm gateway wired", "default_provider", llmCfg.DefaultProvider)
	return ContextLLMStack{
		Gateway:      wired.Bridge,
		RawGateway:   wired.Gateway,
		TokenCounter: wired.TokenCounter,
	}
}
