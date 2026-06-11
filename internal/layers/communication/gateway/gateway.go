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
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
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

// IContextEngine is the L1 alias for the cross-layer engine contract.
type IContextEngine = contracts.IEngine

// EngineEvent is the L1 alias for engine events.
type EngineEvent = contracts.EngineEvent

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
	agentFactory    multiagent.IAgentFactory
	agentObserverFactory func(ctx context.Context, session *types.Session) multiagent.AgentObserver
	sessionAgents   map[string]multiagent.Agent

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
	if g.permissionMgr != nil {
		g.permissionMgr.SetObservability(obs)
	}
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
		if _, active := g.activeProcesses[sessionID]; active {
			continue
		}
		if now.Sub(session.LastMessageAt) > timeout {
			slog.Debug("cleaning up expired session", "sessionID", sessionID)
			delete(g.sessions, sessionID)
		}
	}
}

// RouteInbound processes an inbound message from an adapter
func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	ctx, endSpan := g.startInboundSpan(ctx, msg)
	ctx = g.seedInboundBaggage(ctx, msg)

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
	session, err := g.getOrCreateSession(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to get or create session: %w", err)
	}

	// Inbound message means the user is active again — resume idle sessions instead
	// of rejecting. Background cleanup (StartCleanupRoutine) handles abandoned sessions.
	if g.config.Session.IdleTimeout > 0 && session.IsIdle(g.config.Session.IdleTimeout) {
		slog.Info("gateway: resuming idle session on inbound message", "sessionID", session.SessionID)
	}

	// Update session
	session.LastMessageAt = time.Now()
	session.RequestID = msg.MessageID
	g.mu.Lock()
	if cached, ok := g.sessions[session.SessionID]; ok && cached != nil {
		cached.LastMessageAt = session.LastMessageAt
		cached.RequestID = session.RequestID
	} else {
		g.sessions[session.SessionID] = session
	}
	g.mu.Unlock()
	_, storeSpan := g.startStoreUpdateSpan(ctx, session.SessionID)
	if err := g.sessionStore.Update(session); err != nil {
		if storeSpan != nil {
			storeSpan.RecordError(err)
			storeSpan.End()
		}
		slog.Warn("failed to update session", "sessionID", session.SessionID)
	} else {
		if storeSpan != nil { storeSpan.End() }
	}

	if g.agentFactory != nil {
		return g.routeInboundViaAgent(ctx, msg, session, endSpan)
	}

	if g.contextEngine == nil {
		return fmt.Errorf("context engine not configured")
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
		g.persistSessionAfterProcess(session)
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

// PublishEngineEvent delivers an engine event for a session (async worker progress, etc.).
func (g *CommunicationGateway) PublishEngineEvent(ev *EngineEvent) {
	if g == nil || ev == nil || ev.SessionID == "" {
		return
	}
	session, err := g.GetSession(ev.SessionID)
	if err != nil || session == nil {
		slog.Debug("gateway: PublishEngineEvent session not found", "sessionID", ev.SessionID)
		return
	}
	go g.handleEngineEvent(context.Background(), session, ev)
}

// handleEngineEvent handles a single engine event
func (g *CommunicationGateway) handleEngineEvent(ctx context.Context, session *types.Session, event *EngineEvent) {
	slog.Info("gateway: handleEngineEvent", "type", event.Type, "sessionID", session.SessionID)

	ctx, evSpan := g.startEngineEventSpan(ctx, session, event.Type)
	if evSpan != nil {
		defer evSpan.End()
	}

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
			Metadata:  outboundMetadata("thinking", event),
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
			Metadata:  outboundMetadata("text", event),
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
		meta := map[string]string{
			"event_type":         "milestone_progress",
			"progress":           event.Metadata["progress"],
			"task":               event.Metadata["task"],
			"milestone_id":       event.Metadata["milestone_id"],
			"render":             "milestone",
			"milestone_name":     event.Metadata["task"],
			"milestone_progress": event.Metadata["progress"],
			"milestone_status": string(types.MilestoneStatusInProgress),
		}
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: false,
			Role:      types.MessageRoleAssistant,
			Metadata:  meta,
		}
		g.eventHandler.OnMessage(outMsg)

	case "worker_progress":
		meta := map[string]string{"event_type": "worker_progress"}
		for k, v := range event.Metadata {
			meta[k] = v
		}
		if meta["render"] == "" {
			meta["render"] = "worker_tree"
		}
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: false,
			Role:      types.MessageRoleAssistant,
			Metadata:  meta,
		}
		g.eventHandler.OnMessage(outMsg)

	case "info":
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.ChatID,
			Content:   event.Content,
			IsComplete: true,
			Role:      types.MessageRoleAssistant,
			Metadata:  outboundMetadata("info", event),
		}
		g.eventHandler.OnMessage(outMsg)

	case "complete":
		session.SetState(types.SessionStateCompleted)
		usage := event.Metadata["usage"]
		duration := event.Metadata["duration"]
		model := event.Metadata["model"]
		// DM-20260611-008：D2 PEV/QueryLoop 在 emit complete 时透传 ctx_pct。
		// 为空/0/异常时 D1 buildCompletionSummary 自动省略 ctx 段。
		ctxPct := event.Metadata["ctx_pct"]
		summary := buildCompletionSummary(duration, usage, model, ctxPct)
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
	}
}

// RouteOutbound sends an outbound message to the adapter
func (g *CommunicationGateway) RouteOutbound(msg *types.OutboundMessage) error {
	g.eventHandler.OnMessage(msg)
	return nil
}

// RoutePermission handles a permission request
func (g *CommunicationGateway) RoutePermission(req *types.PermissionRequest) (bool, error) {
	_, permSpan := g.startPermissionSpan(context.Background(), req)
	approved := g.eventHandler.OnPermissionRequest(req)
	if permSpan != nil {
		permSpan.SetAttributes(
			tracer.Attribute{Key: "permission.result", Value: fmt.Sprintf("%t", approved)},
		)
		permSpan.SetStatus(tracer.StatusCodeOk, "")
		permSpan.End()
	}
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

	_, createSpan := g.startSessionCreateSpan(context.Background(), chatID, workDir)

	sessionID := generateSessionID()
	session := types.NewSession(sessionID, "cli", workDir)
	session.ChatID = chatID

	if err := g.sessionStore.Create(session); err != nil {
		if createSpan != nil { createSpan.End() }
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
	if createSpan != nil {
		createSpan.SetAttributes(tracer.Attribute{Key: "session.id", Value: sessionID})
		createSpan.End()
	}

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

// ResolveSessionByChatID returns the most recently active session for chatID
// that has not exceeded the idle timeout. Used to recover context after restart.
func (g *CommunicationGateway) ResolveSessionByChatID(chatID string) (*types.Session, error) {
	if chatID == "" {
		return nil, nil
	}

	sessions, err := g.sessionStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var best *types.Session
	var bestScore int64
	for _, session := range sessions {
		if session == nil || session.ChatID != chatID {
			continue
		}
		score := sessionRestoreScore(session)
		if best == nil || score > bestScore {
			best = session
			bestScore = score
		}
	}
	if best == nil {
		return nil, nil
	}

	g.mu.Lock()
	g.sessions[best.SessionID] = best
	g.mu.Unlock()

	slog.Info("gateway: restored session from store",
		"sessionID", best.SessionID,
		"chatID", chatID,
		"snapshotBytes", len(best.ContextSnapshot),
	)
	return best, nil
}

// sessionRestoreScore ranks sessions for post-restart recovery.
// Sessions with persisted context outrank empty ones even if they are idle.
func sessionRestoreScore(session *types.Session) int64 {
	if session == nil {
		return 0
	}
	snapshotLen := int64(len(session.ContextSnapshot))
	if snapshotLen > 0 {
		return snapshotLen*1_000_000_000_000 + session.LastMessageAt.Unix()
	}
	return session.LastMessageAt.Unix()
}

// ExpireSession marks a session as expired
func (g *CommunicationGateway) ExpireSession(sessionID string) error {
	_, expireSpan := g.startSessionExpireSpan(context.Background(), sessionID)

	session, err := g.sessionStore.Get(sessionID)
	if err != nil {
		if expireSpan != nil { expireSpan.End() }
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		if expireSpan != nil { expireSpan.End() }
		return errors.NewSessionNotFoundError(sessionID)
	}

	session.State = types.SessionStateFailed
	if err := g.sessionStore.Update(session); err != nil {
		if expireSpan != nil { expireSpan.End() }
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
	if expireSpan != nil {
		expireSpan.SetAttributes(tracer.Attribute{Key: "adapter", Value: session.AdapterID})
		expireSpan.End()
	}

	return nil
}

func (g *CommunicationGateway) startSessionExpireSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionExpire,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewaySessionExpire, opts...)
}

// getOrCreateSession gets an existing session or creates a new one
func (g *CommunicationGateway) getOrCreateSession(ctx context.Context, msg *types.InboundMessage) (*types.Session, error) {
	if msg.SessionID != "" {
		_, getSpan := g.startSessionGetSpan(ctx, msg.SessionID)
		session, err := g.sessionStore.Get(msg.SessionID)
		if getSpan != nil {
			getSpan.End()
		}
		if err != nil {
			return nil, err
		}
		if session != nil {
			return session, nil
		}
	}

	return g.CreateSession(msg.ChatID, msg.Metadata["work_dir"])
}

func (g *CommunicationGateway) startSessionGetSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionGet,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewaySessionGet, opts...)
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

func outboundMetadata(eventType string, event *EngineEvent) map[string]string {
	meta := map[string]string{"event_type": eventType}
	if event == nil || event.Metadata == nil {
		return meta
	}
	for k, v := range event.Metadata {
		if k == "event_type" || v == "" {
			continue
		}
		meta[k] = v
	}
	return meta
}

func (g *CommunicationGateway) startEngineEventSpan(ctx context.Context, session *types.Session, eventType string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayEngineEvent,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "event.type", Value: eventType},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewayEngineEvent, opts...)
}

func (g *CommunicationGateway) startStoreUpdateSpan(ctx context.Context, sessionID string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayStoreUpdate,
			tracer.Attribute{Key: "session.id", Value: sessionID},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewayStoreUpdate, opts...)
}

func (g *CommunicationGateway) startSessionCreateSpan(ctx context.Context, chatID, workDir string) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewaySessionCreate,
			tracer.Attribute{Key: "adapter", Value: "cli"},
			tracer.Attribute{Key: "work_dir", Value: workDir},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewaySessionCreate, opts...)
}

func (g *CommunicationGateway) startPermissionSpan(ctx context.Context, req *types.PermissionRequest) (context.Context, tracer.Span) {
	if g.obsBridge == nil || g.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpGatewayPermissionCheck,
			tracer.Attribute{Key: "session.id", Value: req.SessionID},
			tracer.Attribute{Key: "tool.name", Value: req.ToolName},
			tracer.Attribute{Key: "risk_level", Value: string(req.RiskLevel)},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return g.obsBridge.Tracer().Start(ctx, telemetry.OpGatewayPermissionCheck, opts...)
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

func (g *CommunicationGateway) seedInboundBaggage(ctx context.Context, msg *types.InboundMessage) context.Context {
	if msg == nil {
		return ctx
	}
	bm := tracer.DefaultBaggageManager
	ctx = bm.Set(ctx, "session.id", msg.SessionID)
	if msg.UserID != "" {
		ctx = bm.Set(ctx, "user.id", msg.UserID)
	}
	return ctx
}
