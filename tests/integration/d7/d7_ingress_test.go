//go:build integration && d7

package d7integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-D1-T02 — production wiring: agent factory present but ingress stays on D7.
func TestIntegration_D7Ingress_WithMultiAgentFactory(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{MultiAgent: true})
	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := stack.Gateway.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "chat-d7-ingress",
		MessageID: "msg-d7-ingress",
		Content:   "hello via d7",
		UserID:    "user-d7",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	stack.Gateway.WaitForProcesses()

	ag := stack.SessionAgents.SessionAgent(session.SessionID)
	if ag == nil {
		t.Fatal("expected session leader after D7 ingress")
	}
	if ag.State() != multiagent.AgentStateCreated {
		t.Fatalf("leader state = %v, want Created", ag.State())
	}
}
