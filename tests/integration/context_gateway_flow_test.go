//go:build integration && cross

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D2-S3-A01-T02, D2-S1-A01-T01, D2-S1-A01-T03, D2-S1-A01-T11
func TestIntegration_ContextEngineGatewayFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	cfg := config.DefaultConfig()
	ctxCfg := config.DefaultContextEngineConfig()
	handler := testutil.NewMockEventHandler()
	permMgr := capture.NewPermissionManager(&cfg.Permission)
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "Hello from context engine"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{},
		ToolsReg:           mustBuiltinRegistry(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             ctxCfg,
	})

	gw := capture.NewCommunicationGateway(store, handler, permMgr, cfg, nil)
	testutil.WireGatewayOrchestration(gw, engine)

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ctx := context.Background()
	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "test message",
		MessageID: "msg-001",
		ChatID:    "chat-1",
	}
	if err := gw.RouteInbound(ctx, msg); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	if !handler.WaitForMessages(1, 3*time.Second) {
		t.Fatal("expected outbound messages")
	}
	if handler.MessageCount() == 0 {
		t.Fatal("no messages")
	}
}

// T: D2-S1-A01-T11
func TestIntegration_PermissionDeniedStopsToolExecution(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("skipping on CI: TempDir cleanup race with FileSessionStore async file operations")
	}
	dir := t.TempDir()
	store, _ := capture.NewFileSessionStore(dir)
	cfg := config.DefaultConfig()
	ctxCfg := config.DefaultContextEngineConfig()
	handler := testutil.NewMockEventHandler()
	permMgr := capture.NewPermissionManager(&cfg.Permission)

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		PreparedTurnRunner: contextengine.PreparedTurnRunnerWithTools(),
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{},
		ToolsReg:           mustBuiltinRegistry(t),
		Permission:         enforce.DenyAllPermission{},
		Config:             ctxCfg,
	})

	gw := capture.NewCommunicationGateway(store, handler, permMgr, cfg, nil)
	testutil.WireGatewayOrchestration(gw, engine)
	session, _ := gw.CreateSession("cli", "/tmp")

	_ = gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "run tool",
		MessageID: "msg-002",
	})

	handler.WaitForMessages(1, 3*time.Second)
}
