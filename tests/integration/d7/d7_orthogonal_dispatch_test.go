//go:build integration && d7

package d7integration

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-A01-T04 — D7 Intent path orthogonal dispatch (DM-20260615-004).
//
// v1.1.0+: ProcessMessage's 4-case switch maps each IntentKind to an
// independent execution chain:
//   - IntentCommand  → CommandHandler.Handle   (zero LLM)
//   - IntentFast     → RunSessionTurnLoop      (ItemPipeline + WorkTree)
//   - IntentOrchestrate → RunSessionTurnLoop  (same ingress as IntentFast)
//   - IntentSkip     → close channel           (inlined)

// T: D7-S2-A01-T04 (Command path)
func TestIntegration_D7ProcessMessage_CommandBypassesLLM(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "should-not-be-called"}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{LLMStub: stub})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "/plan")

	if got := stub.CallCount.Load(); got != 0 {
		t.Fatalf("CommandHandler must not invoke LLM, but CallCount = %d", got)
	}

	var sawHelp bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "Plan Commands:") {
			sawHelp = true
			break
		}
	}
	if !sawHelp {
		t.Fatalf("expected Plan Commands help text from CommandHandler, got: %+v",
			stack.Handler.OutboundMessages())
	}
}

// T: D7-S2-A01-T05 (Turn loop path)
//
// "hi" matches the fast-pattern set in RuleClassifier (greeting) →
// IntentFast → RunSessionTurnLoop. The WorkItem executor must invoke
// the LLM at least once.
func TestIntegration_D7ProcessMessage_TurnLoopUsesLLM(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "turn loop reply"}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{LLMStub: stub})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "hi")

	if got := stub.CallCount.Load(); got < 1 {
		t.Fatalf("RunSessionTurnLoop must invoke LLM at least once, but CallCount = %d", got)
	}

	var sawReply bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "turn loop reply") {
			sawReply = true
			break
		}
	}
	if !sawReply {
		t.Fatalf("expected LLM stub reply in outbound, got: %+v",
			stack.Handler.OutboundMessages())
	}
}
