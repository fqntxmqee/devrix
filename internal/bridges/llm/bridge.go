package llmbridge

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/llmgateway"
)

// Bridge adapts the L3 gateway to contextengine.ILLMGateway.
type Bridge struct {
	gw llmgateway.IGateway
}

// New creates an L2-facing LLM bridge.
func New(gw llmgateway.IGateway) *Bridge {
	return &Bridge{gw: gw}
}

// ChatStream implements contextengine.ILLMGateway.
func (b *Bridge) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	if b.gw == nil {
		return nil, fmt.Errorf("llm gateway is nil")
	}
	internal := &llmgateway.Request{
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
		Tools:        mapTools(req.Tools),
		Stream:       true,
	}
	ch, err := b.gw.Stream(ctx, internal)
	if err != nil {
		return nil, err
	}

	out := make(chan contextengine.LLMChunk, 32)
	go func() {
		defer close(out)
		for chunk := range ch {
			select {
			case <-ctx.Done():
				return
			case out <- mapChunk(chunk):
			}
		}
	}()
	return out, nil
}

func mapTools(tools []contextengine.ToolSchema) []llmgateway.ToolSchema {
	if len(tools) == 0 {
		return nil
	}
	out := make([]llmgateway.ToolSchema, len(tools))
	for i, t := range tools {
		out[i] = llmgateway.ToolSchema{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}
	return out
}

func mapChunk(c llmgateway.Chunk) contextengine.LLMChunk {
	out := contextengine.LLMChunk{
		Content:   c.Content,
		Thinking:  c.Thinking,
		Done:      c.Done,
		Usage: contextengine.TokenUsage{
			PromptTokens:     c.Usage.PromptTokens,
			CompletionTokens: c.Usage.CompletionTokens,
			CacheReadTokens:  c.Usage.CacheReadTokens,
			ReasoningTokens:  c.Usage.ReasoningTokens,
		},
	}
	if len(c.ToolCalls) > 0 {
		out.ToolCalls = make([]contextengine.ToolCall, len(c.ToolCalls))
		for i, tc := range c.ToolCalls {
			out.ToolCalls[i] = contextengine.ToolCall{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
			}
		}
	}
	return out
}
