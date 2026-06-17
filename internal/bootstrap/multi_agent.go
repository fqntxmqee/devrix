package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireMultiAgent builds the Layer 4 agent provision. Root inbound agents reuse
// sharedEngine for session-scoped context; forked workers still get isolated engines.
//
// DM-20260617-002 W7: 同时构造 freefork.DefaultForker 并 SetGlobalForker，
// 让 free_fork LLM tool (toolrunner 层) 通过 GlobalForker() 拿到注入。
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
	factory := multiagentprovision.NewAgentFactoryWithBuilder(deps, builder, cfg)
	// DefaultConfig 注入给 freefork.DefaultForker,子 agent 继承 session id
	freefork.SetGlobalForker(freefork.NewDefaultForker(freefork.ForkerDeps{
		Factory:       factory,
		DefaultConfig: multiagent.AgentConfig{Mode: multiagent.ModeDefault},
	}))
	slog.Info("freefork global forker injected")
	return factory
}
