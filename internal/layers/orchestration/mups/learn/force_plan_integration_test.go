// Package learn: cross-package contract tests (DM-20260707-001 PR-F, T71 cross).
//
// The force_plan signal is emitted by the Learn side (learn.EmitForcePlanMetadata)
// and consumed by the Plan side (plan.ReadForcePlanHint / ShouldForcePlanFromPlan).
// The two packages cannot import each other directly (cycle through learn/asset),
// so the integration test that ties them together lives HERE — the learn package
// can safely import plan, and verifies the contract by:
//
//   1. Round-tripping a signal through emit + read.
//   2. Asserting the field-name set matches plan.ForcePlanMeta* keys.
//
// If either side changes a key name without updating the other, this test catches it.
package learn

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// TestEmitMetadataContract_LearnAndPlanMatch: the Learn-side emitter and
// the Plan-side reader share the same field names. This is the cross-package
// contract that ties PR-E T63 (force_plan.go) to PR-F T71
// (plan_force_plan_injection.go).
func TestEmitMetadataContract_LearnAndPlanMatch(t *testing.T) {
	t.Parallel()
	ts, _ := time.Parse(time.RFC3339Nano, "2026-07-07T10:30:00Z")
	sig := &ForcePlanSignal{
		Triggered:  true,
		BetaRatio:  0.85,
		Alpha:      12,
		Beta:       67,
		Reason:     "force_plan_threshold_crossed",
		ComputedAt: ts,
		SessionID:  "sess_xxx",
	}
	emitted := EmitForcePlanMetadata(sig)
	if emitted == nil {
		t.Fatalf("EmitForcePlanMetadata returned nil for triggered signal")
	}
	// Read it back through Plan-side.
	p := &plan.Plan{Metadata: emitted}
	if !plan.ShouldForcePlanFromPlan(p) {
		t.Errorf("ShouldForcePlanFromPlan should be true for emitted metadata")
	}
	hint := plan.ReadForcePlanHint(p)
	if hint == nil {
		t.Fatalf("ReadForcePlanHint returned nil on learn-emitted metadata")
	}
	// round-trip comparators
	if hint.BetaRatio != sig.BetaRatio {
		t.Errorf("BetaRatio round-trip: %v vs %v", hint.BetaRatio, sig.BetaRatio)
	}
	if hint.Alpha != sig.Alpha {
		t.Errorf("Alpha round-trip: %d vs %d", hint.Alpha, sig.Alpha)
	}
	if hint.Beta != sig.Beta {
		t.Errorf("Beta round-trip: %d vs %d", hint.Beta, sig.Beta)
	}
	if hint.SessionID != sig.SessionID {
		t.Errorf("SessionID round-trip: %q vs %q", hint.SessionID, sig.SessionID)
	}
}

// TestEmitMetadataContract_FieldNamesMatch: every Plan-side key constant must
// match a Learn-emitted key (and vice versa). If a key is renamed on one
// side without updating the other, this table fails.
func TestEmitMetadataContract_FieldNamesMatch(t *testing.T) {
	t.Parallel()
	sig := &ForcePlanSignal{
		Triggered:  true,
		BetaRatio:  0.5,
		Alpha:      1,
		Beta:       2,
		Reason:     "r",
		ComputedAt: time.Now(),
		SessionID:  "s",
	}
	emitted := EmitForcePlanMetadata(sig)
	for _, k := range []string{
		plan.ForcePlanMetaKey,
		plan.ForcePlanMetaRatioKey,
		plan.ForcePlanMetaAlphaKey,
		plan.ForcePlanMetaBetaKey,
		plan.ForcePlanMetaReasonKey,
		plan.ForcePlanMetaComputedAtKey,
		plan.ForcePlanMetaSessionIDKey,
	} {
		if _, ok := emitted[k]; !ok {
			t.Errorf("learn-side metadata missing key %q expected by plan side", k)
		}
	}
}
