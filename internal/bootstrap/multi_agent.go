package bootstrap

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireMultiAgent builds the Layer 4 agent provision. Root inbound agents reuse
// sharedEngine for session-scoped context; forked workers still get isolated engines.
func WireMultiAgent(
	builder *ContextEngineBuilder,
	cfg *config.MultiAgentConfig,
	obsBridge *observability.Bridge,
	sharedEngine contracts.IEngine,
) *multiagentprovision.AgentFactory {
	if cfg == nil {
		cfg = config.DefaultMultiAgentConfig()
	}
	deps := multiagent.AgentDeps{
		Engine:    sharedEngine,
		ObsBridge: obsBridge,
	}
	return multiagentprovision.NewAgentFactoryWithBuilder(deps, builder, cfg)
}
