package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// EventHandler defines the interface for handling gateway events
type EventHandler interface {
	OnMessage(msg *types.OutboundMessage)
	OnPermissionRequest(req *types.PermissionRequest) bool
	OnError(err error, sessionID string)
	OnStatus(sessionID string, state types.SessionState)
}

// IContextEngine defines the interface for the context engine
type IContextEngine interface {
	Process(ctx context.Context, session *types.Session, message string) <-chan *EngineEvent
}

// EngineEvent represents an event from the context engine
type EngineEvent struct {
	Type      string            // thinking | text | tool_call | tool_result | permission | status | complete | error
	Content   string
	ToolName  string
	ToolInput string
	SessionID string
	Metadata  map[string]string
}

// CommunicationGateway routes messages between adapters and the context engine
type CommunicationGateway struct {
	sessionStore  SessionStore
	eventHandler EventHandler
	contextEngine IContextEngine
	permissionMgr *PermissionManager
	config       *config.CommunicationConfig
	obsBridge    *observability.Bridge

	mu              sync.RWMutex
	sessions        map[string]*types.Session
	activeProcesses map[string]context.CancelFunc

	// metrics
	metricInboundMsgs    metrics.Counter
	metricOutboundMsgs   metrics.Counter
	metricSessionsTotal  metrics.Counter
	metricActiveSessions metrics.Gauge
}

// NewCommunicationGateway creates a new CommunicationGateway
func NewCommunicationGateway(
	sessionStore SessionStore,
	eventHandler EventHandler,
	contextEngine IContextEngine,
	permissionMgr *PermissionManager,
	cfg *config.CommunicationConfig,
) *CommunicationGateway {
	gw := &CommunicationGateway{
		sessionStore:  sessionStore,
		eventHandler: eventHandler,
		contextEngine: contextEngine,
		permissionMgr: permissionMgr,
		config:       cfg,
		sessions:        make(map[string]*types.Session),
		activeProcesses: make(map[string]context.CancelFunc),
	}
	return gw
}

// Stop implements commands.Stopper — cancels the active context engine Process.
func (g *CommunicationGateway) Stop(sessionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if cancel, ok := g.activeProcesses[sessionID]; ok {
		cancel()
		delete(g.activeProcesses, sessionID)
	}
	return nil
}

func (g *CommunicationGateway) registerProcess(sessionID string, cancel context.CancelFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if prev, ok := g.activeProcesses[sessionID]; ok {
		prev()
	}
	g.activeProcesses[sessionID] = cancel
}

func (g *CommunicationGateway) unregisterProcess(sessionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.activeProcesses, sessionID)
}

// SetObservability wires tracing/metrics into the gateway.
func (g *CommunicationGateway) SetObservability(obs *observability.Observability) {
	if obs == nil {
		g.obsBridge = nil
		return
	}
	g.obsBridge = observability.NewBridge(obs)
	g.initMetrics(obs)
}

func (g *CommunicationGateway) initMetrics(obs *observability.Observability) {
	if g.obsBridge == nil || g.obsBridge.Meter() == nil {
		return
	}
	m := g.obsBridge.Meter()
	g.metricInboundMsgs, _ = m.Int64Counter("gateway_inbound_messages", metrics.WithLabels(metrics.LabelMap{
		"adapter": "all",
	}))
	g.metricOutboundMsgs, _ = m.Int64Counter("gateway_outbound_messages", metrics.WithLabels(metrics.LabelMap{
		"event_type": "all",
	}))
	g.metricSessionsTotal, _ = m.Int64Counter("gateway_sessions_total", metrics.WithLabels(metrics.LabelMap{
		"adapter": "all",
	}))
	sessionBridge := observability.NewSessionBridge(obs)
	if sessionBridge != nil {
		g.metricActiveSessions, _ = sessionBridge.ActiveSessions("all")
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up expired sessions
func (g *CommunicationGateway) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.cleanupExpiredSessions()
			}
		}
	}()
}

// cleanupExpiredSessions removes expired sessions from the in-memory cache
func (g *CommunicationGateway) cleanupExpiredSessions() {
	g.mu.Lock()
	defer g.mu.Unlock()

	timeout := g.config.Session.IdleTimeout
	now := time.Now()

	for sessionID, session := range g.sessions {
		if now.Sub(session.LastMessageAt) > timeout {
			slog.Debug("cleaning up expired session", "sessionID", sessionID)
			delete(g.sessions, sessionID)
		}
	}
}

// RouteInbound processes an inbound message from an adapter
func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	ctx, endSpan := g.startInboundSpan(ctx, msg)

	slog.Info("gateway: RouteInbound called", "sessionID", msg.SessionID, "content", msg.Content, "chatID", msg.ChatID)

	// Record inbound metric
	if g.metricInboundMsgs != nil {
		g.metricInboundMsgs.Inc()
	}

	// Validate message
	if msg.Content == "" {
		return errors.NewMessageEmptyError()
	}

	if len(msg.Content) > 64000 {
		return errors.WithCode("COMM_MESSAGE_TOO_LONG", "message too long", errors.ErrMessageTooLong)
	}

	// Get or create session
	session, err := g.getOrCreateSession(msg)
	if err != nil {
		return fmt.Errorf("failed to get or create session: %w", err)
	}

	// Check idle timeout
	if session.IsIdle(g.config.Session.IdleTimeout) {
		if err := g.ExpireSession(session.SessionID); err != nil {
			slog.Warn("failed to expire idle session", "sessionID", session.SessionID)
		}
		return errors.NewSessionExpiredError(session.SessionID)
	}

	// Update session
	session.LastMessageAt = time.Now()
	session.RequestID = msg.MessageID
	if err := g.sessionStore.Update(session); err != nil {
		slog.Warn("failed to update session", "sessionID", session.SessionID)
	}

	processCtx, cancel := context.WithCancel(ctx)
	g.registerProcess(session.SessionID, cancel)

	// Process message through context engine
	eventChan := g.contextEngine.Process(processCtx, session, msg.Content)

	// Handle events from context engine
	go func() {
		defer endSpan()
		defer cancel()
		g.handleEngineEvents(processCtx, session, eventChan)
		g.unregisterProcess(session.SessionID)
	}()

	return nil
}

// handleEngineEvents processes events from the context engine
func (g *CommunicationGateway) handleEngineEvents(ctx context.Context, session *types.Session, events <-chan *EngineEvent) {
	slog.Info("gateway: handleEngineEvents started", "sessionID", session.SessionID)
	for {
		select {
		case <-ctx.Done():
			slog.Info("gateway: handleEngineEvents ctx done", "sessionID", session.SessionID)
			return
		case event, ok := <-events:
			if !ok {
				slog.Info("gateway: handleEngineEvents channel closed", "sessionID", session.SessionID)
				return
			}
			g.handleEngineEvent(ctx, session, event)
		}
	}
}

// handleEngineEvent handles a single engine event
func (g *CommunicationGateway) handleEngineEvent(ctx context.Context, session *types.Session, event *EngineEvent) {
	slog.Info("gateway: handleEngineEvent", "type", event.Type, "sessionID", session.SessionID)

	// Record outbound metric
	if g.metricOutboundMsgs != nil {
		g.metricOutboundMsgs.Inc()
	}

	switch event.Type {
	case "thinking":
		session.SetState(types.SessionStateThinking)
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: false,
			Role:      types.MessageRoleAssistant,
			Metadata:  map[string]string{"event_type": "thinking"},
		}
		g.eventHandler.OnMessage(outMsg)

	case "text":
		isComplete := event.Metadata != nil && event.Metadata["is_complete"] == "true"
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: isComplete,
			Role:      types.MessageRoleAssistant,
			Metadata:  map[string]string{"event_type": "text"},
		}
		g.eventHandler.OnMessage(outMsg)

	case "tool_call":
		toolName := event.Metadata["tool_name"]
		if toolName == "" {
			toolName = event.ToolName
		}
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   toolName,
			IsComplete: false,
			Role:      types.MessageRoleAssistant,
			Metadata: map[string]string{
				"event_type": "tool_call",
				"tool_name":  toolName,
				"input":      event.Metadata["input"],
			},
		}
		g.eventHandler.OnMessage(outMsg)
		// Permission handled in L2 via IPermissionGate; Gateway display only.

	case "tool_result":
		session.SetState(types.SessionStateToolExecuting)
		toolName := event.ToolName
		if toolName == "" {
			toolName = event.Metadata["tool_name"]
		}
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: true,
			Role:      types.MessageRoleAssistant,
			Metadata:  map[string]string{"event_type": "tool_result", "tool_name": toolName},
		}
		g.eventHandler.OnMessage(outMsg)

	case "milestone_progress":
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: false,
			Role:      types.MessageRoleAssistant,
			Metadata: map[string]string{
				"event_type": "milestone_progress",
				"progress":   event.Metadata["progress"],
				"task":       event.Metadata["task"],
			},
		}
		g.eventHandler.OnMessage(outMsg)

	case "info":
		// Send info message
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: true,
			Role:      types.MessageRoleAssistant,
			Metadata:  map[string]string{"event_type": "info"},
		}
		g.eventHandler.OnMessage(outMsg)

	case "complete":
		session.SetState(types.SessionStateCompleted)
		usage := event.Metadata["usage"]
		duration := event.Metadata["duration"]
		summary := fmt.Sprintf("用时: %s, 消耗: %s", duration, usage)
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   summary,
			IsComplete: true,
			Role:      types.MessageRoleAssistant,
			Metadata:  map[string]string{"event_type": "complete"},
		}
		g.eventHandler.OnMessage(outMsg)
		g.eventHandler.OnStatus(session.SessionID, types.SessionStateCompleted)

	case "error":
		session.SetState(types.SessionStateFailed)
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: true,
			Role:      types.MessageRoleAssistant,
			Metadata:  map[string]string{"event_type": "error"},
		}
		g.eventHandler.OnMessage(outMsg)
		g.eventHandler.OnError(fmt.Errorf("%s", event.Content), session.SessionID)
	}
}

// RouteOutbound sends an outbound message to the adapter
func (g *CommunicationGateway) RouteOutbound(msg *types.OutboundMessage) error {
	g.eventHandler.OnMessage(msg)
	return nil
}

// RoutePermission handles a permission request
func (g *CommunicationGateway) RoutePermission(req *types.PermissionRequest) (bool, error) {
	approved := g.eventHandler.OnPermissionRequest(req)
	return approved, nil
}

// RouteError sends an error to the adapter
func (g *CommunicationGateway) RouteError(err error, sessionID string) {
	g.eventHandler.OnError(err, sessionID)
}

// CreateSession creates a new session
func (g *CommunicationGateway) CreateSession(chatID, workDir string) (*types.Session, error) {
	if workDir == "" {
		if wd, err := os.Getwd(); err == nil {
			workDir = wd
		}
	}
	sessionID := generateSessionID()
	session := types.NewSession(sessionID, "cli", workDir)
	session.ChatID = chatID

	if err := g.sessionStore.Create(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	g.mu.Lock()
	g.sessions[sessionID] = session
	g.mu.Unlock()

	// Record session creation metric
	if g.metricSessionsTotal != nil {
		g.metricSessionsTotal.Inc()
	}
	if g.metricActiveSessions != nil {
		g.metricActiveSessions.Inc()
	}
	g.recordSessionLifecycle(sessionID, session.AdapterID, "create")

	return session, nil
}

// GetSession returns a session by ID
func (g *CommunicationGateway) GetSession(sessionID string) (*types.Session, error) {
	session, err := g.sessionStore.Get(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, errors.NewSessionNotFoundError(sessionID)
	}
	return session, nil
}

// ExpireSession marks a session as expired
func (g *CommunicationGateway) ExpireSession(sessionID string) error {
	session, err := g.sessionStore.Get(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return errors.NewSessionNotFoundError(sessionID)
	}

	session.State = types.SessionStateFailed
	if err := g.sessionStore.Update(session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	g.mu.Lock()
	delete(g.sessions, sessionID)
	g.mu.Unlock()

	// Also delete from persistent store to prevent storage leak
	if err := g.sessionStore.Delete(sessionID); err != nil {
		// Log but don't fail - session is already removed from memory
		// The store implementation may not support delete or already cleaned up
	}

	if g.metricActiveSessions != nil {
		g.metricActiveSessions.Dec()
	}
	g.recordSessionLifecycle(sessionID, session.AdapterID, "expire")

	return nil
}

// getOrCreateSession gets an existing session or creates a new one
func (g *CommunicationGateway) getOrCreateSession(msg *types.InboundMessage) (*types.Session, error) {
	if msg.SessionID != "" {
		session, err := g.sessionStore.Get(msg.SessionID)
		if err != nil {
			return nil, err
		}
		if session != nil {
			return session, nil
		}
	}

	return g.CreateSession(msg.ChatID, msg.Metadata["work_dir"])
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}

// parseRiskLevel parses a risk level string
func parseRiskLevel(level string) types.RiskLevel {
	switch level {
	case "LOW":
		return types.RiskLevelLow
	case "MEDIUM":
		return types.RiskLevelMedium
	case "HIGH":
		return types.RiskLevelHigh
	case "CRITICAL":
		return types.RiskLevelCritical
	default:
		return types.RiskLevelMedium
	}
}

func (g *CommunicationGateway) recordSessionLifecycle(sessionID, adapter, action string) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return
	}
	if adapter == "" {
		adapter = "unknown"
	}
	_, span := g.obsBridge.Tracer().Start(context.Background(), telemetry.OpGatewaySessionLifecycle,
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionLifecycle,
			tracer.Attribute{Key: "session.action", Value: action},
			tracer.Attribute{Key: "session.id", Value: sessionID},
			tracer.Attribute{Key: "adapter", Value: adapter},
		)...),
	)
	span.End()
}

func (g *CommunicationGateway) startInboundSpan(ctx context.Context, msg *types.InboundMessage) (context.Context, func()) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, func() {}
	}

	tr := g.obsBridge.Tracer()
	ctx, span := tr.Start(ctx, telemetry.OpGatewayMessageReceive,
		tracer.WithSpanKind(tracer.SpanKindServer),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayMessageReceive,
			tracer.Attribute{Key: "session.id", Value: msg.SessionID},
			tracer.Attribute{Key: "message.adapter_id", Value: msg.AdapterID},
			tracer.Attribute{Key: "message.chat_id", Value: msg.ChatID},
			tracer.Attribute{Key: "message.user_id", Value: msg.UserID},
			tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(msg.Content)),
			},
		)...),
	)

	return ctx, func() {
		span.SetStatus(tracer.StatusCodeOk, "")
		span.End()
	}
}
