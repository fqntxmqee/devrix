package orchtypes_test

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn/reputation"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// TestBuildAdaptivePriorWithReport_ColdStartNilReport — DM-20260630-011.
//
// nil report must produce the same AdaptivePrior as the 2-arg
// BuildAdaptivePrior (cold-start baseline Developer / Operator prior).
func TestBuildAdaptivePriorWithReport_ColdStartNilReport(t *testing.T) {
	got := orchtypes.BuildAdaptivePriorWithReport(nil, reputation.TrackModeDeveloper, nil)
	if got == nil {
		t.Fatal("got nil prior")
	}
	if want := learn.BuildAdaptivePrior(nil, reputation.TrackModeDeveloper); got.PriorBeta != want.PriorBeta {
		t.Fatalf("nil report must equal 2-arg prior: got Beta(%d,%d), want Beta(%d,%d)",
			got.PriorBeta.Alpha, got.PriorBeta.Beta,
			want.PriorBeta.Alpha, want.PriorBeta.Beta)
	}
}

// TestBuildAdaptivePriorWithReport_NoQualifyingObs confirms that a
// report without high-strength ObsUncertainty under CatSystem does not
// shift the prior.
func TestBuildAdaptivePriorWithReport_NoQualifyingObs(t *testing.T) {
	report := &orchtypes.UncertaintyReport{
		SessionID: "sess_no_qualifying",
		Observations: []orchtypes.Observation{
			// CatBusiness, not CatSystem — must be ignored
			newObs(orchtypes.ObsDeviation, orchtypes.CatBusiness, 0.95, "obs1"),
			// CatSystem + ObsUncertainty but below threshold (< 0.7) — ignored
			newObs(orchtypes.ObsUncertainty, orchtypes.CatSystem, 0.5, "obs2"),
			// CatSystem + ObsDeviation — different kind, ignored
			newObs(orchtypes.ObsDeviation, orchtypes.CatSystem, 0.9, "obs3"),
		},
	}
	base := learn.BuildAdaptivePrior(nil, reputation.TrackModeDeveloper)
	got := orchtypes.BuildAdaptivePriorWithReport(nil, reputation.TrackModeDeveloper, report)
	if got.PriorBeta != base.PriorBeta {
		t.Fatalf("no qualifying obs must equal 2-arg prior: got Beta(%d,%d), want Beta(%d,%d)",
			got.PriorBeta.Alpha, got.PriorBeta.Beta,
			base.PriorBeta.Alpha, base.PriorBeta.Beta)
	}
}

// TestBuildAdaptivePriorWithReport_PenaltyShiftsPriorDown verifies that
// 2 high-strength ObsUncertainty+CatSystem observations (each 0.85)
// shift the Beta prior: penalty = 1.7 → -2 alpha, +2 beta.
func TestBuildAdaptivePriorWithReport_PenaltyShiftsPriorDown(t *testing.T) {
	report := &orchtypes.UncertaintyReport{
		SessionID: "sess_high_uncertainty",
		Observations: []orchtypes.Observation{
			newObs(orchtypes.ObsUncertainty, orchtypes.CatSystem, 0.85, "obs_a"),
			newObs(orchtypes.ObsUncertainty, orchtypes.CatSystem, 0.80, "obs_b"),
		},
	}
	got := orchtypes.BuildAdaptivePriorWithReport(nil, reputation.TrackModeDeveloper, report)
	base := learn.BuildAdaptivePrior(nil, reputation.TrackModeDeveloper)
	wantAlpha := base.PriorBeta.Alpha - 2
	if wantAlpha < 1 {
		wantAlpha = 1
	}
	wantBeta := base.PriorBeta.Beta + 2
	if got.PriorBeta.Alpha != wantAlpha || got.PriorBeta.Beta != wantBeta {
		t.Fatalf("penalty must shift Beta(%d,%d) -> Beta(%d,%d), got Beta(%d,%d)",
			base.PriorBeta.Alpha, base.PriorBeta.Beta,
			wantAlpha, wantBeta,
			got.PriorBeta.Alpha, got.PriorBeta.Beta)
	}
	// Sanity: mean should be lower than baseline mean (penalty depresses
	// confidence in developer track).
	if got.PriorBeta.Mean() >= base.PriorBeta.Mean() {
		t.Errorf("penalty must depress mean: got %.3f, baseline %.3f",
			got.PriorBeta.Mean(), base.PriorBeta.Mean())
	}
}

// TestBuildAdaptivePriorWithReport_AlphaFloorAt1 confirms the
// defensive guard: even with extreme penalty, Alpha never drops below
// 1 (avoids degenerate Beta(0, n) = 0 prior).
func TestBuildAdaptivePriorWithReport_AlphaFloorAt1(t *testing.T) {
	// Stack 20 high-strength signals to force penalty > base Alpha.
	obs := make([]orchtypes.Observation, 20)
	for i := range obs {
		obs[i] = newObs(orchtypes.ObsUncertainty, orchtypes.CatSystem, 1.0, "obs")
	}
	report := &orchtypes.UncertaintyReport{
		SessionID:    "sess_floor",
		Observations: obs,
	}
	got := orchtypes.BuildAdaptivePriorWithReport(nil, reputation.TrackModeDeveloper, report)
	if got.PriorBeta.Alpha < 1 {
		t.Fatalf("Alpha must floor at 1: got Alpha=%d", got.PriorBeta.Alpha)
	}
}

// helper — builds an Observation with the given kind/category/strength.
func newObs(kind orchtypes.ObservationKind, cat orchtypes.Category, strength float64, id string) orchtypes.Observation {
	return orchtypes.Observation{
		ID:         id,
		Kind:       kind,
		Category:   cat,
		Strength:   strength,
		Payload:    orchtypes.FactPayload{Statement: "test"},
		DetectedAt: time.Now(),
		Source:     "test",
	}
}
