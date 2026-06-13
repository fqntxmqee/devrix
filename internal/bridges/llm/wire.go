package llmbridge

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/gateway"
	"github.com/devrix/devrix/internal/layers/observability"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireResult holds the wired LLM stack.
type WireResult struct {
	Gateway      *gateway.Gateway
	Bridge       llmgateway.ILLMGateway
	TokenCounter contracts.ITokenCounter
}

// WireFromConfig builds gateway + L2 bridge from configuration.
func WireFromConfig(cfg *sharedconfig.LLMGatewayConfig, obs *observability.Bridge) (*WireResult, error) {
	gw, err := gateway.NewFromConfig(cfg, obs)
	if err != nil {
		return nil, fmt.Errorf("llm gateway: %w", err)
	}
	return &WireResult{
		Gateway:      gw,
		Bridge:       New(gw),
		TokenCounter: gw.TokenCounter(),
	}, nil
}
