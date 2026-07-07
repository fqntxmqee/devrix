package sessionorchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// harnessRet is a tiny per-test fixture that runs Decide and captures
// the (Decision, error) tuple so each sub-test reads cleanly.
type harnessRet struct {
	Decision Decision
	Err      error
}

func runDecide(t *testing.T, ctx DecisionContext) harnessRet {
	t.Helper()
	node := NewStaticDecisionNode()
	d, err := node.Decide(ctx)
	return harnessRet{Decision: d, Err: err}
}

func runDecideWithRetry(t *testing.T, ctx DecisionContext, maxRetry int) harnessRet {
	t.Helper()
	node := NewStaticDecisionNodeWithMaxRetry(maxRetry)
	d, err := node.Decide(ctx)
	return harnessRet{Decision: d, Err: err}
}

// TestDecision_KindStringAllFive locks the wire format for all 5
// DecisionKind values. D5 dashboards grep on these strings; renaming
// them is a breaking change.
func TestDecision_KindStringAllFive(t *testing.T) {
	cases := []struct {
		kind DecisionKind
		want string
	}{
		{DecisionAccept, "accept"},
		{DecisionRetry, "retry"},
		{DecisionChildWorker, "child_worker"},
		{DecisionParentRollup, "parent_rollup"},
		{DecisionHumanReview, "human_review"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("DecisionKind(%d).String() = %q, want %q", uint8(c.kind), got, c.want)
		}
		if err := c.kind.Validate(); err != nil {
			t.Errorf("DecisionKind(%d).Validate() = %v, want nil", uint8(c.kind), err)
		}
	}
}

// TestDecision_InvalidKindRejects verifies Validate catches unknown
// enum values so a typo'd field doesn't slip into round persistence.
func TestDecision_InvalidKindRejects(t *testing.T) {
	bad := DecisionKind(99)
	if err := bad.Validate(); err == nil {
		t.Fatal("Validate() returned nil for DecisionKind(99), want error")
	}
	if !strings.Contains(bad.String(), "DecisionKind(99)") {
		t.Fatalf("bad.String() = %q, want debug-format fallback", bad.String())
	}
}

// --- 11-row mapping table coverage ---

// Row 1: Pass + (default) → accept.
func TestDecision_Row1_PassDefault_Accept(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta: RoundMeta{AttemptNo: 0, RiskLevel: "normal"},
		VerdictKind: uint8(types.VerdictPass),
	})
	if r.Err != nil {
		t.Fatalf("Decide err = %v, want nil", r.Err)
	}
	if r.Decision.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept", r.Decision.Kind)
	}
	if r.Decision.MapRow != 1 {
		t.Errorf("MapRow = %d, want 1", r.Decision.MapRow)
	}
}

// Row 2: Partial + Tolerance=high → accept.
func TestDecision_Row2a_PartialToleranceHigh_Accept(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta:   RoundMeta{AttemptNo: 0, ToleranceHint: "high", ChildBudgetRemaining: 2},
		VerdictKind:  uint8(types.VerdictPartial),
	})
	if r.Decision.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept (row 2a)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 2 {
		t.Errorf("MapRow = %d, want 2", r.Decision.MapRow)
	}
}

// Row 2: Partial + ChildBudget=0 → accept.
func TestDecision_Row2b_PartialChildBudgetZero_Accept(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta:   RoundMeta{AttemptNo: 0, ChildBudgetRemaining: 0},
		VerdictKind:  uint8(types.VerdictPartial),
	})
	if r.Decision.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept (row 2b)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 2 {
		t.Errorf("MapRow = %d, want 2", r.Decision.MapRow)
	}
}

// Row 3: Partial + DecomposableAC + ChildBudget>0 + ACSubset → child_worker.
func TestDecision_Row3_PartialDecomposable_ChildWorker(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta: RoundMeta{
			AttemptNo:            0,
			ChildBudgetRemaining: 2,
			HasDecomposableAC:    true,
		},
		VerdictKind: uint8(types.VerdictPartial),
		ACSubset:    []string{"AC-1", "AC-2"},
	})
	if r.Decision.Kind != DecisionChildWorker {
		t.Errorf("Kind = %s, want child_worker", r.Decision.Kind)
	}
	if r.Decision.MapRow != 3 {
		t.Errorf("MapRow = %d, want 3", r.Decision.MapRow)
	}
	if r.Decision.NextWorkItemSpec == nil {
		t.Fatal("NextWorkItemSpec = nil, want populated")
	}
	if got := r.Decision.NextWorkItemSpec.SubSegmentIDs; len(got) != 2 || got[0] != "AC-1" {
		t.Errorf("SubSegmentIDs = %v, want [AC-1 AC-2]", got)
	}
}

// Row 4: Partial + (other, no decomposition, no high tolerance) → accept.
func TestDecision_Row4_PartialFallback_Accept(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta: RoundMeta{
			AttemptNo:            0,
			ChildBudgetRemaining: 2,
			HasDecomposableAC:    false,
		},
		VerdictKind: uint8(types.VerdictPartial),
	})
	if r.Decision.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept (row 4 fallback)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 4 {
		t.Errorf("MapRow = %d, want 4", r.Decision.MapRow)
	}
}

// Row 5: Fail + AttemptNo (0) < MaxRetry (1) → retry.
func TestDecision_Row5_FailBelowMaxRetry_Retry(t *testing.T) {
	r := runDecideWithRetry(t, DecisionContext{
		RoundMeta:  RoundMeta{AttemptNo: 0},
		VerdictKind: uint8(types.VerdictFail),
	}, 1)
	if r.Decision.Kind != DecisionRetry {
		t.Errorf("Kind = %s, want retry", r.Decision.Kind)
	}
	if r.Decision.MapRow != 5 {
		t.Errorf("MapRow = %d, want 5", r.Decision.MapRow)
	}
}

// Row 6: Fail + AttemptNo (1) >= MaxRetry (1) → human_review.
func TestDecision_Row6_FailAtOrAboveMaxRetry_HumanReview(t *testing.T) {
	r := runDecideWithRetry(t, DecisionContext{
		RoundMeta:  RoundMeta{AttemptNo: 1},
		VerdictKind: uint8(types.VerdictFail),
	}, 1)
	if r.Decision.Kind != DecisionHumanReview {
		t.Errorf("Kind = %s, want human_review", r.Decision.Kind)
	}
	if r.Decision.MapRow != 6 {
		t.Errorf("MapRow = %d, want 6", r.Decision.MapRow)
	}
}

// Row 7: Indeterminate + RiskLevel=high → human_review.
func TestDecision_Row7_IndeterminateRiskHigh_HumanReview(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta:  RoundMeta{AttemptNo: 0, RiskLevel: "high"},
		VerdictKind: uint8(types.VerdictIndeterminate),
	})
	if r.Decision.Kind != DecisionHumanReview {
		t.Errorf("Kind = %s, want human_review", r.Decision.Kind)
	}
	if r.Decision.MapRow != 7 {
		t.Errorf("MapRow = %d, want 7", r.Decision.MapRow)
	}
}

// Row 8: Indeterminate + RiskLevel=normal/low → retry.
func TestDecision_Row8_IndeterminateRiskNormal_Retry(t *testing.T) {
	cases := []string{"normal", "low", "Normal", "LOW"} // case insensitive
	for _, risk := range cases {
		r := runDecide(t, DecisionContext{
			RoundMeta:  RoundMeta{AttemptNo: 0, RiskLevel: risk},
			VerdictKind: uint8(types.VerdictIndeterminate),
		})
		if r.Decision.Kind != DecisionRetry {
			t.Errorf("RiskLevel=%q → Kind=%s, want retry", risk, r.Decision.Kind)
		}
		if r.Decision.MapRow != 8 {
			t.Errorf("RiskLevel=%q → MapRow=%d, want 8", risk, r.Decision.MapRow)
		}
	}
}

// Row 10: child segment + all siblings decided → parent_rollup, even on Pass.
func TestDecision_Row10_AllSiblingsDecided_ParentRollup(t *testing.T) {
	cases := []uint8{0, 1, 2, 3} // Pass / Partial / Indeterminate / Fail
	for _, vk := range cases {
		r := runDecide(t, DecisionContext{
			RoundMeta: RoundMeta{
				AttemptNo:           0,
				IsChildSegment:      true,
				SiblingDecidedCount: 3,
				SiblingTotalCount:   3,
			},
			VerdictKind: vk,
		})
		if r.Decision.Kind != DecisionParentRollup {
			t.Errorf("Verdict=%d → Kind=%s, want parent_rollup", vk, r.Decision.Kind)
		}
		if r.Decision.MapRow != 10 {
			t.Errorf("Verdict=%d → MapRow=%d, want 10", vk, r.Decision.MapRow)
		}
	}
}

// Row 10 ordering: fires even when plan_error is empty, so test the
// ordering guard by combining parent_rollup with a sibling-not-yet
// decided flag. Verdict=Pass + IsChildSegment but only 2/3 siblings
// decided → falls through to row 1 (accept).
func TestDecision_Row10_OrderingNotReady_FallsThrough(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta: RoundMeta{
			AttemptNo:           0,
			IsChildSegment:      true,
			SiblingDecidedCount: 2,
			SiblingTotalCount:   3,
		},
		VerdictKind: uint8(types.VerdictPass),
	})
	if r.Decision.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept (row 1 — row 10 not ready)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 1 {
		t.Errorf("MapRow = %d, want 1", r.Decision.MapRow)
	}
}

// Row 11: plan_error → human_review (PR-F anchor). Triggered before
// every other row including row 10, so even with all-siblings-decided
// + Pass, plan_error wins.
func TestDecision_Row11_PlanError_HumanReview(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta: RoundMeta{
			AttemptNo:           0,
			IsChildSegment:      true,
			SiblingDecidedCount: 3,
			SiblingTotalCount:   3,
		},
		PlanErrorClass: "PlanLLMCallTimeout",
	})
	if r.Decision.Kind != DecisionHumanReview {
		t.Errorf("Kind = %s, want human_review (row 11 plan_error)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 11 {
		t.Errorf("MapRow = %d, want 11", r.Decision.MapRow)
	}
	if !strings.Contains(r.Decision.Reason, "plan_error:PlanLLMCallTimeout") {
		t.Errorf("Reason = %q, want plan_error:PlanLLMCallTimeout", r.Decision.Reason)
	}
}

// Row 9: out-of-range VerdictKind + VerdictErrorClass=network_timeout → retry.
// We feed VerdictKind=99 (no enum entry) + a Network/Timeout class to
// exercise the path that fires for non-4-state verdicts. Mirrors
// decision-tree.md §8.6.1 row 9.
func TestDecision_Row9_OutOfRangeVerdictError_Retry(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta:         RoundMeta{AttemptNo: 0},
		VerdictKind:       99, // out of enum
		VerdictErrorClass: "network_timeout",
	})
	if r.Decision.Kind != DecisionRetry {
		t.Errorf("Kind = %s, want retry (row 9)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 9 {
		t.Errorf("MapRow = %d, want 9", r.Decision.MapRow)
	}
}

// Safety-net fallback: out-of-range VerdictKind with no VerdictErrorClass
// falls through to A accept + MapRow=0 (decision_map_miss_fallback).
func TestDecision_MapMiss_FallbackAccept(t *testing.T) {
	r := runDecide(t, DecisionContext{
		RoundMeta:   RoundMeta{AttemptNo: 0},
		VerdictKind: 99,
	})
	if r.Decision.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept (fallback)", r.Decision.Kind)
	}
	if r.Decision.MapRow != 0 {
		t.Errorf("MapRow = %d, want 0 (fallback)", r.Decision.MapRow)
	}
	if r.Decision.Reason != "decision_map_miss_fallback" {
		t.Errorf("Reason = %q, want decision_map_miss_fallback", r.Decision.Reason)
	}
}

// Defensive guard: runaway AttemptNo (>100) errors out so a corrupted
// RoundMeta can't infinitely retry.
func TestDecision_RunawayAttemptNo_ErrorsOut(t *testing.T) {
	_, err := NewStaticDecisionNode().Decide(DecisionContext{
		RoundMeta:  RoundMeta{AttemptNo: 101},
		VerdictKind: uint8(types.VerdictFail),
	})
	if err == nil {
		t.Fatal("Decide err = nil, want non-nil (runaway AttemptNo)")
	}
}

// Defensive guard: negative AttemptNo errors out.
func TestDecision_NegativeAttemptNo_ErrorsOut(t *testing.T) {
	_, err := NewStaticDecisionNode().Decide(DecisionContext{
		RoundMeta:  RoundMeta{AttemptNo: -1},
		VerdictKind: uint8(types.VerdictFail),
	})
	if err == nil {
		t.Fatal("Decide err = nil, want non-nil")
	}
}

// TestChildWorkItemSpec_Validate covers the §2.13 design invariants.
// SubWorkerSpawner calls Validate before allocating a WorkItem.
func TestChildWorkItemSpec_Validate(t *testing.T) {
	cases := []struct {
		name    string
		spec    *ChildWorkItemSpec
		wantErr bool
	}{
		{
			name:    "nil",
			spec:    nil,
			wantErr: true,
		},
		{
			name: "missing parent",
			spec: &ChildWorkItemSpec{
				ParentWorkItemID: "",
				SubSegmentIDs:    []string{"AC-1"},
				InheritACSubset:  []string{"AC-1"},
				MaxBudget:        1,
			},
			wantErr: true,
		},
		{
			name: "missing sub segments",
			spec: &ChildWorkItemSpec{
				ParentWorkItemID: "wi_parent",
				InheritACSubset:  []string{"AC-1"},
				MaxBudget:        1,
			},
			wantErr: true,
		},
		{
			name: "missing AC subset",
			spec: &ChildWorkItemSpec{
				ParentWorkItemID: "wi_parent",
				SubSegmentIDs:    []string{"AC-1"},
				MaxBudget:        1,
			},
			wantErr: true,
		},
		{
			name: "max budget over 2",
			spec: &ChildWorkItemSpec{
				ParentWorkItemID: "wi_parent",
				SubSegmentIDs:    []string{"AC-1"},
				InheritACSubset:  []string{"AC-1"},
				MaxBudget:        3,
			},
			wantErr: true,
		},
		{
			name: "valid",
			spec: &ChildWorkItemSpec{
				ParentWorkItemID: "wi_parent",
				SubSegmentIDs:    []string{"AC-1", "AC-2"},
				InheritACSubset:  []string{"AC-1"},
				MaxBudget:        2,
			},
			wantErr: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.spec.Validate()
			if (err != nil) != c.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

// TestMarshalDecisionJSON_RoundTrip verifies the wire format carries
// every Decision field and that re-marshaling produces identical JSON.
func TestMarshalDecisionJSON_RoundTrip(t *testing.T) {
	spec := &ChildWorkItemSpec{
		ParentWorkItemID: "wi_parent",
		SubSegmentIDs:    []string{"AC-1"},
		InheritACSubset:  []string{"AC-1"},
		MaxBudget:        2,
	}
	d := Decision{
		Kind:             DecisionChildWorker,
		Reason:           "verdict_partial+ac_decomposable",
		NextWorkItemSpec: spec,
		MapRow:           3,
	}
	encoded, err := MarshalDecisionJSON(d)
	if err != nil {
		t.Fatalf("MarshalDecisionJSON err = %v", err)
	}
	if encoded == "" {
		t.Fatal("encoded JSON = empty string")
	}

	// Decode and confirm every field survives.
	var got DecisionJSON
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		t.Fatalf("Unmarshal err = %v on %s", err, encoded)
	}
	if got.Kind != "child_worker" {
		t.Errorf("Kind = %q, want child_worker", got.Kind)
	}
	if got.Reason != "verdict_partial+ac_decomposable" {
		t.Errorf("Reason = %q", got.Reason)
	}
	if got.MapRow != 3 {
		t.Errorf("MapRow = %d, want 3", got.MapRow)
	}
	if got.NextWorkItemSpec == nil {
		t.Fatal("NextWorkItemSpec = nil after round-trip")
	}
	if got.NextWorkItemSpec.MaxBudget != 2 {
		t.Errorf("NextWorkItemSpec.MaxBudget = %d, want 2", got.NextWorkItemSpec.MaxBudget)
	}
	if got.DecidedAt.IsZero() {
		t.Error("DecidedAt = zero, want populated")
	}
}

// TestMarshalDecisionJSON_NoSpec verifies the omit-empty rule for
// D (parent_rollup) / A / B / E paths where NextWorkItemSpec is nil.
func TestMarshalDecisionJSON_NoSpec(t *testing.T) {
	d := Decision{
		Kind:   DecisionParentRollup,
		Reason: "all_siblings_decided (3/3)",
		MapRow: 10,
	}
	encoded, err := MarshalDecisionJSON(d)
	if err != nil {
		t.Fatalf("MarshalDecisionJSON err = %v", err)
	}
	if strings.Contains(encoded, "next_spec") {
		t.Fatalf("expected no next_spec field, got %s", encoded)
	}
}

// TestDecision_MapRow_OnlyFromFallback pins MapRow=0 to the safety-net
// path (codex consensus Q4). D5 dashboards grep on MapRow; if any
// non-fallback row ever produced MapRow=0, telemetry would conflate
// Pass-accept rows with map-miss fallbacks.
func TestDecision_MapRow_OnlyFromFallback(t *testing.T) {
	cases := []struct {
		name string
		ctx  DecisionContext
	}{
		{"Row1_Pass", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0}, VerdictKind: uint8(types.VerdictPass)}},
		{"Row2a_PartialHighTol", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, ToleranceHint: "high"}, VerdictKind: uint8(types.VerdictPartial)}},
		{"Row2b_PartialChildBudget0", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, ChildBudgetRemaining: 0}, VerdictKind: uint8(types.VerdictPartial)}},
		{"Row3_PartialDecomposable", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, ChildBudgetRemaining: 2, HasDecomposableAC: true}, VerdictKind: uint8(types.VerdictPartial), ACSubset: []string{"AC-1"}}},
		{"Row4_PartialFallback", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, ChildBudgetRemaining: 2, HasDecomposableAC: false}, VerdictKind: uint8(types.VerdictPartial)}},
		{"Row5_FailBelow", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0}, VerdictKind: uint8(types.VerdictFail)}},
		{"Row6_FailAbove", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 1}, VerdictKind: uint8(types.VerdictFail)}},
		{"Row7_IndetHigh", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, RiskLevel: "high"}, VerdictKind: uint8(types.VerdictIndeterminate)}},
		{"Row8_IndetNormal", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, RiskLevel: "normal"}, VerdictKind: uint8(types.VerdictIndeterminate)}},
		{"Row10_AllSiblings", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0, IsChildSegment: true, SiblingDecidedCount: 3, SiblingTotalCount: 3}, VerdictKind: uint8(types.VerdictPass)}},
		{"Row11_PlanError", DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0}, PlanErrorClass: "PlanLLMCallTimeout"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := NewStaticDecisionNodeWithMaxRetry(1)
			d, err := node.Decide(c.ctx)
			if err != nil {
				t.Fatalf("Decide err = %v", err)
			}
			if d.MapRow == 0 {
				t.Errorf("MapRow = 0; only the safety-net fallback should produce MapRow=0")
			}
		})
	}
	// And the fallback path explicitly IS MapRow=0.
	r := runDecide(t, DecisionContext{RoundMeta: RoundMeta{AttemptNo: 0}, VerdictKind: 99})
	if r.Decision.MapRow != 0 || r.Decision.Reason != "decision_map_miss_fallback" {
		t.Errorf("fallback: MapRow=%d Reason=%q, want 0 + decision_map_miss_fallback",
			r.Decision.MapRow, r.Decision.Reason)
	}
}

// TestDecision_EmptyRoundMetaSafe_DefaultsApplied pins the Q5 contract:
// callers may pass a zero-value RoundMeta, and Decide must still return
// a valid (Accept) Decision rather than erroring. Without this pin, a
// future refactor adding a "round_meta required" guard would silently
// break TestE2E_Helper_UsesProductionLearner / legacy pre-PR-D callers.
func TestDecision_EmptyRoundMetaSafe_DefaultsApplied(t *testing.T) {
	d, err := NewStaticDecisionNode().Decide(DecisionContext{
		RoundMeta:  RoundMeta{}, // every field zero
		VerdictKind: uint8(types.VerdictPass),
	})
	if err != nil {
		t.Fatalf("Decide err = %v on empty RoundMeta, want nil (Q5 contract)", err)
	}
	if d.Kind != DecisionAccept {
		t.Errorf("Kind = %s, want accept on empty RoundMeta + Pass", d.Kind)
	}
	if d.MapRow != 1 {
		t.Errorf("MapRow = %d, want 1 (row 1)", d.MapRow)
	}
	if d.Reason != "verdict_pass" {
		t.Errorf("Reason = %q, want verdict_pass", d.Reason)
	}

	// Empty RoundMeta + Fail also safe (falls through to row 6 by default
	// since AttemptNo=0 and MaxRetry=1 ⇒ 0 < 1 ⇒ row 5 retry).
	d2, err := NewStaticDecisionNode().Decide(DecisionContext{
		RoundMeta:  RoundMeta{},
		VerdictKind: uint8(types.VerdictFail),
	})
	if err != nil {
		t.Fatalf("Decide err = %v on empty RoundMeta + Fail", err)
	}
	if d2.Kind != DecisionRetry {
		t.Errorf("Kind = %s, want retry (row 5, default MaxRetry=1)", d2.Kind)
	}
}
