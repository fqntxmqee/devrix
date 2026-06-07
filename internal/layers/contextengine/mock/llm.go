package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/types"
)

// LLMGateway is a test double for ILLMGateway.
type LLMGateway struct {
	Response string
	Err      error
}

// ChatStream returns a single text response.
func (m *LLMGateway) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	ch := make(chan contextengine.LLMChunk, 2)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		text := m.Response
		if text == "" {
			text = "I can help you with that."
		}
		ch <- contextengine.LLMChunk{Content: text, Done: true, Usage: contextengine.TokenUsage{PromptTokens: 10, CompletionTokens: 5}}
	}()
	return ch, nil
}

// ToolRunner is a no-op tool runner for tests.
type ToolRunner struct {
	Output string
	Err    error
}

// Execute returns configured output.
func (t *ToolRunner) Execute(ctx context.Context, call contextengine.ToolCall) (*contextengine.ToolResult, error) {
	if t.Err != nil {
		return nil, t.Err
	}
	out := t.Output
	if out == "" {
		out = "ok"
	}
	return &contextengine.ToolResult{Output: out}, nil
}

// AllowAllPermission always approves.
type AllowAllPermission struct{}

func (AllowAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return true
}

// DenyAllPermission always denies.
type DenyAllPermission struct{}

func (DenyAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return false
}

// LLMGatewayWithTools returns a response that requests a tool call.
type LLMGatewayWithTools struct{}

// ChatStream emits a tool call then completes.
func (m *LLMGatewayWithTools) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	ch := make(chan contextengine.LLMChunk, 2)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		ch <- contextengine.LLMChunk{
			ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: "ls"}},
			Done:      true,
		}
	}()
	return ch, nil
}
