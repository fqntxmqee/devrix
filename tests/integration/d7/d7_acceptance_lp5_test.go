//go:build integration && d7

// LP-5 acceptance test (DM-20260629-001 PR-7, T43).
//
// LP-5 reverse-trace lineage: every LearnRequest produces an asset whose
// SourceSessionIDs and SourceVerdictIDs propagate from the input Verdict
// and Plan. The reverse trace must be retrievable: from a stored asset's
// keys we can find which sessions + verdicts produced it.
//
// Acceptance criteria:
//  1. SourceSessionIDs always includes the input sessionID.
//  2. SourceVerdictIDs is non-empty (LP-5 ≥1 invariant).
//  3. Two VerdictPass from two different sessions produce two distinct
//     assets, each traceable back to its origin session.
//  4. Multi-source lineage: a LearnRequest that lists 2 prior source
//     sessions + 2 prior source verdicts preserves them all.
package d7integration

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: DM-20260629-001 T43.c — LP-5 reverse trace: SourceSessionIDs ≥ 1.
func TestAcceptance_LP5_SourceSessionIDsPreserved(t *testing.T) {
	sessionID := "sess-lp5-trace"
	f := newLP1Fixture(t, sessionID)

	req := learn.LearnRequest{
		SessionID: sessionID,
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_lp5_1", Reason: "ok"},
		Plan:      plan.NewPlan("p_lp5", sessionID, plan.CommitmentPlan, []string{"o"}, []plan.Step{{ID: "s"}}, 0.8),
		Artifact:  artifactStub("a_lp5", sessionID, nil),
	}
	assets, err := f.learner.Learn(context.Background(), req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("assets len = %d, want 1", len(assets))
	}
	asset := assets[0]

	if len(asset.SourceSessionIDs) < 1 {
		t.Errorf("SourceSessionIDs len = %d, want ≥ 1 (LP-5 invariant)", len(asset.SourceSessionIDs))
	}
	found := false
	for _, s := range asset.SourceSessionIDs {
		if s == sessionID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SourceSessionIDs = %v, must contain %q", asset.SourceSessionIDs, sessionID)
	}
}

// T: DM-20260629-001 T43.c — LP-5 reverse trace: SourceVerdictIDs ≥ 1.
func TestAcceptance_LP5_SourceVerdictIDsPreserved(t *testing.T) {
	sessionID := "sess-lp5-verdict"
	f := newLP1Fixture(t, sessionID)

	req := learn.LearnRequest{
		SessionID: sessionID,
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_lp5_specific_id", Reason: "ok"},
		Plan:      plan.NewPlan("p_lp5_v", sessionID, plan.CommitmentPlan, []string{"o"}, []plan.Step{{ID: "s"}}, 0.8),
		Artifact:  artifactStub("a_lp5_v", sessionID, nil),
	}
	assets, err := f.learner.Learn(context.Background(), req)
	if err != nil {
		t.Fatalf("Learn: %v", err)
	}
	if len(assets[0].SourceVerdictIDs) < 1 {
		t.Errorf("SourceVerdictIDs len = %d, want ≥ 1 (LP-5 invariant)", len(assets[0].SourceVerdictIDs))
	}
	found := false
	for _, v := range assets[0].SourceVerdictIDs {
		if v == "v_lp5_specific_id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SourceVerdictIDs = %v, must contain %q", assets[0].SourceVerdictIDs, "v_lp5_specific_id")
	}
}

// T: DM-20260629-001 T43.c — LP-5 cross-session isolation: two sessions
// produce two distinct assets, each traceable to its own session only.
func TestAcceptance_LP5_CrossSessionIsolation(t *testing.T) {
	f := newLP1Fixture(t, "shared-fixture")

	// Session A: VerdictPass.
	aAssets, err := f.learner.Learn(context.Background(), learn.LearnRequest{
		SessionID: "sess-A",
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_A", Reason: "ok"},
		Plan:      plan.NewPlan("pA", "sess-A", plan.CommitmentPlan, []string{"o"}, []plan.Step{{ID: "s"}}, 0.8),
		Artifact:  artifactStub("aA", "sess-A", nil),
	})
	if err != nil {
		t.Fatalf("Learn(A): %v", err)
	}

	// Session B: VerdictPass.
	bAssets, err := f.learner.Learn(context.Background(), learn.LearnRequest{
		SessionID: "sess-B",
		Verdict:   workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_B", Reason: "ok"},
		Plan:      plan.NewPlan("pB", "sess-B", plan.CommitmentPlan, []string{"o"}, []plan.Step{{ID: "s"}}, 0.8),
		Artifact:  artifactStub("aB", "sess-B", nil),
	})
	if err != nil {
		t.Fatalf("Learn(B): %v", err)
	}

	if aAssets[0].AssetKey == bAssets[0].AssetKey {
		t.Errorf("A and B must have distinct AssetKey, both = %q", aAssets[0].AssetKey)
	}

	// Reverse-trace: A's asset must reference sess-A, not sess-B.
	aSessHasA := false
	for _, s := range aAssets[0].SourceSessionIDs {
		if s == "sess-A" {
			aSessHasA = true
		}
		if s == "sess-B" {
			t.Errorf("A's asset must NOT reference sess-B, got %v", aAssets[0].SourceSessionIDs)
		}
	}
	if !aSessHasA {
		t.Errorf("A's SourceSessionIDs missing sess-A: %v", aAssets[0].SourceSessionIDs)
	}
	bSessHasB := false
	for _, s := range bAssets[0].SourceSessionIDs {
		if s == "sess-B" {
			bSessHasB = true
		}
	}
	if !bSessHasB {
		t.Errorf("B's SourceSessionIDs missing sess-B: %v", bAssets[0].SourceSessionIDs)
	}
}
