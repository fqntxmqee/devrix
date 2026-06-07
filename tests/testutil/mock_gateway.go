package testutil

import (
	"context"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/types"
)

// MockGatewayAPI is a test double for gateway.GatewayAPI.
type MockGatewayAPI struct {
	GetSessionFunc    func(sessionID string) (*types.Session, error)
	CreateSessionFunc func(chatID, workDir string) (*types.Session, error)
	RouteInboundFunc  func(ctx context.Context, msg *types.InboundMessage) error
	RouteOutboundFunc func(msg *types.OutboundMessage) error
}

var _ gateway.GatewayAPI = (*MockGatewayAPI)(nil)

func (m *MockGatewayAPI) GetSession(sessionID string) (*types.Session, error) {
	if m.GetSessionFunc != nil {
		return m.GetSessionFunc(sessionID)
	}
	return nil, nil
}

func (m *MockGatewayAPI) CreateSession(chatID, workDir string) (*types.Session, error) {
	if m.CreateSessionFunc != nil {
		return m.CreateSessionFunc(chatID, workDir)
	}
	return nil, nil
}

func (m *MockGatewayAPI) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	if m.RouteInboundFunc != nil {
		return m.RouteInboundFunc(ctx, msg)
	}
	return nil
}

func (m *MockGatewayAPI) RouteOutbound(msg *types.OutboundMessage) error {
	if m.RouteOutboundFunc != nil {
		return m.RouteOutboundFunc(msg)
	}
	return nil
}
