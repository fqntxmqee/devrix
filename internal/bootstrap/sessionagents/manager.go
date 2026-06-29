package sessionagents

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// PermissionRouter forwards tool permission prompts to the user (D1-S13-A04).
type PermissionRouter interface {
	RoutePermission(req *types.PermissionRequest) (bool, error)
	PermissionDefaultTimeout() time.Duration
}

// ActiveProcessChecker reports whether a session has an active D7 dispatch turn.
type ActiveProcessChecker interface {
	HasActiveProcess(sessionID string) bool
}

// OrphanEngineEventSink handles engine events from background agents when no
// inbound turn is active (D4 delegate / fork path).
type OrphanEngineEventSink func(ctx context.Context, session *types.Session, ev *contracts.EngineEvent)

// Manager holds per-session D4 leader agents. Lives in bootstrap so D1 capture
// does not import multiagent (DM-20260628-003 Phase 1).
type Manager struct {
	mu sync.RWMutex

	factory         multiagent.IAgentFactory
	observerFactory func(ctx context.Context, session *types.Session) multiagent.AgentObserver
	sessionAgents   map[string]multiagent.Agent

	permRouter   PermissionRouter
	activeCheck  ActiveProcessChecker
	orphanEvents OrphanEngineEventSink
}

// NewManager creates a session agent manager. factory may be nil (multi-agent disabled).
func NewManager(factory multiagent.IAgentFactory) *Manager {
	return &Manager{
		factory:       factory,
		sessionAgents: make(map[string]multiagent.Agent),
	}
}

// SetObserverFactory wires D6 guard or other agent observers at provision time.
func (m *Manager) SetObserverFactory(factory func(ctx context.Context, session *types.Session) multiagent.AgentObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observerFactory = factory
}

// SetPermissionRouter wires D1 permission gate for agent-initiated tool prompts.
func (m *Manager) SetPermissionRouter(r PermissionRouter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permRouter = r
}

// SetActiveProcessChecker avoids duplicate engine event handling during D7 turns.
func (m *Manager) SetActiveProcessChecker(c ActiveProcessChecker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeCheck = c
}

// SetOrphanEngineEventSink routes background agent events to D1 presenters.
func (m *Manager) SetOrphanEngineEventSink(sink OrphanEngineEventSink) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orphanEvents = sink
}

// EnsureSessionLeader provisions a session leader for D4 delegate/fork without
// running Agent.Run on inbound. D7 owns ingress; the leader is the execution anchor.
func (m *Manager) EnsureSessionLeader(ctx context.Context, session *types.Session) error {
	if m == nil || m.factory == nil || session == nil {
		return nil
	}
	if ag := m.SessionAgent(session.SessionID); ag != nil {
		return nil
	}
	ag, err := m.factory.Create(ctx, multiagent.AgentConfig{
		SessionID: session.SessionID,
		WorkDir:   session.WorkDir,
	}, session)
	if err != nil {
		return fmt.Errorf("agent create: %w", err)
	}
	m.RegisterSessionAgent(session.SessionID, ag)
	m.attachSessionAgent(ctx, session, ag)
	slog.Info("sessionagents: session leader provisioned",
		"session_id", session.SessionID,
		"agent_id", ag.ID(),
	)
	return nil
}

func (m *Manager) attachSessionAgent(
	ctx context.Context,
	session *types.Session,
	ag multiagent.Agent,
) {
	ag.SetAgentObserver(&managerAgentObserver{m: m, session: session})
	if m.observerFactory != nil {
		obs := m.observerFactory(ctx, session)
		if obs != nil {
			ag.SetAgentObserver(obs)
			slog.Info("sessionagents: orchestration observer attached",
				"session_id", session.SessionID,
			)
		} else {
			slog.Warn("sessionagents: observerFactory returned nil",
				"session_id", session.SessionID,
			)
		}
	}
	ag.SetEngineEventSink(func(ev *contracts.EngineEvent) {
		if m != nil && m.activeCheck != nil && m.activeCheck.HasActiveProcess(session.SessionID) {
			return
		}
		if m != nil && m.orphanEvents != nil {
			m.orphanEvents(ctx, session, ev)
		}
	})
}

// RegisterSessionAgent binds an active agent to a session (tests and D6 reroute).
func (m *Manager) RegisterSessionAgent(sessionID string, ag multiagent.Agent) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessionAgents == nil {
		m.sessionAgents = make(map[string]multiagent.Agent)
	}
	m.sessionAgents[sessionID] = ag
}

// SessionAgent returns the active agent for a session, if any.
func (m *Manager) SessionAgent(sessionID string) multiagent.Agent {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessionAgents[sessionID]
}

// Leader returns the session leader for delegate dispatch (D7-S4).
func (m *Manager) Leader(sessionID string) (multiagent.Agent, bool) {
	ag := m.SessionAgent(sessionID)
	return ag, ag != nil
}

// ResolveAgentPermission forwards the user decision to the session's active agent.
func (m *Manager) ResolveAgentPermission(sessionID, toolName string, granted bool) {
	ag := m.SessionAgent(sessionID)
	if ag != nil {
		ag.ResolvePermission(toolName, granted)
	}
}

type managerAgentObserver struct {
	m       *Manager
	session *types.Session
}

func (o *managerAgentObserver) EmitAgentEvent(ev multiagent.AgentEvent) {
	if ev.EventType != orchtypes.EventPermissionRequired {
		return
	}
	tool, _ := ev.Metadata["tool"].(string)
	if tool == "" || o.m == nil || o.session == nil {
		return
	}
	router := o.m.permRouter
	if router == nil {
		return
	}
	timeout := router.PermissionDefaultTimeout()
	req := types.NewPermissionRequest(
		kernel.NewMessageID(),
		o.session.SessionID,
		tool,
		types.RiskLevelCritical,
		timeout,
	)
	approved, _ := router.RoutePermission(req)
	o.m.ResolveAgentPermission(o.session.SessionID, tool, approved)
}
