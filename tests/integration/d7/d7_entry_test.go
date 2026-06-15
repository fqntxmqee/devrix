//go:build integration && d7

package d7integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-D1-T01
func TestIntegration_D7Entry_RoutesThroughWireD7(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	routeAndWait(t, stack, session.SessionID, "hello d7")
}

// T: D7-D1-T01
func TestIntegration_D7Entry_MissingOrchestrationEntryFailsFast(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	gw := capture.NewCommunicationGateway(store, handler, capture.NewPermissionManager(&config.DefaultConfig().Permission), config.DefaultConfig())

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	err = gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat",
		MessageID: "m1",
		Content:   "hello",
		UserID:    "u1",
	})
	if err == nil {
		t.Fatal("expected error when orchestration entry is not wired")
	}
}

// T: D7-MIG-T01
func TestIntegration_D7Entry_PlanModeStillUsesD7Path(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	routeAndWait(t, stack, session.SessionID, "/plan add authentication")
}

// T: D7-D1-T01
func TestIntegration_D7Entry_StopProcessInvokesCancel(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := stack.Gateway.StopProcess(session.SessionID); err != nil {
		t.Fatalf("StopProcess: %v", err)
	}
	if err := stack.Gateway.StopProcess(session.SessionID); err != nil {
		t.Fatalf("StopProcess idempotent: %v", err)
	}
}
