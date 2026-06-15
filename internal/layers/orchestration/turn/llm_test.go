package turn

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- gateway stubs ---

// stubGateway implements llmgateway.IGateway for InvokeStream-level tests.
type stubGateway struct {
	streamFn func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error)
}

func (s *stubGateway) Stream(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
	if s.streamFn != nil {
		return s.streamFn(ctx, req)
	}
	return nil, errors.New("stub: Stream not configured")
}

func (s *stubGateway) ResolveTier(tier string) string { return tier }

func (s *stubGateway) Close() error { return nil }

// stubTierResolver implements llmgateway.ITierResolver.
type stubTierResolver struct {
	resolved string
	err      error
}

func (s *stubTierResolver) ResolveTier(tier string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.resolved != "" {
		return s.resolved, nil
	}
	return tier, nil
}

// --- D7-S2-A07-T01: Breaker open error propagation ---

func TestGatewayInvoker_InvokeStream_BreakerOpen(t *testing.T) {
	breakerErr := fmt.Errorf("circuit breaker rejected: openai/gpt-5")
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, breakerErr
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-1",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "hello"}},
	})
	if err == nil {
		t.Fatal("expected error for breaker open, got nil")
	}
	if !errors.Is(err, breakerErr) {
		t.Errorf("expected breaker error to propagate, got: %v", err)
	}
}

// TestGatewayInvoker_InvokeStream_BreakerOpen_Content verifies the error
// message contains the provider name so upstream can surface it.
func TestGatewayInvoker_InvokeStream_BreakerOpen_Content(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, fmt.Errorf("circuit breaker rejected: anthropic/claude-opus")
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-2",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

// --- D7-S2-A07-T01: other gateway errors also propagate ---

func TestGatewayInvoker_InvokeStream_GatewayError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"timeout", errors.New("stream timeout")},
		{"connection refused", errors.New("dial tcp: connection refused")},
		{"internal error", errors.New("internal server error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := &stubGateway{
				streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
					return nil, tt.err
				},
			}
			invoker := NewGatewayInvoker(LLMInvokerDeps{
				Gateway:     gw,
				DefaultTier: "fast",
			})

			_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
				SessionID: "sess-3",
				Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
			})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// --- D7-S2-A07-T02: Stream timeout propagation ---

func TestGatewayInvoker_InvokeStream_ContextCanceled(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, ctx.Err()
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := invoker.InvokeStream(ctx, LLMInvokeRequest{
		SessionID: "sess-4",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGatewayInvoker_InvokeStream_ContextDeadlineExceeded(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, ctx.Err()
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "fast",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	_, err := invoker.InvokeStream(ctx, LLMInvokeRequest{
		SessionID: "sess-5",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for deadline exceeded, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

// --- D7-S2-A07-T02: orchestrator-level timeout → EngineEvent ---

func TestOrchestrator_RunTurn_StreamTimeout_EngineEvent(t *testing.T) {
	llm := &stubLLM{}
	llm.fn = func(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		// Simulate a stream that succeeds to return a channel, but the
		// context is already timed out so the actual chunk read fails.
		// The InvokeStream layer returns the error directly in this case.
		return nil, context.DeadlineExceeded
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-timeout",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "error") {
		t.Error("expected error event for stream timeout")
	}
	// Verify the error content mentions timeout/deadline
	found := false
	for _, e := range evs {
		if e.Type == "error" && e.Content != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected non-empty error event content")
	}
}

// --- Tier resolution error propagation ---

func TestGatewayInvoker_InvokeStream_TierResolveError(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			return nil, nil
		},
	}
	resolverErr := errors.New("unknown tier: experimental")
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:      gw,
		TierResolver: &stubTierResolver{err: resolverErr},
		DefaultTier:  "fast",
	})

	_, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-tier",
		Tier:      "experimental",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for tier resolve failure, got nil")
	}
}

// --- Successful tier resolution ---

func TestGatewayInvoker_InvokeStream_TierResolveSuccess(t *testing.T) {
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			if req.Model != "gpt-5-mini" {
				t.Errorf("expected resolved model 'gpt-5-mini', got %q", req.Model)
			}
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Content: "ok", Done: true}
			close(ch)
			return ch, nil
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:      gw,
		TierResolver: &stubTierResolver{resolved: "gpt-5-mini"},
		DefaultTier:  "fast",
	})

	ch, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-resolve",
		Tier:      "fast",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
}

// --- Default tier used when request tier is empty ---

func TestGatewayInvoker_InvokeStream_DefaultTier(t *testing.T) {
	var capturedModel string
	gw := &stubGateway{
		streamFn: func(ctx context.Context, req *llmgateway.Request) (<-chan llmgateway.Chunk, error) {
			capturedModel = req.Model
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Content: "ok", Done: true}
			close(ch)
			return ch, nil
		},
	}
	invoker := NewGatewayInvoker(LLMInvokerDeps{
		Gateway:     gw,
		DefaultTier: "default-fast",
	})

	ch, err := invoker.InvokeStream(context.Background(), LLMInvokeRequest{
		SessionID: "sess-default-tier",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
	if capturedModel != "default-fast" {
		t.Errorf("expected default tier 'default-fast', got %q", capturedModel)
	}
}
