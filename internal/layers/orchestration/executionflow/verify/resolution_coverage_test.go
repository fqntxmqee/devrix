package verify

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

func TestComputeResolutionCoverage_EmptyStrategiesReturnsNil(t *testing.T) {
	// Safety-net gate: Plan LLM did not file RC-1 contract → Decide stays
	// on legacy verdict-based routing. Caller observes nil report and
	// skips the AnySubWorktreePending / MaxUnresolvedStrength checks.
	cases := []struct {
		name      string
		strategies []interfaces.ResolutionStrategy
		claims     []interfaces.ResolutionClaim
	}{
		{
			name:       "nil strategies",
			strategies: nil,
			claims: []interfaces.ResolutionClaim{
				{ObsID: "obs-1", Answer: "x", Confidence: 0.9, SupportingEvidence: "ev"},
			},
		},
		{
			name:       "empty strategies",
			strategies: []interfaces.ResolutionStrategy{},
			claims:     []interfaces.ResolutionClaim{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeResolutionCoverage(tc.strategies, tc.claims, "s1", "wi-1", 1)
			if got != nil {
				t.Errorf("ComputeResolutionCoverage with %s: got %+v, want nil", tc.name, got)
			}
		})
	}
}

func TestComputeResolutionCoverage_FourStateMatrix(t *testing.T) {
	// Exercises the 4-state decision table that downstream Decide reads:
	//   resolved / no_claim / low_confidence / evidence_missing
	// plus the no_resolution_strategy orphan-claim path.
	//
	// The table is exhaustive over the 4 ResolutionReason values per
	// L5-D7-RT-03 + RT-05 + RT-06 (design.md §6 Fallback paths).
	strats := []interfaces.ResolutionStrategy{
		{ObsID: "obs-resolved", PlannedTool: "read_file"},
		{ObsID: "obs-no-claim"},
		{ObsID: "obs-low-confidence"},
		{ObsID: "obs-evidence-missing"},
	}
	claims := []interfaces.ResolutionClaim{
		{ObsID: "obs-resolved", Answer: "ans", Confidence: 0.9, SupportingEvidence: "ev-resolved"},
		{ObsID: "obs-low-confidence", Answer: "weak-ans", Confidence: 0.4, SupportingEvidence: "ev-low"},
		{ObsID: "obs-evidence-missing", Answer: "ans-no-ev", Confidence: 0.9, SupportingEvidence: ""},
		{ObsID: "obs-orphan", Answer: "orphan", Confidence: 0.9, SupportingEvidence: "ev-orphan"}, // no matching strategy
	}
	rep := ComputeResolutionCoverage(strats, claims, "s1", "wi-1", 1)
	if rep == nil {
		t.Fatal("ComputeResolutionCoverage returned nil for valid input")
	}

	if rep.TotalStrategies != 4 {
		t.Errorf("TotalStrategies = %d, want 4", rep.TotalStrategies)
	}
	if rep.TotalClaims != 4 {
		t.Errorf("TotalClaims = %d, want 4", rep.TotalClaims)
	}
	wantRatio := 1.0 / 4.0 // 1 resolved out of 4 strategies
	if rep.CoverageRatio < wantRatio-1e-9 || rep.CoverageRatio > wantRatio+1e-9 {
		t.Errorf("CoverageRatio = %.6f, want %.6f", rep.CoverageRatio, wantRatio)
	}

	// Build a lookup by ObsID for easier asserts.
	byID := make(map[string]interfaces.UnresolvedObs, len(rep.UnresolvedObs))
	for _, uo := range rep.UnresolvedObs {
		byID[uo.ObsID] = uo
	}

	// obs-resolved is NOT in unresolved.
	if _, ok := byID["obs-resolved"]; ok {
		t.Error("obs-resolved should not be in UnresolvedObs")
	}

	wantReasons := map[string]interfaces.ResolutionReason{
		"obs-no-claim":         interfaces.ResolutionReasonNoClaim,
		"obs-low-confidence":   interfaces.ResolutionReasonLowConfidence,
		"obs-evidence-missing": interfaces.ResolutionReasonEvidenceMissing,
		"obs-orphan":           interfaces.ResolutionReasonNoStrategy,
	}
	for obsID, wantReason := range wantReasons {
		uo, ok := byID[obsID]
		if !ok {
			t.Errorf("missing %s in UnresolvedObs", obsID)
			continue
		}
		if uo.Reason != wantReason {
			t.Errorf("%s Reason = %s, want %s", obsID, uo.Reason, wantReason)
		}
	}
}

func TestComputeResolutionCoverage_SubWorktreePropagates(t *testing.T) {
	// RC-4a hook: a strategy with SubWorktree propagates HasSubWorktree
	// onto its UnresolvedObs so Decide.AnySubWorktreePending() returns
	// true without re-scanning the originating strategies.
	strats := []interfaces.ResolutionStrategy{
		{
			ObsID:       "obs-1",
			SubWorktree: &interfaces.SubWorktreeSpec{Title: "investigate"},
		},
	}
	rep := ComputeResolutionCoverage(strats, nil, "s1", "wi-1", 1)
	if rep == nil {
		t.Fatal("ComputeResolutionCoverage returned nil")
	}
	if !rep.AnySubWorktreePending() {
		t.Error("AnySubWorktreePending = false, want true (RC-4a trigger)")
	}
	if len(rep.UnresolvedObs) != 1 {
		t.Fatalf("UnresolvedObs len = %d, want 1", len(rep.UnresolvedObs))
	}
	if !rep.UnresolvedObs[0].HasSubWorktree {
		t.Error("HasSubWorktree = false on UnresolvedObs, want true")
	}
}

func TestComputeResolutionCoverage_MaxUnresolvedStrengthZeroWhenEmpty(t *testing.T) {
	// When the round was fully covered MaxUnresolvedStrength must read
	// 0 so Decide.MaxUnresolvedStrength() < threshold (default 0.85)
	// triggers RC-4c SpawnInline rather than RC-4b SpawnUserGate.
	strats := []interfaces.ResolutionStrategy{{ObsID: "obs-1"}}
	claims := []interfaces.ResolutionClaim{
		{ObsID: "obs-1", Answer: "x", Confidence: 0.95, SupportingEvidence: "ev"},
	}
	rep := ComputeResolutionCoverage(strats, claims, "s1", "wi-1", 1)
	if rep == nil {
		t.Fatal("ComputeResolutionCoverage returned nil")
	}
	if rep.MaxUnresolvedStrength() != 0 {
		t.Errorf("MaxUnresolvedStrength = %.3f, want 0 (no unresolved)", rep.MaxUnresolvedStrength())
	}
}

func TestComputeResolutionCoverage_InvalidStrategiesDropsReport(t *testing.T) {
	// NewResolutionReport validates strategies via Validate(); an invalid
	// entry must yield nil so the round still completes (no panic, no
	// corrupted report). Downstream Decide falls back to verdict-based
	// routing as if no strategies had been filed.
	strats := []interfaces.ResolutionStrategy{{ObsID: ""}} // invalid: empty ObsID
	rep := ComputeResolutionCoverage(strats, nil, "s1", "wi-1", 1)
	if rep != nil {
		t.Errorf("ComputeResolutionCoverage with invalid strategy: got %+v, want nil", rep)
	}
}

func TestComputeResolutionCoverage_PreservesSessionAndWorkItem(t *testing.T) {
	// SessionID + WorkItemID + RoundNo are the cross-trace correlation
	// triple; downstream dashboards surface them as filter keys.
	strats := []interfaces.ResolutionStrategy{{ObsID: "obs-1"}}
	rep := ComputeResolutionCoverage(strats, nil, "sess_X", "wi_Y", 7)
	if rep == nil {
		t.Fatal("ComputeResolutionCoverage returned nil")
	}
	if rep.SessionID != "sess_X" || rep.WorkItemID != "wi_Y" || rep.RoundNo != 7 {
		t.Errorf("diagnostic triple = (%s, %s, %d), want (sess_X, wi_Y, 7)",
			rep.SessionID, rep.WorkItemID, rep.RoundNo)
	}
}
