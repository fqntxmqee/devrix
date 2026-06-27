//go:build integration && d7

package d7integration

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-L5-01 — loop_first greeting uses RunSessionTurnLoop, no wave ingress.
func TestIntegration_D7LoopFirst_GreetingNoWave(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "你好！"}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{LLMStub: stub})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "你好")

	if got := stub.CallCount.Load(); got == 0 {
		t.Fatal("greeting should invoke Turn LLM at least once")
	}
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "plan_formed") || strings.Contains(msg.Content, "wave_started") {
			t.Fatalf("unexpected wave content in outbound: %q", msg.Content)
		}
	}
}
