package learn

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestBetaPrior_String_BetaFormat(t *testing.T) {
	cases := []struct {
		p    BetaPrior
		want string
	}{
		{BetaPrior{Alpha: 5, Beta: 3}, "Beta(5,3)"},
		{BetaPrior{Alpha: 8, Beta: 1}, "Beta(8,1)"},
		{BetaPrior{Alpha: 0, Beta: 0}, "Beta(0,0)"},
	}
	for _, tc := range cases {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("BetaPrior%+v.String() = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestAdaptivePrior_Immutable(t *testing.T) {
	// Compile-time: AdaptivePrior has no setter methods.
	prior := &AdaptivePrior{
		Reputation: nil,
		PriorBeta:  BetaPrior{Alpha: 5, Beta: 3},
	}
	if prior.Reputation != nil {
		t.Error("Reputation should be nil for cold start")
	}
	if prior.PriorBeta.Alpha != 5 || prior.PriorBeta.Beta != 3 {
		t.Errorf("PriorBeta = %+v, want Beta(5,3)", prior.PriorBeta)
	}
}

func TestBuildAdaptivePrior_DeveloperMode_DefaultDeveloperPrior(t *testing.T) {
	rep, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	rep.Alpha = 10
	rep.Beta = 2

	ap := BuildAdaptivePrior(rep, TrackModeDeveloper)

	// Developer: Beta(5,3) + rep(10,2) = Beta(15, 5)
	want := BetaPrior{Alpha: 15, Beta: 5}
	if ap.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", ap.PriorBeta, want)
	}
	if ap.Reputation != rep {
		t.Error("Reputation field should reference the input rep")
	}
}

func TestBuildAdaptivePrior_OperatorMode_DefaultOperatorPrior(t *testing.T) {
	rep, _ := NewReputationEvidence("sess_1", TrackModeOperator)
	rep.Alpha = 5
	rep.Beta = 1

	ap := BuildAdaptivePrior(rep, TrackModeOperator)

	// Operator: Beta(8,1) + rep(5,1) = Beta(13, 2)
	want := BetaPrior{Alpha: 13, Beta: 2}
	if ap.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", ap.PriorBeta, want)
	}
}

func TestBuildAdaptivePrior_NilReputation_UseDefaultPrior(t *testing.T) {
	ap := BuildAdaptivePrior(nil, TrackModeDeveloper)

	if ap.Reputation != nil {
		t.Error("Reputation should be nil for cold start")
	}
	// Cold start: just Developer Beta(5,3)
	want := BetaPrior{Alpha: 5, Beta: 3}
	if ap.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", ap.PriorBeta, want)
	}
}

func TestBuildAdaptivePrior_EmptyTrackMode_DefaultDeveloper(t *testing.T) {
	// Empty trackMode → fail-safe to Developer prior
	rep, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	rep.Alpha = 3
	rep.Beta = 0

	ap := BuildAdaptivePrior(rep, TrackMode(""))

	// Empty → Developer Beta(5,3) + rep(3,0) = Beta(8,3)
	want := BetaPrior{Alpha: 8, Beta: 3}
	if ap.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", ap.PriorBeta, want)
	}
}

func TestBuildAdaptivePrior_UnknownTrackMode_FailSafe(t *testing.T) {
	rep, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	rep.Alpha = 1
	rep.Beta = 0

	ap := BuildAdaptivePrior(rep, TrackMode("unknown_mode"))

	// Unknown trackMode → fail-safe Developer Beta(5,3) + rep(1,0) = Beta(6,3)
	want := BetaPrior{Alpha: 6, Beta: 3}
	if ap.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", ap.PriorBeta, want)
	}
}

func TestBuildAdaptivePrior_BayesianMerge(t *testing.T) {
	rep, _ := NewReputationEvidence("sess_1", TrackModeDeveloper)
	rep.Alpha = 100
	rep.Beta = 20

	ap := BuildAdaptivePrior(rep, TrackModeDeveloper)

	// Developer Beta(5,3) + rep(100, 20) = Beta(105, 23)
	want := BetaPrior{Alpha: 105, Beta: 23}
	if ap.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", ap.PriorBeta, want)
	}
}

func TestBetaPrior_Mean(t *testing.T) {
	cases := []struct {
		p    BetaPrior
		want float64
	}{
		{BetaPrior{Alpha: 5, Beta: 3}, 5.0 / 8.0},
		{BetaPrior{Alpha: 8, Beta: 1}, 8.0 / 9.0},
		{BetaPrior{Alpha: 0, Beta: 0}, 0},   // cold start → 0 (fail-safe)
		{BetaPrior{Alpha: 1, Beta: 0}, 1.0}, // all positive
		{BetaPrior{Alpha: 0, Beta: 1}, 0.0}, // all negative
		{BetaPrior{Alpha: 100, Beta: 50}, 100.0 / 150.0},
	}
	for _, tc := range cases {
		got := tc.p.Mean()
		if got != tc.want {
			t.Errorf("BetaPrior%+v.Mean() = %f, want %f", tc.p, got, tc.want)
		}
	}
}

func TestBetaPrior_Mean_ColdStart_ZeroTotal(t *testing.T) {
	// When Alpha+Beta=0 (cold start), Mean must be 0 to prevent
	// downstream Observer submodules from multiplying by NaN/Inf.
	p := BetaPrior{Alpha: 0, Beta: 0}
	if got := p.Mean(); got != 0 {
		t.Errorf("Cold start Mean() = %f, want 0", got)
	}
}

// Reference to types package (compile-time dependency).
var _ = types.VerdictKind(0)