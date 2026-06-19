package turn

import (
	"context"
	"fmt"

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
func convertToolSchemas(in []ToolSchema) []llmgateway.ToolSchema {
	if len(in) == 0 {
		return nil
	}
	out := make([]llmgateway.ToolSchema, len(in))
	for i, ts := range in {
		out[i] = llmgateway.ToolSchema{
			Name:        ts.Name,
			Description: ts.Description,
		}
	}
	return out
}
