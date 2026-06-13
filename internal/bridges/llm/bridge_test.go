package llmbridge_test

import (
	"context"
	"testing"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubGW struct{}

func (stubGW) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{
		Content:   "hello",
		ToolCalls: []llmgateway.ToolCall{{ID: "1", Name: "bash", Input: "{}"}},
		Done:      true,
		Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 2},
	}
	close(ch)
	return ch, nil
}

func (stubGW) Close() error              { return nil }
func (stubGW) ResolveTier(tier string) string { return tier }

func TestBridge_should_map_chunks_to_context_engine(t *testing.T) {
	b := llmbridge.New(stubGW{})
	ch, err := b.ChatStream(context.Background(), &llmgateway.Request{
		Model: "deepseek-v4-flash",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "hi"),
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	var got llmgateway.Chunk
	for c := range ch {
		got = c
	}
	if got.Content != "hello" {
		t.Errorf("content: %q", got.Content)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "bash" {
		t.Errorf("tool calls: %+v", got.ToolCalls)
	}
	if got.Usage.PromptTokens != 1 {
		t.Errorf("usage: %+v", got.Usage)
	}
}
