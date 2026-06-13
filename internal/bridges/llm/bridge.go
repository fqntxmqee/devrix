package llmbridge

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

// Bridge adapts the L3 gateway to llmgateway.ILLMGateway for D2 consumers.
//
// DSAFT: D3-S1-A01-F03 (AdaptToContextEngine)
type Bridge struct {
	gw llmgateway.IGateway
}

// New creates an L2-facing LLM bridge.
func New(gw llmgateway.IGateway) *Bridge {
	return &Bridge{gw: gw}
}

// ResolveTier resolves a tier alias to a concrete model name.
// Implements llmgateway.ITierResolver.
func (b *Bridge) ResolveTier(tier string) (string, error) {
	if b.gw == nil {
		return "", fmt.Errorf("llm gateway is nil")
	}
	resolved := b.gw.ResolveTier(tier)
	if resolved == "" {
		return "", fmt.Errorf("tier %q resolved to empty model", tier)
	}
	return resolved, nil
}

// ChatStream implements llmgateway.ILLMGateway.
func (b *Bridge) ChatStream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if b.gw == nil {
		return nil, fmt.Errorf("llm gateway is nil")
	}
	if req == nil {
		req = &llmgateway.Request{}
	}
	internal := *req
	internal.Stream = true
	return b.gw.Stream(ctx, &internal)
}
