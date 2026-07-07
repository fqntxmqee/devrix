// Package plan: ForcePlan Plan-injection tests (DM-20260707-001 PR-F, T71).
//
// Coverage matrix:
//
//   1. TestShouldForcePlanFromPlan_Defaults: nil Plan / no Metadata → false.
//   2. TestShouldForcePlanFromPlan_Injected: Inject + Read round-trip.
//   3. TestShouldForcePlanFromPlan_NotTriggered: nil hint / Triggered=false
//      → no metadata written; ShouldForcePlanFromPlan returns false.
//   4. TestInjectForcePlanHint_PreservesOtherKeys: existing keys survive.
//   5. TestReadForcePlanHint_PartialMetadata: missing keys → zero values.
//   6. TestClearForcePlanHint_RemovesForcePlanKeys: 7 keys removed, others
//      preserved.
//   7. TestInjectForcePlanHint_ImmutableCopy: receiver Metadata not mutated.
//
// Cross-package TestEmitMetadataContract_LearnAndPlanMatch lives in
// mups/learn/force_plan_integration_test.go (the only place that can
// import both packages without creating a cycle).
package plan

import (
	"testing"
)

// TestShouldForcePlanFromPlan_Defaults.
func TestShouldForcePlanFromPlan_Defaults(t *testing.T) {
	t.Parallel()
	var nilPlan *Plan
	if ShouldForcePlanFromPlan(nilPlan) {
		t.Errorf("nil Plan should report false")
	}
	noMeta := &Plan{}
	if ShouldForcePlanFromPlan(noMeta) {
		t.Errorf("Plan with nil Metadata should report false")
	}
	withMeta := &Plan{Metadata: map[string]string{"other_key": "x"}}
	if ShouldForcePlanFromPlan(withMeta) {
		t.Errorf("Plan without force_plan key should report false")
	}
}

// TestShouldForcePlanFromPlan_NilMetadata.
func TestShouldForcePlanFromPlan_NilMetadata(t *testing.T) {
	t.Parallel()
	if ShouldForcePlanFromMetadata(nil) {
		t.Errorf("nil metadata should report false")
	}
	if ShouldForcePlanFromMetadata(map[string]string{}) {
		t.Errorf("empty metadata should report false")
	}
}

// TestInjectForcePlanHint_RoundTrip.
func TestInjectForcePlanHint_RoundTrip(t *testing.T) {
	t.Parallel()
	original := &Plan{ID: "p1"}
	hint := &PlanForcePlanHint{
		Triggered:  true,
		BetaRatio:  0.85,
		Alpha:      12,
		Beta:       67,
		Reason:     "force_plan_threshold_crossed",
		ComputedAt: "2026-07-07T10:30:00Z",
		SessionID:  "sess_xx",
	}
	injected := InjectForcePlanHint(*original, hint)
	if !ShouldForcePlanFromPlan(&injected) {
		t.Fatalf("injected plan should report ShouldForcePlan=true")
	}
	got := ReadForcePlanHint(&injected)
	if got == nil {
		t.Fatalf("ReadForcePlanHint returned nil on injected plan")
	}
	if got.BetaRatio != 0.85 {
		t.Errorf("BetaRatio = %v, want 0.85", got.BetaRatio)
	}
	if got.Alpha != 12 || got.Beta != 67 {
		t.Errorf("Alpha/Beta = %d/%d, want 12/67", got.Alpha, got.Beta)
	}
	if got.Reason != "force_plan_threshold_crossed" {
		t.Errorf("Reason = %q, want force_plan_threshold_crossed", got.Reason)
	}
	if got.SessionID != "sess_xx" {
		t.Errorf("SessionID = %q, want sess_xx", got.SessionID)
	}
}

// TestInjectForcePlanHint_NilOrUntriggered_NoOp.
func TestInjectForcePlanHint_NilOrUntriggered_NoOp(t *testing.T) {
	t.Parallel()
	original := &Plan{ID: "p1"}
	// nil hint → unchanged
	injected := InjectForcePlanHint(*original, nil)
	if ShouldForcePlanFromPlan(&injected) {
		t.Errorf("nil hint should not write metadata")
	}
	// Triggered=false hint → unchanged
	injected2 := InjectForcePlanHint(*original, &PlanForcePlanHint{Triggered: false})
	if ShouldForcePlanFromPlan(&injected2) {
		t.Errorf("untriggered hint should not write metadata")
	}
}

// TestInjectForcePlanHint_PreservesOtherKeys.
func TestInjectForcePlanHint_PreservesOtherKeys(t *testing.T) {
	t.Parallel()
	original := &Plan{
		Metadata: map[string]string{
			"session_key": "val_x",
			"user_key":    "val_y",
		},
	}
	hint := &PlanForcePlanHint{Triggered: true, Reason: "r"}
	injected := InjectForcePlanHint(*original, hint)
	if injected.Metadata["session_key"] != "val_x" {
		t.Errorf("session_key lost: %v", injected.Metadata)
	}
	if injected.Metadata["user_key"] != "val_y" {
		t.Errorf("user_key lost: %v", injected.Metadata)
	}
	if injected.Metadata[ForcePlanMetaKey] != "true" {
		t.Errorf("force_plan key missing: %v", injected.Metadata)
	}
}

// TestInjectForcePlanHint_ImmutableCopy: mutating injected.Metadata must
// not affect original.
func TestInjectForcePlanHint_ImmutableCopy(t *testing.T) {
	t.Parallel()
	original := &Plan{
		Metadata: map[string]string{"original_key": "original_val"},
	}
	hint := &PlanForcePlanHint{Triggered: true, Reason: "r"}
	injected := InjectForcePlanHint(*original, hint)
	injected.Metadata["mutated_key"] = "mutated_val"
	if _, ok := original.Metadata["mutated_key"]; ok {
		t.Errorf("original Metadata was mutated through injected reference (immutability broken)")
	}
}

// TestReadForcePlanHint_NoForcePlanKey.
func TestReadForcePlanHint_NoForcePlanKey(t *testing.T) {
	t.Parallel()
	p := &Plan{Metadata: map[string]string{"other_key": "x"}}
	if got := ReadForcePlanHint(p); got != nil {
		t.Errorf("ReadForcePlanHint on no-force_plan metadata = %+v, want nil", got)
	}
}

// TestReadForcePlanHint_PartialMetadata: missing keys default to zero.
func TestReadForcePlanHint_PartialMetadata(t *testing.T) {
	t.Parallel()
	p := &Plan{Metadata: map[string]string{ForcePlanMetaKey: "true"}}
	got := ReadForcePlanHint(p)
	if got == nil {
		t.Fatalf("ReadForcePlanHint returned nil")
	}
	if got.BetaRatio != 0 || got.Alpha != 0 || got.Beta != 0 {
		t.Errorf("partial metadata numerics = (%v, %d, %d), want zero", got.BetaRatio, got.Alpha, got.Beta)
	}
	if got.Reason != "" || got.ComputedAt != "" || got.SessionID != "" {
		t.Errorf("partial metadata strings should be empty: %+v", got)
	}
}

// TestReadForcePlanHint_BadNumericTolerated.
func TestReadForcePlanHint_BadNumericTolerated(t *testing.T) {
	t.Parallel()
	p := &Plan{Metadata: map[string]string{
		ForcePlanMetaKey:           "true",
		ForcePlanMetaRatioKey:      "not_a_number",
		ForcePlanMetaAlphaKey:      "abc",
	}}
	got := ReadForcePlanHint(p)
	if got == nil {
		t.Fatalf("ReadForcePlanHint returned nil")
	}
	if got.BetaRatio != 0 || got.Alpha != 0 {
		t.Errorf("bad numerics should map to zero: (%v, %d)", got.BetaRatio, got.Alpha)
	}
}

// TestClearForcePlanHint_RemovesForcePlanKeys.
func TestClearForcePlanHint_RemovesForcePlanKeys(t *testing.T) {
	t.Parallel()
	p := &Plan{
		Metadata: map[string]string{
			ForcePlanMetaKey:        "true",
			ForcePlanMetaRatioKey:   "0.85",
			ForcePlanMetaReasonKey:  "r",
			"session_key":           "preserve_me",
		},
	}
	cleared := ClearForcePlanHint(*p)
	for _, k := range []string{
		ForcePlanMetaKey, ForcePlanMetaRatioKey, ForcePlanMetaReasonKey,
		ForcePlanMetaAlphaKey, ForcePlanMetaBetaKey,
		ForcePlanMetaComputedAtKey, ForcePlanMetaSessionIDKey,
	} {
		if _, ok := cleared.Metadata[k]; ok {
			t.Errorf("cleared.Metadata still has %s", k)
		}
	}
	if cleared.Metadata["session_key"] != "preserve_me" {
		t.Errorf("non-force-plan key was removed: %v", cleared.Metadata)
	}
}

// TestClearForcePlanHint_NoForcePlanKeys_NoOp.
func TestClearForcePlanHint_NoForcePlanKeys_NoOp(t *testing.T) {
	t.Parallel()
	p := &Plan{Metadata: map[string]string{"x": "y"}}
	cleared := ClearForcePlanHint(*p)
	if cleared.Metadata["x"] != "y" {
		t.Errorf("non-force_plan key was removed")
	}
}

// TestClearForcePlanHint_NilMetadata.
func TestClearForcePlanHint_NilMetadata(t *testing.T) {
	t.Parallel()
	p := &Plan{}
	cleared := ClearForcePlanHint(*p)
	if cleared.Metadata != nil {
		t.Errorf("nil Metadata → expected nil, got %v", cleared.Metadata)
	}
}
