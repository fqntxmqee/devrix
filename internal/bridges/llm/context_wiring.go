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
	// DefaultModel 是 LLM 网关解析后的全局默认模型名（来自
	// llm_gateway.default_model）。供上层（如 ContextEngine）在
	// SessionContext.Model 为空时回填，用于"任务完成卡片显示模型"等
	// 仅展示用途，不参与路由（路由仍由 LLM 网关执行）。
	DefaultModel string
	// TierResolver resolves model tier aliases to concrete model names.
	TierResolver contextengine.ITierResolver
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
	slog.Info("llm gateway wired", "default_provider", llmCfg.DefaultProvider, "default_model", llmCfg.DefaultModel)
	return ContextLLMStack{
		Gateway:      wired.Bridge,
		RawGateway:   wired.Gateway,
		TokenCounter: wired.TokenCounter,
		DefaultModel: llmCfg.DefaultModel,
		TierResolver: wired.Bridge.(contextengine.ITierResolver),
	}
}
