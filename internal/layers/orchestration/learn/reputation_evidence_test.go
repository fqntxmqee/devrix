package learn

import (
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestParseTrackMode_2Values(t *testing.T) {
	cases := []struct {
		in   string
		want TrackMode
	}{
		{"developer", TrackModeDeveloper},
		{"operator", TrackModeOperator},
	}
	for _, tc := range cases {
		got, err := ParseTrackMode(tc.in)
		if err != nil {
			t.Errorf("ParseTrackMode(%q) returned error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseTrackMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseTrackMode_UnknownFailFast(t *testing.T) {
	_, err := ParseTrackMode("admin")
	if err == nil {
		t.Fatal(`ParseTrackMode("admin") should return error`)
	}
	if !strings.Contains(err.Error(), "unknown track mode") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewReputationEvidence_DefaultZero(t *testing.T) {
	rep, err := NewReputationEvidence("sess_1", TrackModeDeveloper)
	if err != nil {
		t.Fatalf("NewReputationEvidence: %v", err)
	}
	if rep.Alpha != 0 || rep.Beta != 0 {
		t.Errorf("Alpha/Beta = %d/%d, want 0/0", rep.Alpha, rep.Beta)
	}
	if rep.Mean != 0 || rep.Variance != 0 {
		t.Errorf("Mean/Variance = %g/%g, want 0/0", rep.Mean, rep.Variance)
	}
	if rep.ConfidenceLow != 0 || rep.ConfidenceHigh != 1 {
		t.Errorf("ConfidenceLow/High = %g/%g, want 0/1", rep.ConfidenceLow, rep.ConfidenceHigh)
	}
	if rep.UpdateCount != 0 {
		t.Errorf("UpdateCount = %d, want 0", rep.UpdateCount)
	}
	if rep.VerifierFailureCount != 0 {
		t.Errorf("VerifierFailureCount = %d, want 0", rep.VerifierFailureCount)
	}
	if rep.IndeterminateCount != 0 {
		t.Errorf("IndeterminateCount = %d, want 0", rep.IndeterminateCount)
	}
	if rep.SessionID != "sess_1" {
		t.Errorf("SessionID = %q, want sess_1", rep.SessionID)
	}
	if rep.TrackMode != TrackModeDeveloper {
		t.Errorf("TrackMode = %q, want developer", rep.TrackMode)
	}
	if len(rep.SourceVerdictIDs) != 0 {
		t.Errorf("SourceVerdictIDs = %v, want empty", rep.SourceVerdictIDs)
	}
}

func TestNewReputationEvidence_RequiredFieldsFailFast(t *testing.T) {
	cases := []struct {
		name      string
		sessionID string
		trackMode TrackMode
	}{
		{"empty_session", "", TrackModeDeveloper},
		{"unknown_trackmode", "sess_1", TrackMode("admin")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := NewReputationEvidence(tc.sessionID, tc.trackMode)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "reputation store unavailable") {
				t.Errorf("unexpected error: %v", err)
			}
			if rep != nil {
				t.Error("expected nil rep on error")
			}
		})
	}
}

func TestNewReputationEvidence_AutoTimestamp(t *testing.T) {
	before := time.Now()
	rep, err := NewReputationEvidence("sess_1", TrackModeDeveloper)
	after := time.Now()
	if err != nil {
		t.Fatalf("NewReputationEvidence: %v", err)
	}
	if rep.LastUpdated.Before(before) || rep.LastUpdated.After(after) {
		t.Errorf("LastUpdated = %v, want between %v and %v", rep.LastUpdated, before, after)
	}
}

func TestBayesianUpdate_VerdictPass_IncrementsAlpha(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	verdict := workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_1"}

	next := BayesianUpdate(prior, verdict)

	if next.Alpha != 1 || next.Beta != 0 {
		t.Errorf("Alpha/Beta = %d/%d, want 1/0", next.Alpha, next.Beta)
	}
	if next.Mean != 1.0 {
		t.Errorf("Mean = %g, want 1.0", next.Mean)
	}
	if next.UpdateCount != 1 {
		t.Errorf("UpdateCount = %d, want 1", next.UpdateCount)
	}
	if len(next.SourceVerdictIDs) != 1 || next.SourceVerdictIDs[0] != "v_1" {
		t.Errorf("SourceVerdictIDs = %v, want [v_1]", next.SourceVerdictIDs)
	}
}

func TestBayesianUpdate_VerdictPartial_IncrementsAlpha(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	verdict := workmodel.Verdict{Kind: types.VerdictPartial, SourceID: "v_1"}

	next := BayesianUpdate(prior, verdict)

	if next.Alpha != 1 || next.Beta != 0 {
		t.Errorf("Partial should increment Alpha; Alpha/Beta = %d/%d", next.Alpha, next.Beta)
	}
}

func TestBayesianUpdate_VerdictFail_IncrementsBeta(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	verdict := workmodel.Verdict{Kind: types.VerdictFail, SourceID: "v_1"}

	next := BayesianUpdate(prior, verdict)

	if next.Alpha != 0 || next.Beta != 1 {
		t.Errorf("Alpha/Beta = %d/%d, want 0/1", next.Alpha, next.Beta)
	}
}

func TestBayesianUpdate_VerdictIndeterminate_OtherReason_NotPollutes(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	verdict := workmodel.Verdict{
		Kind:               types.VerdictIndeterminate,
		IndeterminateReason: "env_limited",
		SourceID:           "v_1",
	}

	next := BayesianUpdate(prior, verdict)

	// Other INDETERMINATE: do NOT update α/β, only IndeterminateCount
	if next.Alpha != 0 || next.Beta != 0 {
		t.Errorf("env_limited INDETERMINATE should not touch α/β; got Alpha/Beta = %d/%d", next.Alpha, next.Beta)
	}
	if next.IndeterminateCount != 1 {
		t.Errorf("IndeterminateCount = %d, want 1", next.IndeterminateCount)
	}
	if next.VerifierFailureCount != 0 {
		t.Errorf("VerifierFailureCount = %d, want 0", next.VerifierFailureCount)
	}
}

func TestBayesianUpdate_VerdictIndeterminate_VerifierParseFailure_OnlyIncrementsVerifierFailureCount(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	verdict := workmodel.Verdict{
		Kind:                types.VerdictIndeterminate,
		IndeterminateReason: "verifier_parse_failure",
		SourceID:            "v_1",
	}

	next := BayesianUpdate(prior, verdict)

	// ⭐G8-1 fix: verifier_parse_failure must NOT pollute α/β
	if next.Alpha != 0 || next.Beta != 0 {
		t.Errorf("verifier_parse_failure must not touch α/β; got Alpha/Beta = %d/%d", next.Alpha, next.Beta)
	}
	if next.VerifierFailureCount != 1 {
		t.Errorf("VerifierFailureCount = %d, want 1", next.VerifierFailureCount)
	}
	if next.IndeterminateCount != 0 {
		t.Errorf("IndeterminateCount = %d, want 0 (this is verifier failure, not env-limited)", next.IndeterminateCount)
	}
}

func TestBayesianUpdate_ColdStartZeroAlphaBeta_KeepsPriorMean(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	// Cold start: prior.Mean = 0 (default NewReputationEvidence)
	verdict := workmodel.Verdict{Kind: types.VerdictIndeterminate, SourceID: "v_1"}

	next := BayesianUpdate(prior, verdict)

	// Cold start α=β=0 → Mean should preserve prior.Mean (0)
	if next.Mean != 0 {
		t.Errorf("cold start Mean = %g, want 0", next.Mean)
	}
	if next.Variance != 0 {
		t.Errorf("cold start Variance = %g, want 0", next.Variance)
	}
	if next.ConfidenceLow != 0 || next.ConfidenceHigh != 1 {
		t.Errorf("cold start ConfidenceLow/High = %g/%g, want 0/1", next.ConfidenceLow, next.ConfidenceHigh)
	}
}

func TestBayesianUpdate_Convergence50Passes(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	current := prior
	for i := 0; i < 50; i++ {
		v := workmodel.Verdict{
			Kind:     types.VerdictPass,
			SourceID: "v_" + string(rune('a'+i%26)),
		}
		current = BayesianUpdate(current, v)
	}
	// 50 PASS → Alpha=50, Beta=0 → Mean = 1.0
	if current.Alpha != 50 {
		t.Errorf("Alpha = %d, want 50", current.Alpha)
	}
	if current.Mean != 1.0 {
		t.Errorf("Mean = %g, want 1.0", current.Mean)
	}
}

func TestBayesianUpdate_DoesNotMutatePrior(t *testing.T) {
	prior, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	originalAlpha := prior.Alpha
	originalBeta := prior.Beta
	originalUpdateCount := prior.UpdateCount

	verdict := workmodel.Verdict{Kind: types.VerdictPass, SourceID: "v_1"}
	_ = BayesianUpdate(prior, verdict)

	if prior.Alpha != originalAlpha {
		t.Errorf("prior.Alpha mutated: %d → %d", originalAlpha, prior.Alpha)
	}
	if prior.Beta != originalBeta {
		t.Errorf("prior.Beta mutated: %d → %d", originalBeta, prior.Beta)
	}
	if prior.UpdateCount != originalUpdateCount {
		t.Errorf("prior.UpdateCount mutated: %d → %d", originalUpdateCount, prior.UpdateCount)
	}
}

func TestWilsonScoreInterval_BoundsInRange(t *testing.T) {
	// 50 passes, 10 fails → α=50, β=10, p̂≈0.833
	low, high := wilsonScoreInterval(50, 10, 0.95)
	if low < 0 || low > 1 {
		t.Errorf("low = %g, want in [0, 1]", low)
	}
	if high < 0 || high > 1 {
		t.Errorf("high = %g, want in [0, 1]", high)
	}
	if low > high {
		t.Errorf("low (%g) should be <= high (%g)", low, high)
	}
	// p̂ ≈ 0.833; CI should bracket it
	pHat := float64(50) / float64(60)
	if low > pHat || high < pHat {
		t.Errorf("CI [%g, %g] should bracket p̂=%g", low, high, pHat)
	}
}

func TestWilsonScoreInterval_ZeroCount(t *testing.T) {
	low, high := wilsonScoreInterval(0, 0, 0.95)
	if low != 0 || high != 1 {
		t.Errorf("zero count: low/high = %g/%g, want 0/1", low, high)
	}
}

func TestVerdict_WithIndeterminateReason(t *testing.T) {
	v := workmodel.Verdict{Kind: types.VerdictIndeterminate}
	v2 := v.WithIndeterminateReason("verifier_parse_failure")
	if v.IndeterminateReason != "" {
		t.Error("original Verdict should remain unchanged")
	}
	if v2.IndeterminateReason != "verifier_parse_failure" {
		t.Errorf("WithIndeterminateReason: got %q, want %q", v2.IndeterminateReason, "verifier_parse_failure")
	}
}

// Reference to types package (compile-time dependency).
var _ = types.VerdictKind(0)