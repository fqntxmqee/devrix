// Package workmodel — spawn_budget_gate_test.go
//
// Phase 4 tests (D7-S15-A109-T01..T02) for DecomposeFromSubWorktree +
// budget gate degradation.
//
// Coverage:
//   - SubWorktree field plumbing (interfaces → UnresolvedObs → ChildSpec)
//   - buildChildSpecsFromSubWorktrees: full spec vs fallback
//   - composeSubWorktreeDirective: header + suffix
//   - ApplySpawnPolicy RC-4a graceful degradation on budget errors
//     (depth / children / daily) → SpawnInline downgrade
//   - ApplySpawnPolicy: non-budget errors still propagate
//
// Together these prove that an RC-4a-driven SpawnDecompose hitting a
// budget limit falls back to inline retry (matches legacy Phase 2
// `execution_mode: "decompose"` observable behavior) instead of
// aborting the session loop with a non-nil error.

package workmodel

import (
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// SubWorktree field plumbing (T01..T03)
// -----------------------------------------------------------------------------

// SubWorktree spec carried through the report (Phase 4 wire-format extension).
func TestUnresolvedObs_SubWorktreeCarriedThroughReport(t *testing.T) {
	sub := &interfaces.SubWorktreeSpec{
		Title:           "Investigate cross-cutting edge case",
		DirectiveSuffix: "Inspect the GetPipelineFocus path; reason about session focus starvation.",
		ExpectedReturn:  "Diagnose root cause and propose a fix in a follow-up Plan.",
		ScopeIn:         []string{"internal/layers/orchestration/workmodel/work_tree.go"},
		PlannedTool:     "read_file",
	}
	strategy, err := interfaces.NewResolutionStrategy("obs_42", "read_file", "exit_code==0")
	if err != nil {
		t.Fatalf("NewResolutionStrategy: %v", err)
	}
	strategy, err = strategy.WithSubWorktree(sub)
	if err != nil {
		t.Fatalf("WithSubWorktree: %v", err)
	}
	report, err := interfaces.NewResolutionReport("s1", "wi_test", 1, []interfaces.ResolutionStrategy{strategy}, nil)
	if err != nil {
		t.Fatalf("NewResolutionReport: %v", err)
	}
	if len(report.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(report.UnresolvedObs))
	}
	uo := report.UnresolvedObs[0]
	if !uo.HasSubWorktree {
		t.Errorf("HasSubWorktree = false, want true")
	}
	if uo.SubWorktree == nil {
		t.Fatalf("SubWorktree nil; want carried through")
	}
	if uo.SubWorktree.Title != sub.Title {
		t.Errorf("Title = %q, want %q", uo.SubWorktree.Title, sub.Title)
	}
	if uo.SubWorktree.DirectiveSuffix != sub.DirectiveSuffix {
		t.Errorf("DirectiveSuffix mismatch")
	}
	if uo.SubWorktree.ExpectedReturn != sub.ExpectedReturn {
		t.Errorf("ExpectedReturn mismatch")
	}
	if len(uo.SubWorktree.ScopeIn) != 1 || uo.SubWorktree.ScopeIn[0] != sub.ScopeIn[0] {
		t.Errorf("ScopeIn mismatch: %v", uo.SubWorktree.ScopeIn)
	}
}

// buildChildSpecsFromSubWorktrees: full spec path lifts Title/DirectiveSuffix.
func TestBuildChildSpecsFromSubWorktrees_FullSpec(t *testing.T) {
	in := []interfaces.UnresolvedObs{
		{
			ObsID:    "obs_a",
			Strength: 0.9,
			Reason:   interfaces.ResolutionReasonNoClaim,
			HasSubWorktree: true,
			SubWorktree: &interfaces.SubWorktreeSpec{
				Title:           "Investigate A",
				DirectiveSuffix: "Focus on the verify path.",
				ExpectedReturn:  "Deliver a fix.",
				ScopeIn:         []string{"internal/foo/"},
			},
		},
	}
	got := buildChildSpecsFromSubWorktrees(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	cs := got[0]
	if cs.Title != "Investigate A" {
		t.Errorf("Title = %q, want %q", cs.Title, "Investigate A")
	}
	if !strings.Contains(cs.Directive, "Focus on the verify path.") {
		t.Errorf("Directive missing suffix: %q", cs.Directive)
	}
	if !strings.Contains(cs.Directive, "obs_a") {
		t.Errorf("Directive missing ObsID anchor: %q", cs.Directive)
	}
	if cs.ExpectedReturn != "Deliver a fix." {
		t.Errorf("ExpectedReturn = %q, want %q", cs.ExpectedReturn, "Deliver a fix.")
	}
	if len(cs.ScopeIn) != 1 {
		t.Errorf("ScopeIn = %v, want 1 entry", cs.ScopeIn)
	}
}

// buildChildSpecsFromSubWorktrees: fallback when SubWorktree is nil.
func TestBuildChildSpecsFromSubWorktrees_FallbackWhenSpecNil(t *testing.T) {
	in := []interfaces.UnresolvedObs{
		{ObsID: "obs_a", Strength: 0.5, Reason: interfaces.ResolutionReasonNoClaim, HasSubWorktree: true},
	}
	got := buildChildSpecsFromSubWorktrees(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Title != "Resolve unresolved ObsID obs_a" {
		t.Errorf("fallback Title = %q, want placeholder", got[0].Title)
	}
}

// buildChildSpecsFromSubWorktrees: fallback when Title is empty.
func TestBuildChildSpecsFromSubWorktrees_FallbackWhenTitleEmpty(t *testing.T) {
	in := []interfaces.UnresolvedObs{
		{
			ObsID:          "obs_a",
			HasSubWorktree: true,
			SubWorktree:    &interfaces.SubWorktreeSpec{Title: ""}, // invalid; fallback
		},
	}
	got := buildChildSpecsFromSubWorktrees(in)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !strings.Contains(got[0].Title, "obs_a") {
		t.Errorf("Title should fall back to ObsID-based placeholder: %q", got[0].Title)
	}
}

// -----------------------------------------------------------------------------
// composeSubWorktreeDirective (T04)
// -----------------------------------------------------------------------------

func TestComposeSubWorktreeDirective_WithSuffix(t *testing.T) {
	uo := interfaces.UnresolvedObs{
		ObsID:    "obs_a",
		Strength: 0.95,
		Reason:   interfaces.ResolutionReasonLowConfidence,
		SubWorktree: &interfaces.SubWorktreeSpec{
			DirectiveSuffix: "Inspect foo.go line 42.",
		},
	}
	got := composeSubWorktreeDirective(uo)
	if !strings.Contains(got, "Inspect foo.go line 42.") {
		t.Errorf("missing suffix: %q", got)
	}
	if !strings.Contains(got, "obs_a") {
		t.Errorf("missing ObsID header: %q", got)
	}
}

func TestComposeSubWorktreeDirective_NoSuffix(t *testing.T) {
	uo := interfaces.UnresolvedObs{
		ObsID:    "obs_a",
		Strength: 0.5,
		Reason:   interfaces.ResolutionReasonNoClaim,
		SubWorktree: &interfaces.SubWorktreeSpec{
			Title: "x",
		},
	}
	got := composeSubWorktreeDirective(uo)
	if got == "" {
		t.Fatalf("got empty directive")
	}
	if strings.Count(got, "\n\n") > 0 {
		t.Errorf("no-suffix directive should not have \\n\\n separator: %q", got)
	}
}

func TestComposeSubWorktreeDirective_NilSpec(t *testing.T) {
	uo := interfaces.UnresolvedObs{
		ObsID:    "obs_a",
		Strength: 0.5,
		Reason:   interfaces.ResolutionReasonNoClaim,
	}
	got := composeSubWorktreeDirective(uo)
	if !strings.Contains(got, "obs_a") {
		t.Errorf("header missing: %q", got)
	}
}

// -----------------------------------------------------------------------------
// ApplySpawnPolicy RC-4a graceful degradation (T05..T07)
// -----------------------------------------------------------------------------

// Budget gate: ErrDecomposeDepthExceeded → SpawnInline (no abort).
func TestApplySpawnPolicy_RC4a_DepthGateDegradesToInline(t *testing.T) {
	ResetDecomposeLimits()
	tm := NewTaskManager()
	tree := tm.Tree()
	tree.maxDecomposeDepth = 2
	goal, _ := tm.EnsureGoal("s1", "g")
	l1, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "l1", Directive: "l1"})
	l2, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: l1.ID, Kind: WorkKindImplement, Title: "l2", Directive: "l2"})
	// Round ready; force SpawnDecompose + RC-4a-style ChildSpecs.
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.5)
	round.SpawnPolicy = SpawnDecompose
	round.ChildSpecs = []ChildSpec{{Title: "too deep", Directive: "x", ExpectedReturn: "deliver x"}}
	if err := ApplySpawnPolicy("s1", l2, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy should degrade (return nil) on depth gate; got %v", err)
	}
	if round.SpawnPolicy != SpawnInline {
		t.Fatalf("SpawnPolicy = %q, want SpawnInline after depth gate", round.SpawnPolicy)
	}
	if !strings.Contains(round.SpawnRationale, "budget gate") {
		t.Errorf("rationale missing budget gate marker: %q", round.SpawnRationale)
	}
	if !strings.Contains(round.SpawnRationale, ErrDecomposeDepthExceeded.Error()) {
		t.Errorf("rationale missing depth error text: %q", round.SpawnRationale)
	}
}

// Budget gate: ErrTooManyChildren → SpawnInline.
func TestApplySpawnPolicy_RC4a_TooManyChildrenDegradesToInline(t *testing.T) {
	ResetDecomposeLimits()
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "p", Directive: "p"})
	// Fill to DefaultMaxChildren with real children.
	for i := 0; i < DefaultMaxChildren; i++ {
		_, _ = tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindImplement, Title: "existing", Directive: "x"})
	}
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.5)
	round.SpawnPolicy = SpawnDecompose
	round.ChildSpecs = []ChildSpec{{Title: "one more", Directive: "x", ExpectedReturn: "deliver x"}}
	if err := ApplySpawnPolicy("s1", parent, round, tm); err != nil {
		t.Fatalf("ApplySpawnPolicy should degrade on too-many-children; got %v", err)
	}
	if round.SpawnPolicy != SpawnInline {
		t.Fatalf("SpawnPolicy = %q, want SpawnInline", round.SpawnPolicy)
	}
	if !strings.Contains(round.SpawnRationale, "budget gate") {
		t.Errorf("rationale missing budget gate marker: %q", round.SpawnRationale)
	}
}

// Non-budget errors still propagate (parent not found, etc.).
func TestApplySpawnPolicy_RC4a_NonBudgetErrorPropagates(t *testing.T) {
	ResetDecomposeLimits()
	tm := NewTaskManager()
	_, _ = tm.EnsureGoal("s1", "g")
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.5)
	round.SpawnPolicy = SpawnDecompose
	// Force a non-budget error: bogus itemID has no parent, so
	// DecomposeChildren returns "parent not found".
	bogus := &WorkItem{ID: "wi_does_not_exist", Kind: WorkKindPlan, Title: "bogus"}
	round.ChildSpecs = []ChildSpec{{Title: "c", Directive: "x", ExpectedReturn: "deliver x"}}
	err := ApplySpawnPolicy("s1", bogus, round, tm)
	if err == nil {
		t.Fatalf("expected non-nil error for unknown parent; got nil")
	}
	if round.SpawnPolicy != SpawnDecompose {
		t.Errorf("SpawnPolicy = %q, want SpawnDecompose (unchanged on non-budget error)", round.SpawnPolicy)
	}
}

// isBudgetGateError classification unit test.
func TestIsBudgetGateError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{ErrDecomposeDepthExceeded, true},
		{ErrTooManyChildren, true},
		{ErrDecomposeDailyLimit, true},
		{errors.New("other"), false},
		{errors.New("max decompose depth exceeded"), false}, // text-similar but not errors.Is match
	}
	for _, tc := range tests {
		got := isBudgetGateError(tc.err)
		if got != tc.want {
			t.Errorf("isBudgetGateError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// SpawnPolicyEvaluator end-to-end with RC-4a + budget (T08)
// -----------------------------------------------------------------------------

// When the round has AnySubWorktreePending AND the parent is at max depth,
// checkBudget (R1) wins → SpawnInline (NOT SpawnDecompose). This is the
// sub-decision order guarantee: budget is hard cap, RC-4a is override only.
func TestSpawnPolicyEvaluator_BudgetGateBeforeRC4a(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_a", Strength: 0.95, HasSubWorktree: true},
		},
	}
	ctx := baseCtx()
	ctx.Depth = 3
	ctx.MaxDepth = 3
	got := SpawnPolicyEvaluator(round, ctx)
	if got == SpawnDecompose {
		t.Fatalf("budget gate must win over RC-4a; got SpawnDecompose at depth=3")
	}
	if got != SpawnInline && got != SpawnEscalateHuman {
		t.Fatalf("got %q, want inline/escalate (R1 max depth)", got)
	}
}