package capture

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// GatewayAPI defines the interface for CommunicationGateway operations
// that are used by adapters. This allows for mocking in tests.
type GatewayAPI interface {
	// GetSession returns a session by ID
	GetSession(sessionID string) (*types.Session, error)
	// ResolveSessionByChatID returns the latest non-idle session for a chat key.
	ResolveSessionByChatID(chatID string) (*types.Session, error)
	// CreateSession creates a new session
	CreateSession(chatID, workDir string) (*types.Session, error)
	// RouteInbound processes an inbound message
	RouteInbound(ctx context.Context, msg *types.InboundMessage) error
	// RouteOutbound sends an outbound message
	RouteOutbound(msg *types.OutboundMessage) error
	// StopProcess cancels the active context engine process for the given session.
	StopProcess(sessionID string) error
}

// Ensure CommunicationGateway implements GatewayAPI
var _ GatewayAPI = (*CommunicationGateway)(nil)
