package bootstrap

import (
	multiagentfactory "github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/shared/config"
)

// WireMultiAgent builds the Layer 4 agent factory with per-agent engine instances.
func WireMultiAgent(
	builder *ContextEngineBuilder,
	cfg *config.MultiAgentConfig,
) *multiagentfactory.AgentFactory {
	if cfg == nil {
		cfg = config.DefaultMultiAgentConfig()
	}
	return multiagentfactory.NewAgentFactoryWithBuilder(builder, cfg)
}
