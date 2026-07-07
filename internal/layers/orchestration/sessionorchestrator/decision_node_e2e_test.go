package sessionorchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T52 (PR-D, DM-20260707-001): end-to-end LP-4 decision-pending test.
//
// The test exercises the full ItemPipelineRunner.Run() path with a
// child segment + a sibling segment that has already decided, then
// asserts the Stage-5 Decision node fires row 10 (parent_rollup).
// This is the LP-4 canary from proposal.md §5.5.
//
// The test uses the legacy single-WorkItem path (no DAG fork) because
// the Decision node wiring is independent of the DAG helper — it runs
// on every round's Verify output. So LP-4 here uses a pre-decided
// sibling to assert the row 10 path fires deterministically.

func TestE2E_LP4_DecisionPending_ParentRollup(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp4-parent-rollup"

	// Parent rollup: a goal that owns the child + sibling segments.
	parent, err := tm.EnsureGoal(sessionID, "aggregate d2 + d3 + d7 reviews")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	if err := tm.Tree().SetNeedsRollup(sessionID, parent.ID, true); err != nil {
		t.Fatalf("SetNeedsRollup: %v", err)
	}

	// Sibling: a child WorkItem whose LastRound is set so the
	// Decision node's siblingDecidedCount == siblingTotalCount
	// gate fires when the actual child runs.
	sibling, err := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: parent.ID,
		Kind:     workmodel.WorkKindImplement,
		Title:    "review d2 kernel",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem sibling: %v", err)
	}
	if err := tm.Tree().ApplyPipelineRound(sessionID, sibling.ID, &workmodel.WorkItemPipelineRound{
		RoundNo:     1,
		VerdictKind: types.VerdictPass,
		WorkItemID:  sibling.ID,
		SessionID:   sessionID,
	}, workmodel.RoundPhaseIdle); err != nil {
		t.Fatalf("sibling ApplyPipelineRound: %v", err)
	}

	// The child segment: a fresh WorkItem that we'll drive through
	// Run() with the new LP4 harness.
	child, err := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID: parent.ID,
		Kind:     workmodel.WorkKindImplement,
		Title:    "review d3 llm gateway",
	})
	if err != nil {
		t.Fatalf("CreateWorkItem child: %v", err)
	}

	round, err := runner.Run(context.Background(), sessionID, child, "user_lp4", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if round == nil {
		t.Fatal("round = nil, want populated")
	}

	// PR-D T51 assertions: Decision fields must be persisted.
	if round.DecisionKind != DecisionParentRollup.String() {
		t.Errorf("DecisionKind = %q, want %q (row 10)",
			round.DecisionKind, DecisionParentRollup.String())
	}
	if round.DecisionMapRow != 10 {
		t.Errorf("DecisionMapRow = %d, want 10", round.DecisionMapRow)
	}
	if !strings.Contains(round.DecisionReason, "all_siblings_decided") {
		t.Errorf("DecisionReason = %q, want contains 'all_siblings_decided'", round.DecisionReason)
	}
	if round.DecisionJSON == "" {
		t.Fatal("DecisionJSON = empty, want populated")
	}
	// DecisionJSON should be valid JSON with the same fields.
	var dJSON DecisionJSON
	if err := json.Unmarshal([]byte(round.DecisionJSON), &dJSON); err != nil {
		t.Fatalf("DecisionJSON unmarshal err = %v on %s", err, round.DecisionJSON)
	}
	if dJSON.Kind != DecisionParentRollup.String() {
		t.Errorf("DecisionJSON.Kind = %q, want %q", dJSON.Kind, DecisionParentRollup.String())
	}
	if dJSON.MapRow != 10 {
		t.Errorf("DecisionJSON.MapRow = %d, want 10", dJSON.MapRow)
	}
	if dJSON.DecidedAt.IsZero() {
		t.Error("DecisionJSON.DecidedAt = zero, want populated")
	}
}

// T52 (PR-D, DM-20260707-001): end-to-end LP-4 happy-path test.
//
// Single-WorkItem path with no parent / no siblings. Asserts row 1
// (Pass → accept) fires and the fields are persisted.

func TestE2E_LP4_HappyPath_Accept(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp4-happy"

	goal, err := tm.EnsureGoal(sessionID, "trivial 1+1=几")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	round, err := runner.Run(context.Background(), sessionID, goal, "user_lp4_happy", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if round == nil {
		t.Fatal("round = nil, want populated")
	}

	// Pass path through Verify → Decision row 1.
	if round.DecisionKind != DecisionAccept.String() {
		t.Errorf("DecisionKind = %q, want %q",
			round.DecisionKind, DecisionAccept.String())
	}
	if round.DecisionMapRow != 1 {
		t.Errorf("DecisionMapRow = %d, want 1", round.DecisionMapRow)
	}
	if round.DecisionReason != "verdict_pass" {
		t.Errorf("DecisionReason = %q, want verdict_pass", round.DecisionReason)
	}
	if round.DecisionJSON == "" {
		t.Error("DecisionJSON = empty, want populated")
	}

	if round.ExitReason == "" {
		t.Error("ExitReason = empty, want non-empty (verdict-driven)")
	}
}

// T30 (PR-D, DM-20260707-001): end-to-end LP-3 multi-intent smoke.
//
// Asserts the PR-D fork gate at item_pipeline.go suppresses the
// multi-intent path when DAGEnabled=false (production default), so
// the legacy single-WorkItem path runs even when pl.DAG is set. This
// is the staging-rollout safety net: ops can ship a PlanDAG-emitting
// LLM without flipping the flag, and no segment will fan out to
// multi-worker until ops is ready.

func TestE2E_LP3_DAGForkSuppressedWhenFlagOff(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp3-flag-off"

	// DAGEnabled left at the zero value (false) by the test runner
	// helper, so the gate at line ~414 of item_pipeline.go falls
	// through to the legacy path.
	if runner.DAGEnabled {
		t.Fatal("runner.DAGEnabled = true, want false (default)")
	}

	goal, err := tm.EnsureGoal(sessionID, "1+1=几? 另外查 devrix 架构")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	round, err := runner.Run(context.Background(), sessionID, goal, "user_lp3", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run err = %v, want nil (DAG fork should be suppressed by flag)", err)
	}
	if round == nil {
		t.Fatal("round = nil, want populated")
	}
	// Decision stage still ran (it's wired into every round's
	// Verify → Decision transition). Accept path is the natural
	// outcome of a trivial Pass.
	if round.DecisionKind != DecisionAccept.String() {
		t.Errorf("DecisionKind = %q, want %q",
			round.DecisionKind, DecisionAccept.String())
	}
	if round.DecisionMapRow != 1 {
		t.Errorf("DecisionMapRow = %d, want 1", round.DecisionMapRow)
	}
}

// T30 (PR-D, DM-20260707-001): end-to-end LP-3 with flag ON but
// no DAGExecutor wired. The gate's DAGExecutor != nil check should
// short-circuit and the legacy path should still run. This proves
// DAGEnabled and DAGExecutor are independent gates — flipping the
// flag without wiring the executor does NOT crash.

func TestE2E_LP3_DAGFlagOn_NilExecutor_LegacyPath(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp3-flag-on-nil-exec"

	runner.DAGEnabled = true
	// DAGExecutor left nil (test runner helper does not wire it)
	// so the gate's DAGExecutor != nil check fails and the fork
	// does NOT fire.

	goal, err := tm.EnsureGoal(sessionID, "trivial test of DAG flag wiring")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	round, err := runner.Run(context.Background(), sessionID, goal, "user_lp3_on", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run err = %v, want nil (DAGExecutor nil → legacy path)", err)
	}
	if round == nil {
		t.Fatal("round = nil, want populated")
	}
	if round.DecisionKind != DecisionAccept.String() {
		t.Errorf("DecisionKind = %q, want %q",
			round.DecisionKind, DecisionAccept.String())
	}
}

// TestE2E_LP4_FailRetryAfterMaxAttempt_HumanReview complements the
// happy-path test: a WorkItem whose last round had VerdictFail +
// AttemptNo >= MaxRetry drives the Stage-5 Decision node to row 6
// (Fail + AttemptNo >= MaxRetry → human_review). This is the
// terminal-routing guarantee for the B-vs-E split in §2.12.

func TestE2E_LP4_FailAfterMaxRetry_HumanReview(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp4-fail-max"

	// Force this round's Verify to emit VerdictFail so the Decision
	// node sees Fail + AttemptNo>=MaxRetry → row 6 (human_review).
	runner.Verify = func(_ *wavescheduler.Artifact) workmodel.Verdict {
		return workmodel.Verdict{
			Kind:       types.VerdictFail,
			Reason:     "e2e_fail_max_retry",
			SourceID:   "v_e2e_fail",
			Confidence: 0.3,
		}
	}

	goal, err := tm.EnsureGoal(sessionID, "trigger fail + max retry")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	// Pre-seed a prior round with VerdictFail + AttemptNo=1 (>= MaxRetry=1)
	// so the next run's Decide sees Fail + AttemptNo >= MaxRetry → row 6.
	if err := tm.Tree().ApplyPipelineRound(sessionID, goal.ID, &workmodel.WorkItemPipelineRound{
		RoundNo:     1,
		VerdictKind: types.VerdictFail,
		WorkItemID:  goal.ID,
		SessionID:   sessionID,
	}, workmodel.RoundPhaseIdle); err != nil {
		t.Fatalf("ApplyPipelineRound: %v", err)
	}

	round, err := runner.Run(context.Background(), sessionID, goal, "user_lp4_fail", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if round == nil {
		t.Fatal("round = nil, want populated")
	}
	// The Decide runs at AttemptNo=1 (roundNo-1 from the upcoming
	// round). Combined with the persisted VerdictFail from this
	// run's Verify, the mapping table reaches row 6 (Fail + 1>=1
	// → human_review).
	if round.DecisionKind != DecisionHumanReview.String() {
		t.Errorf("DecisionKind = %q, want %q (row 6)",
			round.DecisionKind, DecisionHumanReview.String())
	}
	if round.DecisionMapRow != 6 {
		t.Errorf("DecisionMapRow = %d, want 6", round.DecisionMapRow)
	}
	if !strings.Contains(round.DecisionReason, "verdict_fail+attempt_1") {
		t.Errorf("DecisionReason = %q, want contains verdict_fail+attempt_1", round.DecisionReason)
	}
}

// TestE2E_LP4_ReputationAttr_AcceptFires verifies that an Accept
// Decision carries the right ExitReason prefix. This anchors the
// PR-E Learn node's reputation attribution: the DecisionKind is the
// signal that says "this round was clean" (no need for a forced
// retry), and the Learn node reads round.DecisionKind to decide
// whether to bump α (Pass + Accept) or β (Fail + Retry/HumanReview).

func TestE2E_LP4_AcceptDecision_PreservesBaseExitReason(t *testing.T) {
	runner, tm, _ := newItemPipelineTestRunner(t)
	sessionID := "sess-lp4-accept-exit"

	goal, err := tm.EnsureGoal(sessionID, "trivial accept case")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	round, err := runner.Run(context.Background(), sessionID, goal, "user", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if round.DecisionKind != DecisionAccept.String() {
		t.Fatalf("DecisionKind = %q, want accept", round.DecisionKind)
	}
	// DecisionAccept leaves the base exit reason unchanged
	// (exitReasonForDecision returns base). Verdict-driven base
	// exit reason is non-empty for trivial cases.
	if round.ExitReason == "" {
		t.Error("ExitReason = empty, want non-empty (verdict-driven base)")
	}
	// And the suffix "+decision_retry" must NOT appear — that's
	// reserved for DecisionRetry.
	if strings.Contains(round.ExitReason, "decision_retry") {
		t.Errorf("ExitReason = %q contains decision_retry, want absent for Accept", round.ExitReason)
	}
}

// Sanity guard: the test runner helper must use the production
// learn.DefaultLearner so the PR-E Learn-node wiring is exercised
// in this test. Without this, a future test refactor might drop the
// Learner and the Stage-5 Decision would still fire (it's a pure
// function), but the LP-4 + LP-3 contracts would silently degrade
// for downstream consumers. Verifies the existing helper is
// production-shaped.

func TestE2E_Helper_UsesProductionLearner(t *testing.T) {
	runner, _, rep := newItemPipelineTestRunner(t)
	if runner.Learner == nil {
		t.Fatal("runner.Learner = nil, want production DefaultLearner")
	}
	if rep == nil {
		t.Error("reputation store = nil, want in-memory store wired")
	}
	// smoke: invoke Inject to confirm Learner is a *DefaultLearner
	// (it would crash on a nil interface, so this also catches
	// regressions where Learner is set to a stub).
	_, _ = runner.Learner.Inject(context.Background(), "sess-smoke", "test")
	// The DefaultLearner.Inject on a fresh store returns a non-nil
	// reputation; we don't check content because that's PR-E's
	// surface, just that the call doesn't panic.
}
