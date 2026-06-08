//go:build acceptance

package p0

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// Covers: L5-1-1-01, L5-0-1-06
func TestL5_COMM_Gateway_CreateSessionRejected(t *testing.T) {
	store := &rejectSessionStore{createErr: fmt.Errorf("storage unavailable")}
	cfg := config.DefaultConfig()
	gw := gateway.NewCommunicationGateway(store, testutil.NewMockEventHandler(), nil, nil, cfg)

	_, err := gw.CreateSession("cli", t.TempDir())
	if err == nil {
		t.Fatal("expected CreateSession error")
	}
	if len(store.created) != 0 {
		t.Fatalf("expected no session persisted, got %d", len(store.created))
	}
}

type rejectSessionStore struct {
	createErr error
	created   []*types.Session
}

func (s *rejectSessionStore) Create(session *types.Session) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.created = append(s.created, session)
	return nil
}

func (s *rejectSessionStore) Get(string) (*types.Session, error) { return nil, nil }
func (s *rejectSessionStore) Update(*types.Session) error         { return nil }
func (s *rejectSessionStore) Delete(string) error                 { return nil }
func (s *rejectSessionStore) List() ([]*types.Session, error)     { return nil, nil }
func (s *rejectSessionStore) GetIdleSessions(time.Duration) ([]*types.Session, error) {
	return nil, nil
}

// Covers: L5-COMM-02, L5-COMM-08
func TestL5_COMM_Gateway_InboundOutboundFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	eventHandler := testutil.NewMockEventHandler()
	engine := &testutil.MockContextEngine{
		Events: []*gateway.EngineEvent{
			{Type: "text", Content: "hello from engine"},
			{Type: "complete"},
		},
	}

	gw := gateway.NewCommunicationGateway(store, eventHandler, engine, nil, cfg)

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	err = gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		Content:   "",
	})
	if err == nil {
		t.Fatal("expected error for empty inbound message")
	}

	err = gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		Content:   "ping",
	})
	if err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	if !eventHandler.WaitForMessages(1, 2*time.Second) {
		t.Fatal("expected outbound messages from engine events")
	}
}
