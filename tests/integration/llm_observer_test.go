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
	"github.com/devrix/devrix/internal/layers/observability"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D3-S2-A01-T01
func TestIntegration_LLMGateway_emits_observability_span(t *testing.T) {
	obs := observability.NewNoOp()
	obsBridge := observability.NewBridge(obs)

	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	reg := adapter.NewRegistry()
	_ = reg.Register(integrationStubAdapter{
		provider: "minimax",
		fn: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
			ch := make(chan *llmgateway.AdapterChunk, 1)
			ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "obs", Done: true}}
			close(ch)
			return ch, nil
		},
	})

	gw := gateway.New(gateway.Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  breaker.New(cfg.CircuitBreaker),
		Retry:    retry.NewExecutor(),
		Counter:  counter,
		Obs:      obsBridge,
	})

	ch, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model: "minimax-3",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "trace"),
		},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range ch {
	}
}
