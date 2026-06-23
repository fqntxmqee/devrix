package orchtypes

import (
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

func mkObs(kind ObservationKind, cat Category, strength float64, payload Payload) Observation {
	p := payload
	if p == nil {
		p = mkObsPayload(kind)
	}
	obs, _ := NewObservation(kind, cat, strength, p, "test_source")
	return obs
}

// mkReport builds an UncertaintyReport with the given anomalies for testing
// SystemAnomaly wiring. Pass nil for anomalies to default to empty.
func mkReport(anomalies []Observation) UncertaintyReport {
	obs := []Observation{}
	if len(anomalies) > 0 {
		obs = anomalies
	}
	rep, err := NewUncertaintyReport("session_test", obs)
	if err != nil {
		panic(err)
	}
	rep.Anomalies = anomalies
	return rep
}

// mkObsPayload returns a minimal non-nil payload appropriate for the given
// ObservationKind. Tests use nil as a sentinel for "don't care about the
// payload"; mkObs fills in a kind-correct placeholder so NewObservation
// doesn't reject the call.
func mkObsPayload(kind ObservationKind) Payload {
	switch kind {
	case ObsFact:
		return FactPayload{Statement: "test"}
	case ObsSignal:
		return SignalPayload{Name: "test", Value: 0}
	case ObsDeviation:
		return DeviationPayload{Metric: "test", Expected: 0, Observed: 0, Delta: 0}
	case ObsUncertainty:
		return UncertaintyPayload{Question: "test", Confidence: 0.5}
	default:
		return FactPayload{Statement: "test"}
	}
}

func TestEvaluateSystemAnomaly_HighCatSystem_Triggers(t *testing.T) {
	// 3 CatSystem ObsDeviation anomalies → triggers default config.
	anomalies := []Observation{
		mkObs(ObsDeviation, CatSystem, 0.9, nil),
		mkObs(ObsDeviation, CatSystem, 0.85, nil),
		mkObs(ObsDeviation, CatSystem, 0.95, nil),
	}
	rep := mkReport(anomalies)
	if !EvaluateSystemAnomaly(rep) {
		t.Error("EvaluateSystemAnomaly should trigger: 3 CatSystem anomalies ≥ threshold 3 AND ratio 1.0 ≥ 0.5")
	}
}

func TestEvaluateSystemAnomaly_LowCatSystem_NoTrigger(t *testing.T) {
	// 3 anomalies, 1 CatSystem → 33% ratio < 0.5 → no trigger.
	anomalies := []Observation{
		mkObs(ObsDeviation, CatSystem, 0.9, nil),
		mkObs(ObsDeviation, CatBusiness, 0.85, nil),
		mkObs(ObsDeviation, CatBusiness, 0.95, nil),
	}
	rep := mkReport(anomalies)
	if EvaluateSystemAnomaly(rep) {
		t.Error("EvaluateSystemAnomaly should not trigger: 33% CatSystem ratio < 0.5")
	}
}

func TestEvaluateSystemAnomaly_BelowThreshold_NoTrigger(t *testing.T) {
	// Only 2 anomalies → below default threshold 3 → no trigger.
	anomalies := []Observation{
		mkObs(ObsDeviation, CatSystem, 0.9, nil),
		mkObs(ObsDeviation, CatSystem, 0.85, nil),
	}
	rep := mkReport(anomalies)
	if EvaluateSystemAnomaly(rep) {
		t.Error("EvaluateSystemAnomaly should not trigger: 2 anomalies < threshold 3")
	}
}

func TestEvaluateSystemAnomaly_Empty_NoTrigger(t *testing.T) {
	rep := mkReport(nil)
	if EvaluateSystemAnomaly(rep) {
		t.Error("EvaluateSystemAnomaly should not trigger on empty report")
	}
}

func TestBuildUncertaintyCoordFromReport_NormalCase(t *testing.T) {
	anomalies := []Observation{
		mkObs(ObsDeviation, CatBusiness, 0.5, nil),
		mkObs(ObsDeviation, CatBusiness, 0.6, nil),
	}
	rep := mkReport(anomalies)
	verifier := workmodel.Verdict{
		Kind:       types.VerdictPass,
		Confidence: 0.9,
		Reason:     "all_good",
		SourceID:   "verifier_1",
	}
	coord, err := BuildUncertaintyCoordFromReport(rep, verifier)
	if err != nil {
		t.Fatalf("BuildUncertaintyCoordFromReport returned error: %v", err)
	}
	if coord.Value != 0.0 {
		t.Errorf("Value = %f, want 0.0 (VerdictPass baseline)", coord.Value)
	}
	if !coord.FromVerifier {
		t.Error("FromVerifier should be true")
	}
	if coord.Reason != "all_good" {
		t.Errorf("Reason = %q, want %q", coord.Reason, "all_good")
	}
}

func TestBuildUncertaintyCoordFromReport_SystemAnomalyOverrides_Value(t *testing.T) {
	// 3 CatSystem anomalies → SystemAnomaly=true → Value forced to 0.95.
	anomalies := []Observation{
		mkObs(ObsDeviation, CatSystem, 0.9, nil),
		mkObs(ObsDeviation, CatSystem, 0.85, nil),
		mkObs(ObsDeviation, CatSystem, 0.95, nil),
	}
	rep := mkReport(anomalies)
	verifier := workmodel.Verdict{
		Kind:       types.VerdictPass, // even Pass → Value forced to 0.95 by SystemAnomaly
		Confidence: 0.9,
		Reason:     "all_good_but_system_anomaly",
		SourceID:   "verifier_1",
	}
	coord, err := BuildUncertaintyCoordFromReport(rep, verifier)
	if err != nil {
		t.Fatalf("BuildUncertaintyCoordFromReport returned error: %v", err)
	}
	if coord.Value != 0.95 {
		t.Errorf("Value = %f, want 0.95 (SystemAnomaly override)", coord.Value)
	}
	if !coord.FromVerifier {
		t.Error("FromVerifier should be true")
	}
	if coord.Reason != "all_good_but_system_anomaly" {
		t.Errorf("Reason = %q, want %q", coord.Reason, "all_good_but_system_anomaly")
	}
}

func TestBuildUncertaintyCoordFromReport_InvalidVerdictKind_ReturnsError(t *testing.T) {
	rep := mkReport(nil)
	verifier := workmodel.Verdict{
		Kind:       types.VerdictKind(99), // out of range
		Confidence: 0.5,
		Reason:     "unknown",
	}
	_, err := BuildUncertaintyCoordFromReport(rep, verifier)
	if err == nil {
		t.Fatal("BuildUncertaintyCoordFromReport should fail on unknown VerdictKind")
	}
	if !strings.Contains(err.Error(), "unknown verdict kind") {
		t.Errorf("error should mention unknown verdict kind: %v", err)
	}
}

func TestBuildUncertaintyCoordFromReport_4VerdictKindValues(t *testing.T) {
	// Cover all 4 VerdictKind values via BuildUncertaintyCoordFromReport.
	rep := mkReport(nil)
	cases := []struct {
		kind types.VerdictKind
		want float64
	}{
		{types.VerdictPass, 0.0},
		{types.VerdictPartial, 0.4},
		{types.VerdictIndeterminate, 0.7},
		{types.VerdictFail, 0.9},
	}
	for _, tc := range cases {
		verifier := workmodel.Verdict{Kind: tc.kind, Confidence: 0.8}
		coord, err := BuildUncertaintyCoordFromReport(rep, verifier)
		if err != nil {
			t.Errorf("BuildUncertaintyCoordFromReport(%v) returned error: %v", tc.kind, err)
			continue
		}
		if coord.Value != tc.want {
			t.Errorf("BuildUncertaintyCoordFromReport(%v) Value = %f, want %f", tc.kind, coord.Value, tc.want)
		}
	}
}

func TestBuildUncertaintyCoordFromReport_ExtractedAtPopulated(t *testing.T) {
	rep := mkReport(nil)
	verifier := workmodel.Verdict{Kind: types.VerdictPass}
	before := time.Now()
	coord, err := BuildUncertaintyCoordFromReport(rep, verifier)
	if err != nil {
		t.Fatalf("BuildUncertaintyCoordFromReport returned error: %v", err)
	}
	if coord.UpdatedAt.Before(before) {
		t.Errorf("UpdatedAt (%v) should be ≥ before (%v)", coord.UpdatedAt, before)
	}
}
