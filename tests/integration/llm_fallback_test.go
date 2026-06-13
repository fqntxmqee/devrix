//go:build integration && d3

package integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/breaker"
	"github.com/devrix/devrix/internal/layers/llmgateway/gateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D3-S4-A01-T02, D3-S4-A01-T03
func TestIntegration_LLMGateway_fallback_models(t *testing.T) {
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	reg := adapter.NewRegistry()

	var seen []string
	stub := integrationStubAdapter{
		provider: "deepseek",
		fn: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
			seen = append(seen, model)
			if model == cfg.Providers["deepseek"].DefaultModel {
				return nil, sharederrors.NewProviderUnavailableError(nil)
			}
			ch := make(chan *llmgateway.AdapterChunk, 1)
			ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "fb", Done: true}}
			close(ch)
			return ch, nil
		},
	}
	_ = reg.Register(stub)

	gw := gateway.New(gateway.Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  breaker.New(cfg.CircuitBreaker),
		Retry:    retry.NewExecutor(),
		Counter:  counter,
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
	if text != "fb" {
		t.Errorf("text: %q", text)
	}
	if len(seen) < 2 {
		t.Fatalf("expected fallback attempt, seen: %v", seen)
	}
}

type integrationStubAdapter struct {
	provider string
	fn       func(model string) (<-chan *llmgateway.AdapterChunk, error)
}

func (s integrationStubAdapter) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	return s.fn(req.Model)
}

func (s integrationStubAdapter) Provider() string { return s.provider }
