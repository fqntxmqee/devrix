package turn

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// fakeGateway implements llmgateway.IGateway for unit tests.
type fakeGateway struct {
	stream  func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error)
	resolve func(tier string) string
	close   func() error
}

func (f *fakeGateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if f.stream != nil {
		return f.stream(ctx, req)
	}
	ch := make(chan llmgateway.Chunk)
	close(ch)
	return ch, nil
}

func (f *fakeGateway) ResolveTier(tier string) string {
	if f.resolve != nil {
		return f.resolve(tier)
	}
	return tier
}

func (f *fakeGateway) Close() error {
	if f.close != nil {
		return f.close()
	}
	return nil
}

func TestQueryLLMCaller_NilGateway(t *testing.T) {
	c := NewQueryLLMCaller(QueryLLMCallerDeps{Gateway: nil})
	_, err := c.Call(context.Background(), contracts.LLMRequest{})
	if err == nil {
		t.Fatal("expected error for nil gateway, got nil")
	}
}

func TestQueryLLMCaller_TierResolution(t *testing.T) {
	captured := &llmgateway.Request{}
	res := &fakeTierResolver{resolve: func(tier string) (string, error) { return "resolved-" + tier, nil }}
	c := NewQueryLLMCaller(QueryLLMCallerDeps{
		Gateway: &fakeGateway{
			stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
				*captured = *req
				ch := make(chan llmgateway.Chunk, 1)
				ch <- llmgateway.Chunk{Done: true}
				close(ch)
				return ch, nil
			},
		},
		TierResolver: res,
		DefaultTier:  "tier-A",
	})
	out, err := c.Call(context.Background(), contracts.LLMRequest{
		SystemPrompt: "you are a test",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	for range out {
	}
	if captured.Model != "resolved-tier-A" {
		t.Fatalf("expected model 'resolved-tier-A', got %q", captured.Model)
	}
	if captured.SystemPrompt != "you are a test" {
		t.Fatalf("expected system prompt to round-trip, got %q", captured.SystemPrompt)
	}
}

func TestQueryLLMCaller_StreamConversion(t *testing.T) {
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			ch := make(chan llmgateway.Chunk, 3)
			ch <- llmgateway.Chunk{Content: "hello ", ToolCalls: []llmgateway.ToolCall{{ID: "tc1", Name: "t1", Input: "{}"}}}
			ch <- llmgateway.Chunk{Content: "world", Usage: llmgateway.TokenUsage{PromptTokens: 10, CompletionTokens: 5}}
			ch <- llmgateway.Chunk{Done: true, Usage: llmgateway.TokenUsage{TotalTokens: 15}}
			close(ch)
			return ch, nil
		},
	}
	c := NewQueryLLMCaller(QueryLLMCallerDeps{Gateway: gw, DefaultTier: "tier-B"})
	out, err := c.Call(context.Background(), contracts.LLMRequest{})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	var chunks []contracts.LLMChunk
	for c := range out {
		chunks = append(chunks, c)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Content != "hello " {
		t.Errorf("chunk[0] content = %q, want %q", chunks[0].Content, "hello ")
	}
	if len(chunks[0].ToolCalls) != 1 || chunks[0].ToolCalls[0].Name != "t1" {
		t.Errorf("chunk[0] tool call not converted: %+v", chunks[0].ToolCalls)
	}
	if chunks[1].Content != "world" {
		t.Errorf("chunk[1] content = %q, want %q", chunks[1].Content, "world")
	}
	if chunks[1].Usage.PromptTokens != 10 || chunks[1].Usage.CompletionTokens != 5 {
		t.Errorf("chunk[1] usage not converted: %+v", chunks[1].Usage)
	}
	if !chunks[2].Done {
		t.Errorf("chunk[2] Done = false, want true")
	}
}

func TestQueryLLMCaller_StreamError(t *testing.T) {
	gw := &fakeGateway{
		stream: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, errors.New("upstream down")
		},
	}
	c := NewQueryLLMCaller(QueryLLMCallerDeps{Gateway: gw, DefaultTier: "tier"})
	_, err := c.Call(context.Background(), contracts.LLMRequest{})
	if err == nil || err.Error() != "upstream down" {
		t.Fatalf("expected upstream down error, got %v", err)
	}
}

func TestQueryLLMCaller_TierResolverError(t *testing.T) {
	gw := &fakeGateway{
		resolve: func(tier string) string { return tier },
	}
	res := &errTierResolver{err: errors.New("bad tier")}
	c := NewQueryLLMCaller(QueryLLMCallerDeps{
		Gateway:      gw,
		TierResolver: res,
		DefaultTier:  "tier",
	})
	_, err := c.Call(context.Background(), contracts.LLMRequest{})
	if err == nil {
		t.Fatal("expected tier resolve error, got nil")
	}
}

type errTierResolver struct{ err error }

func (e *errTierResolver) ResolveTier(tier string) (string, error) {
	return "", e.err
}

type fakeTierResolver struct{ resolve func(tier string) (string, error) }

func (f *fakeTierResolver) ResolveTier(tier string) (string, error) {
	if f.resolve != nil {
		return f.resolve(tier)
	}
	return tier, nil
}
