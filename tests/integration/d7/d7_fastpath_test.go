//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-T01, D7-S2-T02c, D7-S2-A06-T01
func TestIntegration_D7FastPath_FullStackTurnLoop(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: &testutil.D7LLMStub{Response: "fast path reply"},
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-fast",
		MessageID: "msg-fast",
		Content:   "hi",
		UserID:    "user-d7",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	stack.Gateway.WaitForProcesses()
	if !stack.Handler.WaitForMessages(1, 5*time.Second) {
		t.Fatal("expected outbound messages")
	}

	var sawText bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "fast path reply") {
			sawText = true
		}
	}
	if !sawText {
		t.Fatalf("expected LLM stub response in outbound messages: %+v", stack.Handler.OutboundMessages())
	}
}

// T: D7-S2-A01-T04 (v1.1.0+ name) — plan command now routes through
// CommandHandler, NOT TurnOrchestrator. v1.0 closure routed command to
// FastPath with a "[command:xxx]" system-prompt hint, but the v1.1.0
// orthogonal dispatch (devrix-d7-orthogonal-intent-paths) eliminates
// the TurnLoop pass-through. The thin smoke test stays for regression
// coverage of the D1→D7 path under the command classifier branch;
// deeper verification of "CommandHandler bypasses LLM" lives in
// d7_orthogonal_dispatch_test.go.
func TestIntegration_D7CommandFirst_PlanCommandBypassesTurnLoop(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: &testutil.D7LLMStub{Response: "plan command handled"},
	})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	routeAndWait(t, stack, session.SessionID, "/plan explore auth module")
}
