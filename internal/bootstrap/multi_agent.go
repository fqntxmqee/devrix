package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentfactory "github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireMultiAgent builds the Layer 4 agent factory. Root inbound agents reuse
// sharedEngine for session-scoped context; forked workers still get isolated engines.
func WireMultiAgent(
	builder *ContextEngineBuilder,
	cfg *config.MultiAgentConfig,
	obsBridge *observability.Bridge,
	sharedEngine contracts.IEngine,
) *multiagentfactory.AgentFactory {
	if cfg == nil {
		cfg = config.DefaultMultiAgentConfig()
	}
	deps := multiagent.AgentDeps{
		Engine:    sharedEngine,
		ObsBridge: obsBridge,
	}
	return multiagentfactory.NewAgentFactoryWithBuilder(deps, builder, cfg)
}
