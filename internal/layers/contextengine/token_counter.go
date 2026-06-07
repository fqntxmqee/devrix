package contextengine

import (
	"github.com/devrix/devrix/internal/layers/contextengine/token"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// ResolveTokenCounter picks the configured token counter implementation.
func ResolveTokenCounter(cfg *config.ContextEngineConfig, gatewayCounter contracts.ITokenCounter) contracts.ITokenCounter {
	if cfg == nil || cfg.TokenCounter.Source == config.TokenCounterSourceHeuristic {
		return token.NewCounter()
	}
	if gatewayCounter != nil {
		return gatewayCounter
	}
	return token.NewCounter()
}
