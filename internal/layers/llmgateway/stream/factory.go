package stream

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	"github.com/devrix/devrix/internal/layers/observability"
)

// NewFromConfig wires the full LLM gateway stack.
func NewFromConfig(cfg *configure.LLMGatewayConfig, obs *observability.Bridge) (*Gateway, error) {
	if cfg == nil {
		cfg = configure.DefaultLLMGatewayConfig()
	}
	counter, err := budget.NewCounter()
	if err != nil {
		return nil, fmt.Errorf("token counter: %w", err)
	}
	reg := adapter.NewRegistry()
	if p, ok := cfg.Providers["deepseek"]; ok {
		if err := reg.Register(adapter.NewDeepSeekAdapter(p)); err != nil {
			return nil, err
		}
	}
	if p, ok := cfg.Providers["minimax"]; ok {
		if err := reg.Register(adapter.NewMiniMaxAdapter(p)); err != nil {
			return nil, err
		}
	}
	return New(Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  protect.New(cfg.CircuitBreaker),
		Retry:    protect.NewExecutor(),
		Counter:  counter,
		Obs:      obs,
	}), nil
}
