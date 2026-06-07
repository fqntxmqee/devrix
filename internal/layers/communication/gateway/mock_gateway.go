package gateway

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// MockGatewayAPI is a mock implementation of GatewayAPI for testing
type MockGatewayAPI struct {
	// GetSessionFunc mocks the GetSession method
	GetSessionFunc func(sessionID string) (*types.Session, error)
	// CreateSessionFunc mocks the CreateSession method
	CreateSessionFunc func(chatID, workDir string) (*types.Session, error)
	// RouteInboundFunc mocks the RouteInbound method
	RouteInboundFunc func(ctx context.Context, msg *types.InboundMessage) error
	// RouteOutboundFunc mocks the RouteOutbound method
	RouteOutboundFunc func(msg *types.OutboundMessage) error
}

// Ensure MockGatewayAPI implements GatewayAPI
var _ GatewayAPI = (*MockGatewayAPI)(nil)

// GetSession implements GatewayAPI
func (m *MockGatewayAPI) GetSession(sessionID string) (*types.Session, error) {
	if m.GetSessionFunc != nil {
		return m.GetSessionFunc(sessionID)
	}
	return nil, nil
}

// CreateSession implements GatewayAPI
func (m *MockGatewayAPI) CreateSession(chatID, workDir string) (*types.Session, error) {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(chatID, workDir)
	}
	return nil, nil
}

// RouteInbound implements GatewayAPI
func (m *MockGatewayAPI) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	if m.RouteInboundFunc != nil {
		return m.RouteInboundFunc(ctx, msg)
	}
	return nil
}

// RouteOutbound implements GatewayAPI
func (m *MockGatewayAPI) RouteOutbound(msg *types.OutboundMessage) error {
	if m.RouteOutboundFunc != nil {
		return m.RouteOutboundFunc(msg)
	}
	return nil
}
