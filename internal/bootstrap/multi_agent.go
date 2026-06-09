package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentfactory "github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
)

// WireMultiAgent builds the Layer 4 agent factory with per-agent engine instances.
func WireMultiAgent(
	builder *ContextEngineBuilder,
	cfg *config.MultiAgentConfig,
	obsBridge *observability.Bridge,
) *multiagentfactory.AgentFactory {
	if cfg == nil {
		cfg = config.DefaultMultiAgentConfig()
	}
	deps := multiagent.AgentDeps{
		ObsBridge: obsBridge,
	}
	return multiagentfactory.NewAgentFactoryWithBuilder(deps, builder, cfg)
}
