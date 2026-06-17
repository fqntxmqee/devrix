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

// WireAgentFactory builds the Layer 4 agent provision. Root inbound agents reuse
// sharedEngine for session-scoped context; forked workers still get isolated engines.
//
// DM-20260617-008 W5: the factory is returned directly (no longer side-effects a
// process-wide singleton). Callers that need a freefork.Forker should call
// WireDefaultForker with the returned factory.
func WireAgentFactory(
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
	return factory
}

// WireDefaultForker constructs a freefork.DefaultForker bound to factory.
//
// DM-20260617-008 W5: replaces the legacy freefork.SetGlobalForker write inside
// the old WireMultiAgent. The forker is returned and the caller (main.go) is
// responsible for plumbing it into the engine's surface list (BuildSurfaces
// via freeforkGlobalFunc closure).
func WireDefaultForker(
	factory multiagent.IAgentFactory,
) freefork.Forker {
	return freefork.NewDefaultForker(freefork.ForkerDeps{
		Factory:       factory,
		DefaultConfig: multiagent.AgentConfig{Mode: multiagent.ModeDefault},
	})
}

// WireMultiAgent composes WireAgentFactory + WireDefaultForker in the legacy
// order so older call sites (and tests) keep working without rewriting.
//
// DM-20260617-008 W5: no longer writes to freefork.SetGlobalForker. Production
// main.go should call WireAgentFactory + WireDefaultForker separately so the
// forker can be injected into the main engine's surface list at engine build
// time (before main engine is built).
func WireMultiAgent(
	builder *ContextEngineBuilder,
	cfg *config.MultiAgentConfig,
	obsBridge *observability.Bridge,
	sharedEngine contracts.IEngine,
) *multiagentprovision.AgentFactory {
	factory := WireAgentFactory(builder, cfg, obsBridge, sharedEngine)
	forker := WireDefaultForker(factory)
	_ = forker
	slog.Info("agent factory + forker wired (legacy WireMultiAgent path)")
	return factory
}
