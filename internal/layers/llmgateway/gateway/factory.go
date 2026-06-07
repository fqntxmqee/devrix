package gateway

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/breaker"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	"github.com/devrix/devrix/internal/layers/observability"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// NewFromConfig wires the full LLM gateway stack.
func NewFromConfig(cfg *sharedconfig.LLMGatewayConfig, obs *observability.Bridge) (*Gateway, error) {
	if cfg == nil {
		cfg = sharedconfig.DefaultLLMGatewayConfig()
	}
	counter, err := token.NewCounter()
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
		Breaker:  breaker.New(cfg.CircuitBreaker),
		Retry:    retry.NewExecutor(),
		Counter:  counter,
		Obs:      obs,
	}), nil
}
