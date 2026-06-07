//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// Covers: L5-CTX-05, L5-CTX-06, L5-CTX-09, L5-CTX-11
func TestIntegration_ContextEngineGatewayFlow(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	cfg := config.DefaultConfig()
	ctxCfg := config.DefaultContextEngineConfig()
	handler := testutil.NewMockEventHandler()
	permMgr := gateway.NewPermissionManager(&cfg.Permission)
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        &mockctx.LLMGateway{Response: "Hello from context engine"},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   registry.NewBuiltinRegistry(),
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
	})

	gw := gateway.NewCommunicationGateway(store, handler, engine, permMgr, cfg)

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

// Covers: L5-CTX-11
func TestIntegration_PermissionDeniedStopsToolExecution(t *testing.T) {
	dir := t.TempDir()
	store, _ := gateway.NewFileSessionStore(dir)
	cfg := config.DefaultConfig()
	ctxCfg := config.DefaultContextEngineConfig()
	handler := testutil.NewMockEventHandler()
	permMgr := gateway.NewPermissionManager(&cfg.Permission)

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM: &mockctx.LLMGatewayWithTools{},
		Tools: &mockctx.ToolRunner{},
		ToolsReg: registry.NewBuiltinRegistry(),
		Permission: mockctx.DenyAllPermission{},
		Config: ctxCfg,
	})

	gw := gateway.NewCommunicationGateway(store, handler, engine, permMgr, cfg)
	session, _ := gw.CreateSession("cli", "/tmp")

	_ = gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "run tool",
		MessageID: "msg-002",
	})

	handler.WaitForMessages(1, 3*time.Second)
}
