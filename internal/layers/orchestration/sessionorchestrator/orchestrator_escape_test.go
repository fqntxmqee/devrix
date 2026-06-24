// V5.5 单元测试: 5 节点接线 (DM-20260625-003, PR-V5.5)
//
// 守护 4 个接线点 (1a/1b/2/3) 独立工作:
//   - 1a: Plan fails → Evaluate → ForceExit
//   - 1b: Plan 前 → Evaluate → ForceExit
//   - 2:  Execute fails → Evaluate → ForceExit
//   - 3:  Verify fails → Evaluate → ForceExit (TODO: 后续 PR)
//
// 失败降级: 接线点错误 → slog.Warn + 继续 (不破坏主链路)
package sessionorchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/escape"
	"github.com/devrix/devrix/internal/layers/orchestration/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// stubDepthChecker returns a fixed EscapeDecision for all calls.
type stubDepthChecker struct {
	decision escape.EscapeDecision
}

func (s *stubDepthChecker) ShouldContinue(_ escape.LoopContext) escape.EscapeDecision {
	return s.decision
}

// newFakeEscapeEngine builds an EscapeEngine that immediately returns
// the desired decision via a stub DepthChecker. This bypasses the
// chained arbitrator entirely, so we test the wiring logic in
// isolation.
func newFakeEscapeEngine(action escape.EscapeAction, reason string) *escape.EscapeEngine {
	return escape.NewEscapeEngine(
		&stubDepthChecker{decision: escape.EscapeDecision{
			Action:     action,
			Reason:     reason,
			AuditLevel: 1,
			SessionID:  "test",
			CreatedAt:  time.Now(),
		}},
		nil, // chain
		escape.NewCircuitBreakerSet(),
		nil, // audit
		nil, // resume
	)
}

// --- Test 1: 1a (Plan fails) - classifier error → EscapeForceExit ---------

// errorClassifier always returns an error.
type errorClassifier struct {
	err error
}

func (c *errorClassifier) Classify(_ context.Context, _ string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{}, c.err
}

func (c *errorClassifier) ClassifyWithPrior(_ context.Context, _ string, _ *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{}, c.err
}

// TestEscapeWiring_1a_PlanFails_ForceExit verifies that when the
// classifier returns an error AND the escape engine decides ForceExit,
// ProcessMessage returns the error.
func TestEscapeWiring_1a_PlanFails_ForceExit(t *testing.T) {
	badClassifier := &errorClassifier{err: errors.New("simulated classifier error")}

	engine := newFakeEscapeEngine(escape.EscapeForceExit, "test_force_exit")
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithClassifier(badClassifier),
		WithEscapeEngine(engine),
	)

	_, err := orch.ProcessMessage(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-1a", Message: "hi"})

	if err == nil {
		t.Fatal("ProcessMessage: want err (classifier error + ForceExit), got nil")
	}
	if !strings.Contains(err.Error(), "classify") {
		t.Errorf("err = %q, want contains 'classify'", err)
	}
}

// TestEscapeWiring_1a_PlanFails_Continue: classifier errors but
// escape decision is Continue → still returns classifier error
// (1a is a true short-circuit, see design §6).
func TestEscapeWiring_1a_PlanFails_Continue(t *testing.T) {
	badClassifier := &errorClassifier{err: errors.New("simulated classifier error")}

	engine := newFakeEscapeEngine(escape.EscapeContinue, "all_depth_limits_passed")
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithClassifier(badClassifier),
		WithEscapeEngine(engine),
	)

	_, err := orch.ProcessMessage(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-1a-continue", Message: "hi"})

	if err == nil {
		t.Fatal("ProcessMessage: want err (classifier still errored), got nil")
	}
}

// --- Test 2: 1b (Plan 前) - force exit returns error ----------------------

// TestEscapeWiring_1b_PlanPre_ForceExit verifies that 1b short-circuits
// when escape engine decides ForceExit.
func TestEscapeWiring_1b_PlanPre_ForceExit(t *testing.T) {
	engine := newFakeEscapeEngine(escape.EscapeForceExit, "loop_depth_exceeded")
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithEscapeEngine(engine),
	)

	_, err := orch.ProcessMessage(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-1b", Message: "hi"})

	if err == nil {
		t.Fatal("ProcessMessage: want err (1b ForceExit), got nil")
	}
	if !strings.Contains(err.Error(), "escape") {
		t.Errorf("err = %q, want contains 'escape'", err)
	}
}

// TestEscapeWiring_1b_PlanPre_Continue: normal path → no escape error
func TestEscapeWiring_1b_PlanPre_Continue(t *testing.T) {
	engine := newFakeEscapeEngine(escape.EscapeContinue, "all_depth_limits_passed")
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithEscapeEngine(engine),
	)

	ch, err := orch.ProcessMessage(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-1b-continue", Message: "hi"})
	if err != nil {
		t.Fatalf("ProcessMessage: want no err (1b Continue), got %v", err)
	}
	// Drain channel to allow autoclose to fire.
	for range ch {
	}
}

// --- Test 3: 2 (Execute fails) - path error → Escape decision -------------

// fixedKindClassifier always returns a specific IntentKind.
type fixedKindClassifier struct {
	kind orchtypes.IntentKind
}

func (c *fixedKindClassifier) Classify(_ context.Context, _ string) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{Kind: c.kind, Confidence: 100}, nil
}

func (c *fixedKindClassifier) ClassifyWithPrior(_ context.Context, _ string, _ *learn.AdaptivePrior) (orchtypes.IntentClassification, error) {
	return orchtypes.IntentClassification{Kind: c.kind, Confidence: 100}, nil
}

// TestEscapeWiring_2_ExecuteFails_ForceExit verifies that when the
// path returns an error AND escape engine decides ForceExit,
// the error is propagated.
//
// Mechanism: set orchestratePath=nil so the switch case in
// ProcessMessage sets err (instead of calling Run). Then escape
// engine is consulted at wiring point 2.
func TestEscapeWiring_2_ExecuteFails_ForceExit(t *testing.T) {
	engine := newFakeEscapeEngine(escape.EscapeForceExit, "circuit_breaker_L0_AnomalyDetector_open")
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithClassifier(&fixedKindClassifier{kind: orchtypes.IntentOrchestrate}),
		WithEscapeEngine(engine),
	)
	// Force the path-error branch by nil-ing orchestratePath.
	orch.orchestratePath = nil

	_, err := orch.ProcessMessage(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-2", Message: "hi"})

	if err == nil {
		t.Fatal("ProcessMessage: want err (path error + ForceExit), got nil")
	}
}

// TestEscapeWiring_2_ExecuteFails_Continue: path errors but escape
// decision is Continue → path error still propagates (caller's
// choice to continue would happen at higher level, not here).
func TestEscapeWiring_2_ExecuteFails_Continue(t *testing.T) {
	engine := newFakeEscapeEngine(escape.EscapeContinue, "all_depth_limits_passed")
	orch := NewSessionOrchestrator(
		orchtypes.DefaultConfig(),
		&completingExecutor{eventType: "complete"},
		WithClassifier(&fixedKindClassifier{kind: orchtypes.IntentOrchestrate}),
		WithEscapeEngine(engine),
	)
	orch.orchestratePath = nil

	_, err := orch.ProcessMessage(context.Background(),
		orchtypes.ProcessRequest{SessionID: "sess-2-continue", Message: "hi"})

	if err == nil {
		t.Fatal("ProcessMessage: want err (path still errored), got nil")
	}
}

// --- Test 4: 3 (Verify 失败) - stub test for future wiring ----------------

// TestEscapeWiring_3_VerifyFails_Stub: placeholder for wiring point 3.
// The actual wiring point 3 (Verify失败) is in processAutoClose and
// requires the synthesized Verdict. We document the contract here:
//
//   - verdict.Kind == VerdictFail or VerdictIndeterminate
//   - → build loopCtx with FailureCriterion=verdict.Reason
//   - → Evaluate
//   - → if ForceExit: return err (closes session)
//
// The full wiring is deferred to a follow-up PR once autoclose pipeline
// exposes the verdict to the orchestrator.

func TestEscapeWiring_3_VerifyFails_Stub(t *testing.T) {
	t.Skip("wiring point 3 (Verify失败) deferred — requires verdict exposure in processAutoClose")
}

// --- Test 5: planKindFromIntent mapping -----------------------------------

func TestPlanKindFromIntent(t *testing.T) {
	tests := []struct {
		in   orchtypes.IntentKind
		want escape.PlanKind
	}{
		{orchtypes.IntentSkip, 0},
		{orchtypes.IntentCommand, plan.CommitmentPlan},
		{orchtypes.IntentFast, plan.ExplorationPlan},
		{orchtypes.IntentOrchestrate, plan.ScenarioPlan},
	}
	for _, tt := range tests {
		got := planKindFromIntent(tt.in)
		if got != tt.want {
			t.Errorf("planKindFromIntent(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// --- Test 6: processEscapeDecision helper ----------------------------------

func TestProcessEscapeDecision(t *testing.T) {
	orch := &SessionOrchestrator{}
	tests := []struct {
		action escape.EscapeAction
		want   bool
	}{
		{escape.EscapeContinue, false},
		{escape.EscapeForceExit, true},
		{escape.EscapeAbortWithAudit, true},
		{escape.EscapePendingHuman, true},
		{escape.EscalateToRule, true},
		{escape.EscalateToHuman, true},
	}
	for _, tt := range tests {
		d := escape.EscapeDecision{Action: tt.action, Reason: "test"}
		got := orch.processEscapeDecision(d, nil)
		if got != tt.want {
			t.Errorf("processEscapeDecision(%v) = %v, want %v", tt.action, got, tt.want)
		}
	}
}
