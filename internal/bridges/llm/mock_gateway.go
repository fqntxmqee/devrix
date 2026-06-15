package llmbridge

import (
	"context"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

// MockGateway is an in-process test double for llmgateway.ILLMGateway.
type MockGateway struct {
	Response string
	Err      error
}

// ChatStream returns a single text response.
func (m *MockGateway) ChatStream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	ch := make(chan llmgateway.Chunk, 2)
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
		ch <- llmgateway.Chunk{
			Content: text,
			Done:    true,
			Usage:   llmgateway.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
		}
	}()
	return ch, nil
}

// MockGatewayWithTools emits a tool call then completes.
type MockGatewayWithTools struct{}

// ChatStream implements llmgateway streaming for tests that need a bash tool call.
func (m *MockGatewayWithTools) ChatStream(ctx context.Context, _ *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk, 2)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{ID: "tc1", Name: "bash", Input: "ls"}},
			Done:      true,
		}
	}()
	return ch, nil
}
