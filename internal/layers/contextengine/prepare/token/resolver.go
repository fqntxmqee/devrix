package token

import (
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ResolveCounter picks the configured token counter implementation.
func ResolveCounter(cfg *config.ContextEngineConfig, gatewayCounter contracts.ITokenCounter) contracts.ITokenCounter {
	if cfg == nil || cfg.TokenCounter.Source == config.TokenCounterSourceHeuristic {
		return NewCounter()
	}
	if gatewayCounter != nil {
		return gatewayCounter
	}
	return NewCounter()
}
