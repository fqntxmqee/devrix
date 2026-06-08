package gateway

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SetAgentFactory enables Layer 4 agent routing for inbound messages.
func (g *CommunicationGateway) SetAgentFactory(factory multiagent.IAgentFactory) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.agentFactory = factory
	if g.sessionAgents == nil {
		g.sessionAgents = make(map[string]multiagent.Agent)
	}
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

func (g *CommunicationGateway) routeInboundViaAgent(
	ctx context.Context,
	msg *types.InboundMessage,
	session *types.Session,
	endSpan func(),
) error {
	if g.agentFactory == nil {
		return fmt.Errorf("agent factory not configured")
	}

	ag, err := g.agentFactory.Create(ctx, multiagent.AgentConfig{
		SessionID:    session.SessionID,
		WorkDir:      session.WorkDir,
		InitialInput: msg.Content,
	}, session)
	if err != nil {
		return fmt.Errorf("agent create: %w", err)
	}

	g.mu.Lock()
	g.sessionAgents[session.SessionID] = ag
	g.mu.Unlock()

	ag.SetAgentObserver(&gatewayAgentObserver{gw: g, session: session})
	ag.SetEngineEventSink(func(ev *contracts.EngineEvent) {
		g.handleEngineEvent(ctx, session, ev)
	})

	processCtx, cancel := context.WithCancel(ctx)
	g.registerProcess(session.SessionID, cancel)
	go func() {
		defer endSpan()
		defer cancel()
		defer g.clearSessionAgent(session.SessionID)
		if _, runErr := ag.Run(processCtx); runErr != nil {
			g.eventHandler.OnError(runErr, session.SessionID)
		}
		g.unregisterProcess(session.SessionID)
	}()
	return nil
}

func (g *CommunicationGateway) clearSessionAgent(sessionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.sessionAgents, sessionID)
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
