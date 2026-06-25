package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

// LLMInvokerDeps holds the dependencies for the LLMInvoker.
type LLMInvokerDeps struct {
	Gateway      llmgateway.IGateway
	TierResolver llmgateway.ITierResolver
	DefaultTier  string
}

// GatewayInvoker implements LLMInvoker using the llmgateway bridge (D7-S2-A07).
type GatewayInvoker struct {
	gateway      llmgateway.IGateway
	tierResolver llmgateway.ITierResolver
	defaultTier  string
}

// NewGatewayInvoker creates a GatewayInvoker.
func NewGatewayInvoker(deps LLMInvokerDeps) *GatewayInvoker {
	return &GatewayInvoker{
		gateway:      deps.Gateway,
		tierResolver: deps.TierResolver,
		defaultTier:  deps.DefaultTier,
	}
}

// InvokeStream performs one D3 streaming call (D7-S2-A07).
//
// Overload / 5xx / rate-limit fallback is handled inside llmgateway
// (protect/retry); TD-QL-03 is satisfied at the gateway layer.
func (g *GatewayInvoker) InvokeStream(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	tier := req.Tier
	if tier == "" {
		tier = g.defaultTier
	}

	model := tier
	if g.tierResolver != nil {
		resolved, err := g.tierResolver.ResolveTier(tier)
		if err != nil {
			return nil, fmt.Errorf("tier resolve %q: %w", tier, err)
		}
		model = resolved
	}

	return g.gateway.Stream(ctx, &llmgateway.Request{
		Model:        model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		Tools:        convertToolSchemas(req.Tools),
		Stream:       true,
	})
}

// convertToolSchemas converts D7 ToolSchema to D3 llmgateway.ToolSchema.
//
// DM-20260621-007: the Parameters JSON schema MUST survive the boundary —
// the LLM gateway serialises Parameters back into the OpenAI-compatible
// tool definition as the function.parameters field. Without it, every
// tool call arrives at the gateway with parameters={} and the model
// defaults to empty arguments (e.g. bash invoked with `{}` instead of
// {"command": "ls"}; glob with no pattern). This regression existed
// silently since the D7 turn move (commit 6c2cf89, 2026-06-15) and was
// only exposed when the user observed repeated empty-args tool calls.
//
// Map[string]any (D7 side) is round-tripped back to a JSON string (D3
// side). A marshal failure (e.g. a non-serialisable channel in the
// schema) falls back to an empty string with a warning — better than
// dropping the schema silently, and the gateway already handles a
// blank Parameters field gracefully.
func convertToolSchemas(in []ToolSchema) []llmgateway.ToolSchema {
	if len(in) == 0 {
		return nil
	}
	out := make([]llmgateway.ToolSchema, len(in))
	for i, ts := range in {
		paramStr := ""
		if len(ts.Parameters) > 0 {
			bz, err := json.Marshal(ts.Parameters)
			if err != nil {
				slog.Warn("turn.convertToolSchemas: marshal Parameters failed",
					"tool", ts.Name, "error", err)
			} else {
				paramStr = string(bz)
			}
		}
		out[i] = llmgateway.ToolSchema{
			Name:        ts.Name,
			Description: ts.Description,
			Parameters:  paramStr,
		}
	}
	return out
}
