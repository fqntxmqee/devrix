package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/breaker"
	"github.com/devrix/devrix/internal/layers/llmgateway/gateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubAdapter struct {
	provider string
	handler  func(model string) (<-chan *llmgateway.AdapterChunk, error)
}

func (s stubAdapter) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	return s.handler(req.Model)
}

func (s stubAdapter) Provider() string { return s.provider }

func testGateway(t *testing.T, handler func(model string) (<-chan *llmgateway.AdapterChunk, error)) *gateway.Gateway {
	t.Helper()
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	reg := adapter.NewRegistry()
	_ = reg.Register(stubAdapter{provider: "deepseek", handler: handler})
	return gateway.New(gateway.Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  breaker.New(cfg.CircuitBreaker),
		Retry:    retry.NewExecutor(),
		Counter:  counter,
	})
}

// Covers: L5-LLM-14
func TestGateway_should_stream_via_adapter(t *testing.T) {
	gw := testGateway(t, func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		ch := make(chan *llmgateway.AdapterChunk, 1)
		ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "ok", Done: true}}
		close(ch)
		return ch, nil
	})
	ch, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model: "deepseek-v4-flash",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "hi"),
		},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var text string
	for c := range ch {
		text += c.Content
	}
	if text != "ok" {
		t.Errorf("text: %q", text)
	}
}

// Covers: L5-LLM-10
func TestGateway_should_fallback_model_on_primary_failure(t *testing.T) {
	var models []string
	gw := testGateway(t, func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		models = append(models, model)
		if model == "deepseek-v4-flash" {
			return nil, errors.New("primary down")
		}
		ch := make(chan *llmgateway.AdapterChunk, 1)
		ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "fallback", Done: true}}
		close(ch)
		return ch, nil
	})
	ch, err := gw.Stream(context.Background(), &llmgateway.Request{Model: "deepseek-v4-flash"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range ch {
	}
	if len(models) < 2 || models[len(models)-1] != "deepseek-v4-pro" {
		t.Errorf("models: %v", models)
	}
}
