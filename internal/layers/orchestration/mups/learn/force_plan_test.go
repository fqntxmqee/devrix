// Package learn: force_plan tests (DM-20260707-001 PR-E, T63).
//
// 6-dimension coverage matrix for BayesianUpdateWithPolicy + ForcePlanSignal +
// EmitForcePlanMetadata + ShouldForcePlan:
//
//   1. ComputeForcePlanSignal_ColdStart     — α=β=0 → not triggered
//   2. ComputeForcePlanSignal_BelowThreshold — 5/10 (0.5) → not triggered
//   3. ComputeForcePlanSignal_AboveThreshold — 8/10 (0.8) → triggered
//   4. BayesianUpdateWithPolicy_PassIncreasesAlpha
//   5. BayesianUpdateWithPolicy_FailIncreasesBeta
//   6. BayesianUpdateWithPolicy_NilPrior_ReturnsError
//   7. EmitForcePlanMetadata_NilReturnsNil
//   8. EmitForcePlanMetadata_TriggeredBuildsMetadata
//   9. ShouldForcePlan_NilFalse
//  10. ShouldForcePlan_EmptyFalse
//  11. ShouldForcePlan_TruePositive
package learn

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/reputation"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// TestComputeForcePlanSignal_ColdStart: α=β=0 → not triggered, reason=cold_start.
func TestComputeForcePlanSignal_ColdStart(t *testing.T) {
	t.Parallel()
	sig := computeForcePlanSignal("sess1", 0, 0)
	if sig.Triggered {
		t.Errorf("cold start unexpectedly triggered force_plan")
	}
	if sig.Reason != "cold_start" {
		t.Errorf("Reason = %s, want cold_start", sig.Reason)
	}
	if sig.BetaRatio != 0 {
		t.Errorf("BetaRatio = %f, want 0", sig.BetaRatio)
	}
	if sig.SessionID != "sess1" {
		t.Errorf("SessionID = %s, want sess1", sig.SessionID)
	}
}

// TestComputeForcePlanSignal_BelowThreshold: 5/10 = 0.5 → not triggered.
func TestComputeForcePlanSignal_BelowThreshold(t *testing.T) {
	t.Parallel()
	sig := computeForcePlanSignal("sess2", 5, 5)
	if sig.Triggered {
		t.Errorf("5/10 ratio unexpectedly triggered force_plan")
	}
	if sig.Reason != "below_threshold" {
		t.Errorf("Reason = %s, want below_threshold", sig.Reason)
	}
	if sig.BetaRatio != 0.5 {
		t.Errorf("BetaRatio = %f, want 0.5", sig.BetaRatio)
	}
}

// TestComputeForcePlanSignal_AboveThreshold: 8/10 = 0.8 → triggered.
func TestComputeForcePlanSignal_AboveThreshold(t *testing.T) {
	t.Parallel()
	sig := computeForcePlanSignal("sess3", 2, 8)
	if !sig.Triggered {
		t.Errorf("2/8 ratio (0.8) should trigger force_plan")
	}
	if sig.Reason != "force_plan_threshold_crossed" {
		t.Errorf("Reason = %s, want force_plan_threshold_crossed", sig.Reason)
	}
	if sig.BetaRatio != 0.8 {
		t.Errorf("BetaRatio = %f, want 0.8", sig.BetaRatio)
	}
}

// TestBayesianUpdateWithPolicy_PassIncreasesAlpha: VerdictPass on a fresh
// prior → alpha incremented, signal not triggered (single pass cannot tip).
func TestBayesianUpdateWithPolicy_PassIncreasesAlpha(t *testing.T) {
	t.Parallel()
	prior, err := reputation.NewReputationEvidence("sess_pass", reputation.TrackModeDeveloper)
	if err != nil {
		t.Fatalf("NewReputationEvidence failed: %v", err)
	}
	verdict := workmodel.Verdict{Kind: types.VerdictPass, Confidence: 0.9, SourceID: "v_pass_1"}
	next, sig, err := BayesianUpdateWithPolicy("sess_pass", prior, verdict)
	if err != nil {
		t.Fatalf("BayesianUpdateWithPolicy returned error: %v", err)
	}
	if next.Alpha != 1 {
		t.Errorf("Alpha = %d, want 1", next.Alpha)
	}
	if next.Beta != 0 {
		t.Errorf("Beta = %d, want 0", next.Beta)
	}
	if sig.Triggered {
		t.Errorf("single Pass unexpectedly triggered force_plan")
	}
}

// TestBayesianUpdateWithPolicy_FailIncreasesBeta: VerdictFail on a fresh prior
// → beta=1, signal not triggered (1 fail at total=1 = 1.0 ratio would trigger,
// but the policy cold-start protection kicks in).
func TestBayesianUpdateWithPolicy_FailIncreasesBeta(t *testing.T) {
	t.Parallel()
	prior, err := reputation.NewReputationEvidence("sess_fail", reputation.TrackModeDeveloper)
	if err != nil {
		t.Fatalf("NewReputationEvidence failed: %v", err)
	}
	verdict := workmodel.Verdict{Kind: types.VerdictFail, Reason: "missing_field", SourceID: "v_fail_1"}
	next, sig, err := BayesianUpdateWithPolicy("sess_fail", prior, verdict)
	if err != nil {
		t.Fatalf("BayesianUpdateWithPolicy returned error: %v", err)
	}
	if next.Beta != 1 {
		t.Errorf("Beta = %d, want 1", next.Beta)
	}
	if next.Alpha != 0 {
		t.Errorf("Alpha = %d, want 0", next.Alpha)
	}
	// Total=1, ratio=1.0, > 0.7 → triggers.
	if !sig.Triggered {
		t.Errorf("1/1 (100%%) should trigger force_plan")
	}
	if sig.Reason != "force_plan_threshold_crossed" {
		t.Errorf("Reason = %s, want force_plan_threshold_crossed", sig.Reason)
	}
}

// TestBayesianUpdateWithPolicy_NilPrior_ReturnsError: prior==nil → cold start
// signal + ErrReputationStoreUnavailable (PR-C Risk A4).
func TestBayesianUpdateWithPolicy_NilPrior_ReturnsError(t *testing.T) {
	t.Parallel()
	verdict := workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_nil"}
	_, sig, err := BayesianUpdateWithPolicy("sess_nil", nil, verdict)
	if err == nil {
		t.Errorf("expected error for nil prior, got nil")
	}
	if sig == nil {
		t.Fatalf("expected non-nil signal even on error")
	}
	if sig.Triggered {
		t.Errorf("nil prior should not trigger force_plan")
	}
	if sig.Reason != "cold_start_or_store_unavailable" {
		t.Errorf("Reason = %s, want cold_start_or_store_unavailable", sig.Reason)
	}
}

// TestEmitForcePlanMetadata_NilReturnsNil: no signal → no metadata.
func TestEmitForcePlanMetadata_NilReturnsNil(t *testing.T) {
	t.Parallel()
	if md := EmitForcePlanMetadata(nil); md != nil {
		t.Errorf("EmitForcePlanMetadata(nil) = %v, want nil", md)
	}
}

// TestEmitForcePlanMetadata_TriggeredBuildsMetadata: triggered signal →
// populated map with the 6 expected keys.
func TestEmitForcePlanMetadata_TriggeredBuildsMetadata(t *testing.T) {
	t.Parallel()
	sig := &ForcePlanSignal{
		Triggered: true,
		BetaRatio: 0.83,
		Alpha:     2,
		Beta:      10,
		Reason:    "force_plan_threshold_crossed",
		SessionID: "sess_emit",
	}
	md := EmitForcePlanMetadata(sig)
	if md == nil {
		t.Fatalf("EmitForcePlanMetadata returned nil for triggered signal")
	}
	expectedKeys := []string{"force_plan", "force_plan_ratio", "force_plan_alpha",
		"force_plan_beta", "force_plan_reason", "force_plan_computed_at"}
	for _, k := range expectedKeys {
		if _, ok := md[k]; !ok {
			t.Errorf("metadata missing key %q", k)
		}
	}
	if md["force_plan"] != "true" {
		t.Errorf("force_plan = %s, want true", md["force_plan"])
	}
	if md["force_plan_alpha"] != "2" {
		t.Errorf("force_plan_alpha = %s, want 2", md["force_plan_alpha"])
	}
	if md["force_plan_beta"] != "10" {
		t.Errorf("force_plan_beta = %s, want 10", md["force_plan_beta"])
	}
}

// TestEmitForcePlanMetadata_NotTriggeredReturnsNil: not-triggered signal →
// no metadata (so Observe reads "absence" as a no-op).
func TestEmitForcePlanMetadata_NotTriggeredReturnsNil(t *testing.T) {
	t.Parallel()
	sig := &ForcePlanSignal{Triggered: false, Reason: "below_threshold"}
	if md := EmitForcePlanMetadata(sig); md != nil {
		t.Errorf("not-triggered signal unexpectedly built metadata: %v", md)
	}
}

// TestShouldForcePlan_NilFalse: nil metadata → false.
func TestShouldForcePlan_NilFalse(t *testing.T) {
	t.Parallel()
	if ShouldForcePlan(nil) {
		t.Errorf("ShouldForcePlan(nil) = true, want false")
	}
}

// TestShouldForcePlan_EmptyFalse: empty metadata → false.
func TestShouldForcePlan_EmptyFalse(t *testing.T) {
	t.Parallel()
	if ShouldForcePlan(map[string]string{}) {
		t.Errorf("ShouldForcePlan(empty) = true, want false")
	}
}

// TestShouldForcePlan_TruePositive: "force_plan=true" → true.
func TestShouldForcePlan_TruePositive(t *testing.T) {
	t.Parallel()
	md := map[string]string{"force_plan": "true", "force_plan_ratio": "0.83"}
	if !ShouldForcePlan(md) {
		t.Errorf("ShouldForcePlan({force_plan:true}) = false, want true")
	}
}

// TestShouldForcePlan_FalsePositive: "force_plan=false" or other → false.
func TestShouldForcePlan_FalsePositive(t *testing.T) {
	t.Parallel()
	if ShouldForcePlan(map[string]string{"force_plan": "false"}) {
		t.Errorf("ShouldForcePlan({force_plan:false}) = true, want false")
	}
	if ShouldForcePlan(map[string]string{"other_key": "value"}) {
		t.Errorf("ShouldForcePlan({other_key}) = true, want false")
	}
}

// TestForcePlanSignal_String: rendered output includes all 5 fields.
func TestForcePlanSignal_String(t *testing.T) {
	t.Parallel()
	sig := ForcePlanSignal{
		Triggered: true, BetaRatio: 0.83, Alpha: 2, Beta: 10, Reason: "force_plan_threshold_crossed",
	}
	s := sig.String()
	for _, want := range []string{"force_plan=true", "ratio=0.83", "alpha=2", "beta=10", "reason=force_plan_threshold_crossed"} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, missing substring %q", s, want)
		}
	}
}