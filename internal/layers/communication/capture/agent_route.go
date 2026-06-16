package capture

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SetAgentFactory enables session leader provisioning for D4 delegate/fork.
// D1 ingress always routes through D7; the factory must not bypass orchestration entry.
func (g *CommunicationGateway) SetAgentFactory(factory multiagent.IAgentFactory) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agentFactory = factory
	if g.sessionAgents == nil {
		g.sessionAgents = make(map[string]multiagent.Agent)
	}
}

// SetAgentObserverFactory sets a factory function that creates additional agent observers.
func (g *CommunicationGateway) SetAgentObserverFactory(factory func(ctx context.Context, session *types.Session) multiagent.AgentObserver) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agentObserverFactory = factory
}

// RegisterSessionAgent binds an active agent to a session (tests and explicit wiring).
func (g *CommunicationGateway) RegisterSessionAgent(sessionID string, ag multiagent.Agent) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sessionAgents == nil {
		g.sessionAgents = make(map[string]multiagent.Agent)
	}
	g.sessionAgents[sessionID] = ag
}

// ResolveAgentPermission forwards the user decision to the session's active agent.
func (g *CommunicationGateway) ResolveAgentPermission(sessionID, toolName string, granted bool) {
	g.mu.RLock()
	ag := g.sessionAgents[sessionID]
	g.mu.RUnlock()
	if ag != nil {
		ag.ResolvePermission(toolName, granted)
	}
}

// SessionAgent returns the active agent for a session, if any.
func (g *CommunicationGateway) SessionAgent(sessionID string) multiagent.Agent {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sessionAgents[sessionID]
}

// ensureSessionLeaderAgent provisions a session leader for D4 delegate/fork without
// running Agent.Run on inbound. D7 owns ingress; the leader is the execution anchor.
func (g *CommunicationGateway) ensureSessionLeaderAgent(ctx context.Context, session *types.Session) error {
	if g.agentFactory == nil {
		return nil
	}
	if g.SessionAgent(session.SessionID) != nil {
		return nil
	}
	ag, err := g.agentFactory.Create(ctx, multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	if err != nil {
		return fmt.Errorf("agent create: %w", err)
	}
	g.RegisterSessionAgent(session.SessionID, ag)
	g.attachSessionAgent(ctx, session, ag)
	slog.Info("gateway: session leader provisioned",
		"session_id", session.SessionID,
		"agent_id", ag.ID(),
	)
	return nil
}

func (g *CommunicationGateway) attachSessionAgent(
	ctx context.Context,
	session *types.Session,
	ag multiagent.Agent,
) {
	ag.SetAgentObserver(&gatewayAgentObserver{gw: g, session: session})
	if g.agentObserverFactory != nil {
		obs := g.agentObserverFactory(ctx, session)
		if obs != nil {
			ag.SetAgentObserver(obs)
			slog.Info("gateway: orchestration observer attached",
				"session_id", session.SessionID,
			)
		} else {
			slog.Warn("gateway: agentObserverFactory returned nil",
				"session_id", session.SessionID,
			)
		}
	}
	ag.SetEngineEventSink(func(ev *contracts.EngineEvent) {
		if g.hasActiveProcess(session.SessionID) {
			return
		}
		g.handleEngineEvent(ctx, session, ev)
	})
}

func (g *CommunicationGateway) hasActiveProcess(sessionID string) bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.activeProcesses[sessionID]
	return ok
}

type gatewayAgentObserver struct {
	gw      *CommunicationGateway
	session *types.Session
}

func (o *gatewayAgentObserver) EmitAgentEvent(ev multiagent.AgentEvent) {
	if ev.EventType != "permission_required" {
		return
	}
	tool, _ := ev.Metadata["tool"].(string)
	if tool == "" {
		return
	}
	timeout := o.gw.config.Permission.DefaultTimeout
	req := types.NewPermissionRequest(
		generateMessageID(),
		o.session.SessionID,
		tool,
		types.RiskLevelCritical,
		timeout,
	)
	approved, _ := o.gw.RoutePermission(req)
	o.gw.ResolveAgentPermission(o.session.SessionID, tool, approved)
}
