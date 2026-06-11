package factory

import (
	"context"
	"fmt"
	"sync"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	"github.com/devrix/devrix/internal/layers/multiagent/collaboration"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// EngineBuilder constructs a per-agent IEngine with the agent permission gate.
type EngineBuilder interface {
	Build(perm multiagent.PermissionGate) contracts.IEngine
}

// AgentFactory creates multi-agent instances.
type AgentFactory struct {
	deps          multiagent.AgentDeps
	cfg           *sharedconfig.MultiAgentConfig
	builder       EngineBuilder
	sessionCounts map[string]int
	mu            sync.Mutex
}

var _ multiagent.IAgentFactory = (*AgentFactory)(nil)

// NewAgentFactory constructs a factory with layer defaults.
func NewAgentFactory(deps multiagent.AgentDeps, cfg *sharedconfig.MultiAgentConfig) *AgentFactory {
	if cfg == nil {
		cfg = sharedconfig.DefaultMultiAgentConfig()
	}
	return &AgentFactory{
		deps:          deps,
		cfg:           cfg,
		sessionCounts: make(map[string]int),
	}
}

// NewAgentFactoryWithBuilder wires per-agent engines via the supplied builder.
func NewAgentFactoryWithBuilder(
	deps multiagent.AgentDeps,
	builder EngineBuilder,
	cfg *sharedconfig.MultiAgentConfig,
) *AgentFactory {
	f := NewAgentFactory(deps, cfg)
	f.builder = builder
	return f
}

// Create implements multiagent.IAgentFactory.
func (f *AgentFactory) Create(
	ctx context.Context,
	cfg multiagent.AgentConfig,
	session *types.Session,
) (multiagent.Agent, error) {
	ctx, createSpan := f.startCreateSpan(ctx, cfg)

	if err := ctx.Err(); err != nil {
		return nil, sharederrors.NewAgentContextCancelledError(cfg.SessionID)
	}
	if err := validateConfig(cfg, f.cfg); err != nil {
		if createSpan != nil {
			createSpan.End()
		}
		return nil, err
	}
	if session == nil {
		if createSpan != nil {
			createSpan.End()
		}
		return nil, sharederrors.NewAgentInvalidConfigError("session is nil")
	}
	if err := f.reserveSessionSlot(cfg.SessionID); err != nil {
		if createSpan != nil {
			createSpan.End()
		}
		return nil, err
	}
	resolved := resolveConfig(cfg, f.cfg)
	impl := agent.New(resolved, session, f.deps, f)
	switch {
	case cfg.ParentID != "" && f.builder != nil:
		// Forked workers need an isolated engine with the agent permission gate.
		inner := f.builder.Build(impl.PermissionGate())
		impl.SetEngine(agent.NewWorkerEngine(inner, resolved, impl.ID()))
	case f.deps.Engine != nil:
		// Root session agents share the gateway context engine so conversation
		// history accumulates across inbound messages in the same session.
		impl.SetEngine(f.deps.Engine)
	case f.builder != nil:
		inner := f.builder.Build(impl.PermissionGate())
		impl.SetEngine(agent.NewWorkerEngine(inner, resolved, impl.ID()))
	default:
		impl.SetEngine(&agent.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}})
	}
	if createSpan != nil {
		createSpan.End()
	}
	return impl, nil
}

func (f *AgentFactory) startCreateSpan(ctx context.Context, cfg multiagent.AgentConfig) (context.Context, tracer.Span) {
	if f.deps.ObsBridge == nil || f.deps.ObsBridge.Tracer() == nil {
		return ctx, nil
	}
	ctx, span := f.deps.ObsBridge.Tracer().Start(ctx, telemetry.OpGatewayAgentCreate,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayAgentCreate,
			tracer.Attribute{Key: "session.id", Value: cfg.SessionID},
			tracer.Attribute{Key: "agent.mode", Value: string(cfg.Mode)},
		)...),
	)
	return ctx, span
}

// ReleaseSession decrements the session agent quota when an agent terminates.
func (f *AgentFactory) ReleaseSession(sessionID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessionCounts[sessionID] > 0 {
		f.sessionCounts[sessionID]--
	}
}

func (f *AgentFactory) reserveSessionSlot(sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	current := f.sessionCounts[sessionID]
	if current >= f.cfg.MaxTotalAgents {
		return sharederrors.NewAgentMaxTotalError(current, f.cfg.MaxTotalAgents)
	}
	f.sessionCounts[sessionID]++
	return nil
}

func validateConfig(cfg multiagent.AgentConfig, defaults *sharedconfig.MultiAgentConfig) error {
	if cfg.SessionID == "" {
		return sharederrors.NewAgentInvalidConfigError("session_id is required")
	}
	if cfg.WorkDir == "" {
		return sharederrors.NewAgentInvalidConfigError("work_dir is required")
	}
	if cfg.Mode != "" {
		if err := collaboration.ValidateMode(cfg.Mode); err != nil {
			return sharederrors.NewAgentInvalidConfigError(err.Error())
		}
	}
	maxChildren := cfg.MaxChildren
	if maxChildren <= 0 {
		maxChildren = defaults.MaxChildren
	}
	if maxChildren < 0 || maxChildren > 10 {
		return sharederrors.NewAgentInvalidConfigError(
			fmt.Sprintf("max_children must be 0..10, got %d", maxChildren),
		)
	}
	if cfg.MaxIter < 0 {
		return sharederrors.NewAgentInvalidConfigError("max_iter must be >= 0")
	}
	if cfg.Timeout < 0 {
		return sharederrors.NewAgentInvalidConfigError("timeout must be >= 0")
	}
	return nil
}

func resolveConfig(cfg multiagent.AgentConfig, defaults *sharedconfig.MultiAgentConfig) multiagent.AgentConfig {
	out := cfg
	if out.Mode == "" {
		out.Mode = multiagent.CollaborationMode(defaults.DefaultMode)
		if out.Mode == "" {
			out.Mode = multiagent.ModeDefault
		}
	}
	if out.MaxChildren <= 0 {
		out.MaxChildren = defaults.MaxChildren
	}
	if out.MaxIter <= 0 {
		out.MaxIter = defaults.DefaultMaxIter
	}
	if out.Timeout <= 0 {
		out.Timeout = defaults.DefaultTimeout
	}
	if out.PermissionTimeout <= 0 {
		out.PermissionTimeout = defaults.PermissionTimeout
	}
	if out.SystemPrompt != "" {
		out.SystemPrompt = collaboration.BuildPromptForMode(out.Mode, out.SystemPrompt)
	}
	return out
}
