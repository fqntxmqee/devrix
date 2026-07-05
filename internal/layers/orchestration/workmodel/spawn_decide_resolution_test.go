// Package workmodel — spawn_decide_resolution_test.go
//
// Unit tests for the 4th sub-decision checkResolutionReport
// (DM-20260704-006 RC-4a/b/c):
//
//   - RC-4a: AnySubWorktreePending → SpawnDecompose (overrides verdict direction)
//   - RC-4b: MaxUnresolvedStrength >= threshold (no SubWorktree) → SpawnUserGate
//   - RC-4c: else → fall through (SpawnNone, false)
//
// Plus the 4 state-table behavior of checkResolutionReport input space:
//   - nil round / nil report → fall through
//   - empty UnresolvedObs → fall through (RC-4c, fully covered)
//   - low-strength only (Strength < threshold, no SubWorktree) → fall through
//   - SubWorktree + low strength → RC-4a wins (SubWorktree priority)
//   - high strength without SubWorktree → RC-4b
//   - high strength with SubWorktree → RC-4a wins (SubWorktree priority)
//   - SpawnUserGate falls through when SubWorktree is present (atomic: only one of a/b)
//   - order vs. checkBudget: depth limit wins, RC-4a is bypassed (DecomposeChildren would fail)
//
// And the SpawnPolicyEvaluator integration test verifying the 4-step chain:
//
//	checkBudget → checkResolutionReport → checkRollupGuard → checkVerdictDirection

package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/shared/types"
)

// -----------------------------------------------------------------------------
// DefaultUnresolvedStrengthThreshold mirror-constant test (T01)
// -----------------------------------------------------------------------------

func TestDefaultUnresolvedStrengthThreshold_Matches(t *testing.T) {
	if DefaultUnresolvedStrengthThreshold != interfaces.DefaultUnresolvedStrengthThreshold {
		t.Fatalf("DefaultUnresolvedStrengthThreshold = %f, want %f (mirror of interfaces constant)",
			DefaultUnresolvedStrengthThreshold, interfaces.DefaultUnresolvedStrengthThreshold)
	}
}

// -----------------------------------------------------------------------------
// checkResolutionReport unit tests (T02..T07)
// -----------------------------------------------------------------------------

func TestCheckResolutionReport_NilRound(t *testing.T) {
	got, fired := checkResolutionReport(nil, baseCtx())
	if got != SpawnNone || fired {
		t.Fatalf("nil round: got (policy=%q, fired=%v), want (SpawnNone, false)", got, fired)
	}
}

func TestCheckResolutionReport_NilReport(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	round.ResolutionReport = nil
	got, fired := checkResolutionReport(round, baseCtx())
	if got != SpawnNone || fired {
		t.Fatalf("nil report: got (policy=%q, fired=%v), want (SpawnNone, false)", got, fired)
	}
}

func TestCheckResolutionReport_EmptyReport_FallThrough(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.5)
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
	}
	got, fired := checkResolutionReport(round, baseCtx())
	if got != SpawnNone || fired {
		t.Fatalf("empty report: got (policy=%q, fired=%v), want (SpawnNone, false)", got, fired)
	}
}

// RC-4a — SubWorktree pending wins regardless of strength.
func TestCheckResolutionReport_RC4a_AnySubWorktreePending(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2) // Pass + low U would normally SpawnNone
	// Build a SubWorktree spec manually so the test does not depend on
	// the constructor's input validation (the constructor only checks
	// ObsID non-empty).
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_42", Strength: 0.3, Reason: interfaces.ResolutionReasonNoClaim, HasSubWorktree: true},
		},
	}
	got, fired := checkResolutionReport(round, baseCtx())
	if got != SpawnDecompose || !fired {
		t.Fatalf("RC-4a: got (policy=%q, fired=%v), want (SpawnDecompose, true)", got, fired)
	}
	if len(round.ChildSpecs) != 1 {
		t.Fatalf("RC-4a: round.ChildSpecs = %d, want 1 (built from SubWorktree)", len(round.ChildSpecs))
	}
	if round.ChildSpecs[0].Directive == "" {
		t.Errorf("RC-4a: ChildSpec.Directive empty; need ObsID hint for child LLM")
	}
}

// RC-4b — high strength, no SubWorktree → SpawnUserGate.
func TestCheckResolutionReport_RC4b_HighStrength_NoSubWorktree(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2) // low U normally SpawnNone
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_42", Strength: 0.95, Reason: interfaces.ResolutionReasonLowConfidence, HasSubWorktree: false},
		},
	}
	got, fired := checkResolutionReport(round, baseCtx())
	if got != SpawnUserGate || !fired {
		t.Fatalf("RC-4b: got (policy=%q, fired=%v), want (SpawnUserGate, true)", got, fired)
	}
	if len(round.ChildSpecs) != 0 {
		t.Errorf("RC-4b: round.ChildSpecs = %d, want 0 (user gate does not decompose)", len(round.ChildSpecs))
	}
}

// RC-4a precedence over RC-4b — SubWorktree + high strength still RC-4a.
func TestCheckResolutionReport_RC4a_WinsOverRC4b(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_a", Strength: 0.95, HasSubWorktree: true},
			{ObsID: "obs_b", Strength: 0.95, HasSubWorktree: false},
		},
	}
	got, fired := checkResolutionReport(round, baseCtx())
	if got != SpawnDecompose || !fired {
		t.Fatalf("RC-4a wins: got (policy=%q, fired=%v), want (SpawnDecompose, true)", got, fired)
	}
	if len(round.ChildSpecs) != 1 {
		t.Errorf("RC-4a wins: round.ChildSpecs = %d, want 1 (only HasSubWorktree=true emitted)", len(round.ChildSpecs))
	}
}

// RC-4c — low strength, no SubWorktree → fall through.
func TestCheckResolutionReport_RC4c_LowStrength_NoSubWorktree(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2)
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_x", Strength: 0.5, Reason: interfaces.ResolutionReasonLowConfidence, HasSubWorktree: false},
		},
	}
	got, fired := checkResolutionReport(round, baseCtx())
	if got != SpawnNone || fired {
		t.Fatalf("RC-4c: got (policy=%q, fired=%v), want (SpawnNone, false)", got, fired)
	}
}

// -----------------------------------------------------------------------------
// SpawnPolicyEvaluator integration: 4-step chain (T08..T11)
// -----------------------------------------------------------------------------

// checkBudget (R1 max depth) wins over checkResolutionReport (RC-4a).
func TestSpawnPolicyEvaluator_BudgetWinsOverRC4a(t *testing.T) {
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
	// Even though RC-4a fires, R1 max-depth budget forces SpawnInline /
	// SpawnEscalateHuman (DecomposeChildren would fail at depth limit).
	got := SpawnPolicyEvaluator(round, ctx)
	if got == SpawnDecompose {
		t.Fatalf("budget must win over RC-4a: got SpawnDecompose at depth=3, want inline/escalate")
	}
}

// checkResolutionReport (RC-4a) wins over checkVerdictDirection (R3 Pass → SpawnNone).
func TestSpawnPolicyEvaluator_RC4a_OverridesPassNone(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_a", Strength: 0.95, HasSubWorktree: true},
		},
	}
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnDecompose {
		t.Fatalf("RC-4a overrides R3 Pass: got %q, want SpawnDecompose", got)
	}
}

// checkResolutionReport (RC-4b) wins over checkVerdictDirection.
func TestSpawnPolicyEvaluator_RC4b_OverridesVerdictDirection(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.2) // low U → SpawnNone
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_a", Strength: 0.95, HasSubWorktree: false},
		},
	}
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnUserGate {
		t.Fatalf("RC-4b overrides R5 low U: got %q, want SpawnUserGate", got)
	}
}

// checkVerdictDirection runs as default when checkResolutionReport fires nothing.
func TestSpawnPolicyEvaluator_ResolutionReportInactive_VerdictDirectionWins(t *testing.T) {
	round := baseRound(types.VerdictPass, plan.CommitmentPlan, 0.2)
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		// empty UnresolvedObs → RC-4a/b not fired → fall through
	}
	if got := SpawnPolicyEvaluator(round, baseCtx()); got != SpawnNone {
		t.Fatalf("empty report → verdict direction: got %q, want SpawnNone (R3 Pass)", got)
	}
}

// -----------------------------------------------------------------------------
// spawnRationale test — RC-4a/b branches (T12)
// -----------------------------------------------------------------------------

func TestSpawnRationale_RC4a_IncludesSubWorktreeCount(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.5)
	round.SpawnPolicy = SpawnDecompose
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_a", HasSubWorktree: true},
		},
	}
	rationale := spawnRationale(SpawnDecompose, round, baseCtx())
	if !contains(rationale, "RC-4a") {
		t.Fatalf("rationale missing RC-4a marker: %q", rationale)
	}
	if !contains(rationale, "n=1") {
		t.Fatalf("rationale missing unresolved count: %q", rationale)
	}
}

func TestSpawnRationale_RC4b_IncludesStrengthAndThreshold(t *testing.T) {
	round := baseRound(types.VerdictPartial, plan.CommitmentPlan, 0.5)
	round.SpawnPolicy = SpawnUserGate
	round.ResolutionReport = &interfaces.ResolutionReport{
		SessionID:  "s1",
		WorkItemID: "wi_test",
		RoundNo:    1,
		UnresolvedObs: []interfaces.UnresolvedObs{
			{ObsID: "obs_a", Strength: 0.95, HasSubWorktree: false},
		},
	}
	rationale := spawnRationale(SpawnUserGate, round, baseCtx())
	if !contains(rationale, "RC-4b") {
		t.Fatalf("rationale missing RC-4b marker: %q", rationale)
	}
	if !contains(rationale, "0.950") {
		t.Fatalf("rationale missing strength: %q", rationale)
	}
	if !contains(rationale, "0.850") {
		t.Fatalf("rationale missing threshold: %q", rationale)
	}
}

// contains is a small substring helper to avoid pulling in strings.Contains
// (which would also work; using a local helper keeps the test file focused
// on the units under test).
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// buildChildSpecsFromSubWorktrees unit test (T13)
// -----------------------------------------------------------------------------

func TestBuildChildSpecsFromSubWorktrees_EmitsOnlyFlagged(t *testing.T) {
	in := []interfaces.UnresolvedObs{
		{ObsID: "obs_a", Strength: 0.9, HasSubWorktree: true},
		{ObsID: "obs_b", Strength: 0.5, HasSubWorktree: false}, // skipped
		{ObsID: "obs_c", Strength: 0.95, HasSubWorktree: true},
	}
	got := buildChildSpecsFromSubWorktrees(in)
	if len(got) != 2 {
		t.Fatalf("ChildSpecs len = %d, want 2 (only HasSubWorktree=true)", len(got))
	}
	if got[0].Title == "" || got[0].Directive == "" {
		t.Errorf("ChildSpec[0] missing title/directive: %+v", got[0])
	}
	if !contains(got[0].Title, "obs_a") {
		t.Errorf("ChildSpec[0].Title = %q, want contains obs_a", got[0].Title)
	}
}

func TestBuildChildSpecsFromSubWorktrees_Empty(t *testing.T) {
	got := buildChildSpecsFromSubWorktrees(nil)
	if len(got) != 0 {
		t.Fatalf("nil input: got %d specs, want 0", len(got))
	}
	got = buildChildSpecsFromSubWorktrees([]interfaces.UnresolvedObs{})
	if len(got) != 0 {
		t.Fatalf("empty input: got %d specs, want 0", len(got))
	}
}

// -----------------------------------------------------------------------------
// buildUserGateObsList test (T14) — covers SpawnApply's SpawnUserGate path
// -----------------------------------------------------------------------------

func TestBuildUserGateObsList_Empty(t *testing.T) {
	got := buildUserGateObsList(nil)
	if got != "  (none)" {
		t.Fatalf("empty: got %q, want %q", got, "  (none)")
	}
}

func TestBuildUserGateObsList_FormatsEntries(t *testing.T) {
	in := []interfaces.UnresolvedObs{
		{ObsID: "obs_42", Strength: 0.95, Reason: interfaces.ResolutionReasonLowConfidence},
	}
	got := buildUserGateObsList(in)
	if !contains(got, "obs_42") {
		t.Errorf("missing ObsID: %q", got)
	}
	if !contains(got, "0.950") {
		t.Errorf("missing strength: %q", got)
	}
	if !contains(got, "low_confidence") {
		t.Errorf("missing reason: %q", got)
	}
}

func TestBuildUserGateObsList_Truncates(t *testing.T) {
	in := make([]interfaces.UnresolvedObs, 15)
	for i := range in {
		in[i] = interfaces.UnresolvedObs{ObsID: "obs_" + string(rune('a'+i)), Strength: 0.5}
	}
	got := buildUserGateObsList(in)
	if !contains(got, "+5 more") {
		t.Errorf("missing truncation marker: %q", got)
	}
}