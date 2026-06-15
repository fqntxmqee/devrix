package turn

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// QueryLLMCallerDeps holds dependencies for the D2 query.LLMCaller adapter.
type QueryLLMCallerDeps struct {
	Gateway      llmgateway.IGateway
	TierResolver llmgateway.ITierResolver
	DefaultTier  string
}

// QueryLLMCaller implements shared/contracts.LLMCaller (D2 query-loop 拆面) by
// routing through the D3 gateway. This is the DM-020拆面出口 for the query loop.
//
// DSAFT: D7-S2-A07 (InvokeLLM) — D2→D3 拆面 adapter.
type QueryLLMCaller struct {
	gateway      llmgateway.IGateway
	tierResolver llmgateway.ITierResolver
	defaultTier  string
}

// NewQueryLLMCaller constructs a QueryLLMCaller adapter.
func NewQueryLLMCaller(deps QueryLLMCallerDeps) *QueryLLMCaller {
	return &QueryLLMCaller{
		gateway:      deps.Gateway,
		tierResolver: deps.TierResolver,
		defaultTier:  deps.DefaultTier,
	}
}

// Compile-time assertion: QueryLLMCaller satisfies contracts.LLMCaller.
var _ contracts.LLMCaller = (*QueryLLMCaller)(nil)

// Call performs one streaming LLM call and converts D3 chunks to contracts.LLMChunk.
func (q *QueryLLMCaller) Call(ctx context.Context, req contracts.LLMRequest) (<-chan contracts.LLMChunk, error) {
	if q.gateway == nil {
		return nil, fmt.Errorf("turn.QueryLLMCaller: gateway is nil")
	}

	model := q.defaultTier
	if model == "" {
		model = req.Model
	}
	if q.tierResolver != nil {
		resolved, err := q.tierResolver.ResolveTier(model)
		if err != nil {
			return nil, fmt.Errorf("tier resolve %q: %w", model, err)
		}
		model = resolved
	}

	tools := make([]llmgateway.ToolSchema, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = llmgateway.ToolSchema{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}

	ch, err := q.gateway.Stream(ctx, &llmgateway.Request{
		Model:        model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		Tools:        tools,
		Stream:       true,
	})
	if err != nil {
		return nil, err
	}

	out := make(chan contracts.LLMChunk, 8)
	go func() {
		defer close(out)
		for c := range ch {
			chunk := contracts.LLMChunk{
				Content:  c.Content,
				Thinking: c.Thinking,
				Done:     c.Done,
				Usage: contracts.TokenUsage{
					PromptTokens:     c.Usage.PromptTokens,
					CompletionTokens: c.Usage.CompletionTokens,
				},
			}
			if len(c.ToolCalls) > 0 {
				chunk.ToolCalls = make([]contracts.ToolCall, len(c.ToolCalls))
				for i, tc := range c.ToolCalls {
					chunk.ToolCalls[i] = contracts.ToolCall{
						ID:    tc.ID,
						Name:  tc.Name,
						Input: tc.Input,
					}
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}
