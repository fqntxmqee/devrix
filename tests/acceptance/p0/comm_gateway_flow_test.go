//go:build acceptance && d1

package p0

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D1-S1-A01-T01, D0-S1-A01-T06
func TestL5_COMM_Gateway_CreateSessionRejected(t *testing.T) {
	store := &rejectSessionStore{createErr: fmt.Errorf("storage unavailable")}
	cfg := config.DefaultConfig()
	gw := capture.NewCommunicationGateway(store, testutil.NewMockEventHandler(), nil, cfg)

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

// T: D1-S1-A01-T01, D1-S1-A01-T03
func TestL5_COMM_Gateway_InboundOutboundFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	eventHandler := testutil.NewMockEventHandler()
	engine := &testutil.MockContextEngine{
		Events: []*capture.EngineEvent{
			{Type: "text", Content: "hello from engine"},
			{Type: "complete"},
		},
	}

	gw := capture.NewCommunicationGateway(store, eventHandler, nil, cfg)
	testutil.WireGatewayOrchestration(gw, engine)

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
