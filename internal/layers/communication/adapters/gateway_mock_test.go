package adapters

import (
	"context"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/types"
)

type mockGatewayAPI struct {
	getSessionFunc             func(sessionID string) (*types.Session, error)
	resolveSessionByChatIDFunc func(chatID string) (*types.Session, error)
	createSessionFunc          func(chatID, workDir string) (*types.Session, error)
	routeInboundFunc           func(ctx context.Context, msg *types.InboundMessage) error
	routeOutboundFunc          func(msg *types.OutboundMessage) error
}

var _ gateway.GatewayAPI = (*mockGatewayAPI)(nil)

func (m *mockGatewayAPI) GetSession(sessionID string) (*types.Session, error) {
	if m.getSessionFunc != nil {
		return m.getSessionFunc(sessionID)
	}
	return nil, nil
}

func (m *mockGatewayAPI) ResolveSessionByChatID(chatID string) (*types.Session, error) {
	if m.resolveSessionByChatIDFunc != nil {
		return m.resolveSessionByChatIDFunc(chatID)
	}
	return nil, nil
}

func (m *mockGatewayAPI) CreateSession(chatID, workDir string) (*types.Session, error) {
	if m.createSessionFunc != nil {
		return m.createSessionFunc(chatID, workDir)
	}
	return nil, nil
}

func (m *mockGatewayAPI) RouteInbound(ctx context.Context, msg *types.InboundMessage) error {
	if m.routeInboundFunc != nil {
		return m.routeInboundFunc(ctx, msg)
	}
	return nil
}

func (m *mockGatewayAPI) RouteOutbound(msg *types.OutboundMessage) error {
	if m.routeOutboundFunc != nil {
		return m.routeOutboundFunc(msg)
	}
	return nil
}
