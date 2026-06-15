package bootstrap

import (
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
)

// WireTurnInvoker creates a turn.LLMInvoker from the LLM gateway stack.
//
// DSAFT: D7-S2-A07 InvokeLLM — the GatewayInvoker resolves model tiers via
// ITierResolver then delegates streaming to IGateway.Stream.
//
// DM-020 (D7 Turn 编排上移): this is the D7→D3 wiring point. The returned
// LLMInvoker is consumed by TurnOrchestrator (D7-S2-A06) in slice c.
func WireTurnInvoker(stack llmbridge.ContextLLMStack) *turn.GatewayInvoker {
	return turn.NewGatewayInvoker(turn.LLMInvokerDeps{
		Gateway:      stack.RawGateway,
		TierResolver: stack.TierResolver,
		DefaultTier:  stack.DefaultModel,
	})
}
