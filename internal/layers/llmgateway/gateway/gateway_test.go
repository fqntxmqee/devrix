package gateway_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

// Covers: L5-LLM-20
func TestGateway_should_not_open_circuit_on_context_cancel(t *testing.T) {
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	cfg.CircuitBreaker.FailureThreshold = 1
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	reg := adapter.NewRegistry()
	_ = reg.Register(stubAdapter{provider: "deepseek", handler: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		ch := make(chan *llmgateway.AdapterChunk, 1)
		ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: "slow"}}
		return ch, nil
	}})
	cb := breaker.New(cfg.CircuitBreaker)
	gw := gateway.New(gateway.Deps{
		Config: cfg, Registry: reg, Breaker: cb, Retry: retry.NewExecutor(), Counter: counter,
	})

	parent, cancel := context.WithCancel(context.Background())
	ctx, stopDeadline := context.WithDeadline(parent, time.Now().Add(time.Hour))
	defer stopDeadline()
	ch, err := gw.Stream(ctx, &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	cancel()
	for range ch {
	}
	if cb.State("deepseek") != llmgateway.CircuitClosed {
		t.Fatalf("circuit state = %s, want closed", cb.State("deepseek"))
	}
}

// Covers: L5-LLM-21
func TestGateway_should_inject_provider_timeout_when_no_deadline(t *testing.T) {
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	cfg.Providers["deepseek"] = sharedconfig.LLMProviderRuntimeConfig{
		Type: "deepseek", DefaultModel: "deepseek-v4-flash", Timeout: 50 * time.Millisecond,
		Retry: sharedconfig.LLMRetryConfig{MaxAttempts: 1},
	}
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	reg := adapter.NewRegistry()
	_ = reg.Register(stubAdapter{provider: "deepseek", handler: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		ch := make(chan *llmgateway.AdapterChunk)
		go func() {
			time.Sleep(200 * time.Millisecond)
			close(ch)
		}()
		return ch, nil
	}})
	gw := gateway.New(gateway.Deps{
		Config: cfg, Registry: reg, Breaker: breaker.New(cfg.CircuitBreaker),
		Retry: retry.NewExecutor(), Counter: counter,
	})

	start := time.Now()
	ch, err := gw.Stream(context.Background(), &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range ch {
	}
	elapsed := time.Since(start)
	if elapsed > 150*time.Millisecond {
		t.Fatalf("expected timeout ~50ms, elapsed=%s", elapsed)
	}
}

// Covers: L5-LLM-21
func TestGateway_should_respect_existing_context_deadline(t *testing.T) {
	cfg := sharedconfig.DefaultLLMGatewayConfig()
	cfg.Providers["deepseek"] = sharedconfig.LLMProviderRuntimeConfig{
		Type: "deepseek", DefaultModel: "deepseek-v4-flash", Timeout: 200 * time.Millisecond,
		Retry: sharedconfig.LLMRetryConfig{MaxAttempts: 1},
	}
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	reg := adapter.NewRegistry()
	_ = reg.Register(stubAdapter{provider: "deepseek", handler: func(model string) (<-chan *llmgateway.AdapterChunk, error) {
		ch := make(chan *llmgateway.AdapterChunk)
		go func() {
			time.Sleep(300 * time.Millisecond)
			close(ch)
		}()
		return ch, nil
	}})
	gw := gateway.New(gateway.Deps{
		Config: cfg, Registry: reg, Breaker: breaker.New(cfg.CircuitBreaker),
		Retry: retry.NewExecutor(), Counter: counter,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	ch, err := gw.Stream(ctx, &llmgateway.Request{
		Model:    "deepseek-v4-flash",
		Messages: []types.Message{*types.NewMessage("1", "s", types.MessageRoleUser, "hi")},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	for range ch {
	}
	elapsed := time.Since(start)
	if elapsed > 120*time.Millisecond {
		t.Fatalf("expected ~80ms deadline, elapsed=%s", elapsed)
	}
}
