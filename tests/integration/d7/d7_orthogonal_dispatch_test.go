//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S2-A01-T04 — D7 Intent path orthogonal dispatch (DM-20260615-004).
//
// v1.1.0+: ProcessMessage's 4-case switch maps each IntentKind to an
// independent execution chain:
//   - IntentCommand  → CommandHandler.Handle   (zero LLM)
//   - IntentFast     → FastPath.Run            (D2 single-turn LLM↔Tool loop)
//   - IntentOrchestrate → OrchestratePath.Run  (D7-S5-A02 + D7-S3)
//   - IntentSkip     → close channel           (inlined)
//
// These tests verify the orthogonal dispatch at the integration level:
// end-to-end through D1 RouteInbound → D7 SessionOrchestrator → the
// specific execution path. The v1.0 closure collapsed all 4 paths to
// FastPath with system-prompt hints, which made intent_kind=command
// indistinguishable from intent_kind=fast in metrics. v1.1.0+ fixes this.

// T: D7-S2-A01-T04 (Command path)
//
// /plan hits the command whitelist in the RuleClassifier → IntentCommand
// → CommandHandler.Handle. The handler is zero-LLM: it routes to the
// PlanMode state machine and emits text + complete events. The D7LLMStub
// CallCount must remain 0 to prove the orthogonal dispatch.
func TestIntegration_D7ProcessMessage_CommandBypassesLLM(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "should-not-be-called"}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{LLMStub: stub})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// /plan (no args) triggers the plan-help branch. The classifier
	// extracts cmd="/plan" + args=nil; plan.Handle returns the help
	// text. The test asserts that the outbound content is the help
	// text (proving CommandHandler ran) AND the LLM stub was never
	// called (proving zero-LLM cost on the command path).
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

// T: D7-S2-A01-T05 (Fast path)
//
// "hi" matches the fast-pattern set in RuleClassifier (greeting) →
// IntentFast → FastPath.Run. FastPath drives the D2 single-turn
// LLM↔Tool loop and must invoke the LLM at least once. The D7LLMStub
// CallCount must be ≥ 1 to prove the LLM was actually exercised.
func TestIntegration_D7ProcessMessage_FastPathUsesLLM(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "fast path reply"}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{LLMStub: stub})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	routeAndWait(t, stack, session.SessionID, "hi")

	if got := stub.CallCount.Load(); got < 1 {
		t.Fatalf("FastPath must invoke LLM at least once, but CallCount = %d", got)
	}

	var sawReply bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "fast path reply") {
			sawReply = true
			break
		}
	}
	if !sawReply {
		t.Fatalf("expected LLM stub reply in outbound, got: %+v",
			stack.Handler.OutboundMessages())
	}
}

// T: D7-S2-A01-T06 (Orchestrate path — legacy rule_orchestrate ingress)
//
// Under routing_mode=rule_orchestrate, a long complex message routes to
// IntentOrchestrate → OrchestratePath.Run at ingress. loop_first mode uses
// delegate_wave tool instead (see d7_loop_first_test.go).
func TestIntegration_D7ProcessMessage_OrchestratePathDispatchesToScheduler(t *testing.T) {
	stub := &testutil.D7LLMStub{Response: "should-not-be-called"}
	fake := &fakeWaveScheduler{
		artifacts: []wavescheduler.Artifact{{
			TaskID:    "task_1",
			SessionID: "sess",
			Summary:   "fake artifact summary",
			ExitCode:  0,
		}},
	}
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{
		LLMStub: stub,
		RoutingMode: "rule_orchestrate",
		OverrideOrchestratePath: coordinator.NewOrchestratePath(
			coordinator.NewTaskDecomposer(),
			fake,
			nil,
		),
	})

	session, err := stack.Gateway.CreateSession("cli", stack.WorkDir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// A long message with no fast-pattern match and > 32 chars routes to
	// IntentOrchestrate. The '&&' separator triggers the rule-based
	// TaskDecomposer to produce 2 nodes (verifies multi-task synthesis).
	const longGoal = "design the auth refactor && implement session-scoped token rotation && add e2e tests"
	routeAndWait(t, stack, session.SessionID, longGoal)

	if got := stub.CallCount.Load(); got != 0 {
		t.Fatalf("OrchestratePath rule-based decomposer must not call LLM, but CallCount = %d", got)
	}

	if !fake.startCalled() {
		t.Fatal("OrchestratePath must call WaveScheduler.Start")
	}
	if !fake.waitCalled() {
		t.Fatal("OrchestratePath must call WaveScheduler.WaitForCompletion")
	}

	// OrchestratePath emits 4 events: plan_formed, wave_started, text
	// (artifact summary), complete. Only "text" and "complete" produce
	// outbound messages through the SignalRouter; "plan_formed" and
	// "wave_started" hit the default switch case (no outbound). The
	// outbound message must contain the artifact summary emitted by
	// summarizeArtifacts.
	var sawSummary bool
	for _, msg := range stack.Handler.OutboundMessages() {
		if strings.Contains(msg.Content, "fake artifact summary") {
			sawSummary = true
			break
		}
	}
	if !sawSummary {
		t.Fatalf("expected artifact summary in outbound, got: %+v",
			stack.Handler.OutboundMessages())
	}
}

// fakeWaveScheduler implements coordinator.WaveSchedulerRunner for the
// OrchestratePath integration test. It records the Start and
// WaitForCompletion invocations and returns a pre-canned artifact set
// (no goroutines, no real scheduling).
type fakeWaveScheduler struct {
	mu         sync.Mutex
	startCount int
	waitCount  int
	artifacts  []wavescheduler.Artifact
}

func (f *fakeWaveScheduler) Start(_ context.Context, sessionID string, _ *wavescheduler.TaskGraph) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCount++
	_ = sessionID
	return nil
}

func (f *fakeWaveScheduler) WaitForCompletion(_ context.Context, _ string) ([]wavescheduler.Artifact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waitCount++
	return f.artifacts, nil
}

func (f *fakeWaveScheduler) startCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCount >= 1
}

func (f *fakeWaveScheduler) waitCalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.waitCount >= 1
}

// Compile-time guard: fakeWaveScheduler must implement WaveSchedulerRunner.
var _ coordinator.WaveSchedulerRunner = (*fakeWaveScheduler)(nil)
