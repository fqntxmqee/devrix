//go:build integration && d1

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// Covers: L5-COMM-01, L5-COMM-03
func TestIntegration_CLIToGatewayToSession(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	eventHandler := testutil.NewMockEventHandler()

	gw := gateway.NewCommunicationGateway(
		store,
		eventHandler,
		nil,
		nil,
		cfg,
	)

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	got, err := gw.GetSession(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if got.SessionID != session.SessionID {
		t.Errorf("expected session ID %q, got %q", session.SessionID, got.SessionID)
	}

	got, err = store.Get(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get session from store: %v", err)
	}
	if got == nil {
		t.Fatal("expected session in store")
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}

// resumeEngine completes immediately for idle-resume integration checks.
type resumeEngine struct{}

func (resumeEngine) Process(context.Context, *types.Session, string) <-chan *gateway.EngineEvent {
	ch := make(chan *gateway.EngineEvent, 1)
	ch <- &gateway.EngineEvent{Type: "complete"}
	close(ch)
	return ch
}

// Covers: L5-COMM-03
func TestIntegration_SessionExpiration(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Session.IdleTimeout = 2 * time.Second

	gw := gateway.NewCommunicationGateway(store, nil, resumeEngine{}, nil, cfg)

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session.LastMessageAt = time.Now().Add(-1 * time.Hour)
	if err := store.Update(session); err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		Content:   "hello",
		MessageID: "msg-resume-1",
	}

	err = gw.RouteInbound(context.Background(), msg)
	if err != nil {
		t.Fatalf("idle session should resume on inbound message, got: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	got, err := store.Get(session.SessionID)
	if err != nil {
		t.Fatalf("failed to get session after resume: %v", err)
	}
	if got == nil {
		t.Fatal("session should still exist after resume")
	}
	if got.IsIdle(cfg.Session.IdleTimeout) {
		t.Error("LastMessageAt should be refreshed after inbound message")
	}
}
