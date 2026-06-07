package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

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

	mu      sync.RWMutex
	sessions map[string]*types.Session
}

// NewCommunicationGateway creates a new CommunicationGateway
func NewCommunicationGateway(
	sessionStore SessionStore,
	eventHandler EventHandler,
	contextEngine IContextEngine,
	permissionMgr *PermissionManager,
	cfg *config.CommunicationConfig,
) *CommunicationGateway {
	return &CommunicationGateway{
		sessionStore:  sessionStore,
		eventHandler: eventHandler,
		contextEngine: contextEngine,
		permissionMgr: permissionMgr,
		config:       cfg,
		sessions:    make(map[string]*types.Session),
	}
}

// RouteInbound processes an inbound message from an adapter
func (g *CommunicationGateway) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
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
	if err := g.sessionStore.Update(session); err != nil {
		slog.Warn("failed to update session", "sessionID", session.SessionID)
	}

	// Process message through context engine
	eventChan := g.contextEngine.Process(ctx, session, msg.Content)

	// Handle events from context engine
	go g.handleEngineEvents(ctx, session, eventChan)

	return nil
}

// handleEngineEvents processes events from the context engine
func (g *CommunicationGateway) handleEngineEvents(ctx context.Context, session *types.Session, events <-chan *EngineEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			g.handleEngineEvent(ctx, session, event)
		}
	}
}

// handleEngineEvent handles a single engine event
func (g *CommunicationGateway) handleEngineEvent(ctx context.Context, session *types.Session, event *EngineEvent) {
	switch event.Type {
	case "thinking":
		session.SetState(types.SessionStateThinking)
		g.eventHandler.OnStatus(session.SessionID, types.SessionStateThinking)

	case "text":
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			ChatID:    session.SessionID,
			Content:   event.Content,
			IsComplete: false,
			Role:      types.MessageRoleAssistant,
		}
		g.eventHandler.OnMessage(outMsg)

	case "tool_call":
		// Request permission
		toolName := event.Metadata["tool_name"]
		riskLevel := parseRiskLevel(event.Metadata["risk_level"])

		approved := g.permissionMgr.Request(ctx, session.SessionID, toolName, event.Metadata["input"], riskLevel)
		if !approved {
			outMsg := &types.OutboundMessage{
				MessageID:  generateMessageID(),
				SessionID: session.SessionID,
				Content:   fmt.Sprintf("Permission denied for tool: %s", toolName),
				IsComplete: true,
				Role:      types.MessageRoleAssistant,
			}
			g.eventHandler.OnMessage(outMsg)
			return
		}

	case "tool_result":
		session.SetState(types.SessionStateToolExecuting)
		outMsg := &types.OutboundMessage{
			MessageID:  generateMessageID(),
			SessionID: session.SessionID,
			Content:   fmt.Sprintf("[Tool: %s] %s", event.ToolName, event.Content),
			IsComplete: true,
			Role:      types.MessageRoleAssistant,
		}
		g.eventHandler.OnMessage(outMsg)

	case "complete":
		session.SetState(types.SessionStateCompleted)
		g.eventHandler.OnStatus(session.SessionID, types.SessionStateCompleted)

	case "error":
		session.SetState(types.SessionStateFailed)
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
	sessionID := generateSessionID()
	session := types.NewSession(sessionID, "cli", workDir)

	if err := g.sessionStore.Create(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	g.mu.Lock()
	g.sessions[sessionID] = session
	g.mu.Unlock()

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
