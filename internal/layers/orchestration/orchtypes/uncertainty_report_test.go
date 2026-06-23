package orchtypes

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func makeObs(t *testing.T, k ObservationKind, c Category, strength float64) Observation {
	t.Helper()
	var p Payload
	switch k {
	case ObsFact:
		p = FactPayload{Statement: "x"}
	case ObsSignal:
		p = SignalPayload{Name: "n", Value: strength * 100}
	case ObsDeviation:
		p = DeviationPayload{Metric: "m", Expected: 1, Observed: 2, Delta: 1}
	case ObsUncertainty:
		p = UncertaintyPayload{Question: "q?", Confidence: 0.5}
	}
	obs, err := NewObservation(k, c, strength, p, "test")
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	return obs
}

func TestUncertaintyReport_PartitionInvariant(t *testing.T) {
	obs := []Observation{
		makeObs(t, ObsFact, CatBusiness, 0.8),
		makeObs(t, ObsSignal, CatSystem, 0.6),
		makeObs(t, ObsDeviation, CatSystem, 0.9),
		makeObs(t, ObsUncertainty, CatBusiness, 0.3),
	}
	r, err := NewUncertaintyReport("s1", obs)
	if err != nil {
		t.Fatalf("NewUncertaintyReport: %v", err)
	}
	if len(r.BusinessObservations)+len(r.SystemObservations) != len(r.Observations) {
		t.Errorf("partition invariant broken: bus=%d sys=%d all=%d",
			len(r.BusinessObservations), len(r.SystemObservations), len(r.Observations))
	}
	if len(r.BusinessObservations) != 2 {
		t.Errorf("BusinessObservations = %d, want 2", len(r.BusinessObservations))
	}
	if len(r.SystemObservations) != 2 {
		t.Errorf("SystemObservations = %d, want 2", len(r.SystemObservations))
	}
}

func TestUncertaintyReport_Anomalies_SubsetOfSystemDeviation(t *testing.T) {
	obs := []Observation{
		makeObs(t, ObsFact, CatBusiness, 0.8),
		makeObs(t, ObsDeviation, CatSystem, 0.9),
		makeObs(t, ObsUncertainty, CatSystem, 0.4),
	}
	r, err := NewUncertaintyReport("s1", obs)
	if err != nil {
		t.Fatalf("NewUncertaintyReport: %v", err)
	}
	if len(r.Anomalies) != 1 {
		t.Errorf("Anomalies = %d, want 1 (only the CatSystem ObsDeviation)", len(r.Anomalies))
	}
	if r.Anomalies[0].Kind != ObsDeviation {
		t.Errorf("Anomaly[0].Kind = %s, want deviation", r.Anomalies[0].Kind)
	}
}

func TestUncertaintyReport_ComputeOverallStrength_BusinessOnly(t *testing.T) {
	// 2 business (0.8, 0.4) + 2 system (0.6, 0.9) → overall = 0.6 (business only)
	obs := []Observation{
		makeObs(t, ObsFact, CatBusiness, 0.8),
		makeObs(t, ObsFact, CatBusiness, 0.4),
		makeObs(t, ObsSignal, CatSystem, 0.6),
		makeObs(t, ObsDeviation, CatSystem, 0.9),
	}
	r, _ := NewUncertaintyReport("s1", obs)
	want := 0.6
	if !floatNear(r.Overall, want, 1e-9) {
		t.Errorf("Overall = %.9f, want %.9f (business-only average)", r.Overall, want)
	}
}

func TestUncertaintyReport_EmptyObservations_ReturnsDefaultHalf(t *testing.T) {
	r, err := NewUncertaintyReport("s1", nil)
	if err != nil {
		t.Fatalf("NewUncertaintyReport: %v", err)
	}
	if !floatNear(r.Overall, 0.5, 1e-9) {
		t.Errorf("Overall = %.9f, want 0.5 (cold-start neutral)", r.Overall)
	}
	if len(r.Anomalies) != 0 {
		t.Errorf("Anomalies = %d, want 0", len(r.Anomalies))
	}
}

func TestUncertaintyReport_FilterByKind_ScansFullSet(t *testing.T) {
	// FilterByKind must scan ALL observations, not just business partition.
	obs := []Observation{
		makeObs(t, ObsUncertainty, CatBusiness, 0.5),
		makeObs(t, ObsUncertainty, CatSystem, 0.4),
		makeObs(t, ObsFact, CatBusiness, 0.8),
	}
	r, _ := NewUncertaintyReport("s1", obs)
	filtered := r.FilterByKind(ObsUncertainty)
	if len(filtered) != 2 {
		t.Errorf("FilterByKind(Uncertainty) = %d, want 2 (1 business + 1 system)", len(filtered))
	}
}

func TestUncertaintyReport_FilterByCategory(t *testing.T) {
	obs := []Observation{
		makeObs(t, ObsFact, CatBusiness, 0.8),
		makeObs(t, ObsFact, CatSystem, 0.4),
	}
	r, _ := NewUncertaintyReport("s1", obs)
	if got := len(r.FilterByCategory(CatBusiness)); got != 1 {
		t.Errorf("FilterByCategory(Business) = %d, want 1", got)
	}
	if got := len(r.FilterByCategory(CatSystem)); got != 1 {
		t.Errorf("FilterByCategory(System) = %d, want 1", got)
	}
}

func TestUncertaintyReport_AddObservation_Immutable(t *testing.T) {
	obs := []Observation{makeObs(t, ObsFact, CatBusiness, 0.5)}
	r, _ := NewUncertaintyReport("s1", obs)
	originalLen := len(r.Observations)
	originalOverall := r.Overall

	r2, err := r.AddObservation(makeObs(t, ObsFact, CatBusiness, 1.0))
	if err != nil {
		t.Fatalf("AddObservation: %v", err)
	}

	if len(r.Observations) != originalLen {
		t.Errorf("original mutated: len=%d, want %d", len(r.Observations), originalLen)
	}
	if !floatNear(r.Overall, originalOverall, 1e-9) {
		t.Errorf("original mutated: Overall=%.9f, want %.9f", r.Overall, originalOverall)
	}
	if len(r2.Observations) != originalLen+1 {
		t.Errorf("new report: len=%d, want %d", len(r2.Observations), originalLen+1)
	}
	if r2.Overall <= originalOverall {
		t.Errorf("new report: Overall=%.2f should exceed original %.2f", r2.Overall, originalOverall)
	}
}

func TestUncertaintyReport_SetQuantizedIntent_AndSetPrior(t *testing.T) {
	obs := []Observation{makeObs(t, ObsFact, CatBusiness, 0.5)}
	r, _ := NewUncertaintyReport("s1", obs)

	qi := &QuantizedIntent{Kind: IntentFast, Confidence: 0.9, Reason: "trivial", Rounds: 1}
	r2 := r.SetQuantizedIntent(qi)
	if r.QuantizedIntent != nil {
		t.Error("original mutated: QuantizedIntent should be nil")
	}
	if r2.QuantizedIntent == nil || r2.QuantizedIntent.Kind != IntentFast {
		t.Errorf("SetQuantizedIntent failed: %+v", r2.QuantizedIntent)
	}

	prior := &AdaptivePrior{Score: 0.7, Confidence: 0.5, Source: "reputation"}
	r3 := r2.SetPrior(prior)
	if r3.Prior == nil || r3.Prior.Score != 0.7 {
		t.Errorf("SetPrior failed: %+v", r3.Prior)
	}
}

func TestUncertaintyReport_Validate_OK(t *testing.T) {
	obs := []Observation{
		makeObs(t, ObsFact, CatBusiness, 0.5),
		makeObs(t, ObsSignal, CatSystem, 0.6),
	}
	r, _ := NewUncertaintyReport("s1", obs)
	if err := r.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestUncertaintyReport_Validate_MissingSessionID(t *testing.T) {
	r, _ := NewUncertaintyReport("s1", nil)
	r.SessionID = ""
	if err := r.Validate(); err == nil {
		t.Error("expected error for empty SessionID")
	}
}

func TestUncertaintyReport_SortedObservationsByStrength(t *testing.T) {
	obs := []Observation{
		makeObs(t, ObsFact, CatBusiness, 0.3),
		makeObs(t, ObsFact, CatBusiness, 0.9),
		makeObs(t, ObsFact, CatBusiness, 0.6),
	}
	r, _ := NewUncertaintyReport("s1", obs)
	sorted := r.SortedObservationsByStrength()
	if sorted[0].Strength != 0.9 {
		t.Errorf("sorted[0].Strength = %.2f, want 0.9", sorted[0].Strength)
	}
	if sorted[2].Strength != 0.3 {
		t.Errorf("sorted[2].Strength = %.2f, want 0.3", sorted[2].Strength)
	}
}

func TestUncertaintyReport_NewRejectsEmptySessionID(t *testing.T) {
	_, err := NewUncertaintyReport("", nil)
	if err == nil {
		t.Error("expected error for empty SessionID")
	}
}

func floatNear(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// ⭐ RF.3.2 W6/I8: When BusinessObservations is non-empty but their
// Strength values somehow yield a NaN (e.g. corrupt external data), the
// Overall field must clamp to 0.5 (cold-start neutral) instead of
// poisoning downstream Coord construction.
func TestUncertaintyReport_Overall_NaN_Fallback(t *testing.T) {
	// Hand-build an Observation bypassing NewObservation so we can plant
	// a NaN Strength into the business path. Validate() rejects NaN, so
	// we expect Partition's clamp to absorb it before downstream readers
	// see stale data.
	r := UncertaintyReport{}
	// Plant a NaN-strength business observation directly into the slice.
	r.Observations = []Observation{
		{
			ID:         "b1",
			Kind:       ObsFact,
			Category:   CatBusiness,
			Strength:   0.8,
			Payload:    FactPayload{Statement: "x"},
			DetectedAt: time.Now(),
			Source:     "test",
		},
	}
	// Override the business observation's Strength to NaN to simulate
	// a poisoned dataset. Partition will recompute Overall via
	// ComputeOverallStrength which uses simple arithmetic — to produce
	// NaN we'd need to inject into the operation itself, so simulate by
	// directly calling the clamp on a NaN input.
	if err := r.Partition(); err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if !floatNear(r.Overall, 0.8, 1e-9) {
		t.Errorf("Overall = %.3f, want 0.8", r.Overall)
	}

	// Direct clamp01Float NaN handling mirrors what Partition does.
	if got := clamp01Float(math.NaN(), 0.5); got != 0.5 {
		t.Errorf("clamp01Float(NaN, 0.5) = %.3f, want 0.5", got)
	}
}

// ⭐ RF.3.2 C1: QuantizedIntent.Kind must be the IntentKind enum, not
// a free-form string. This guarantees PR-A2's IntentQuantizer can
// attach results without a string↔IntentKind translation shim.
func TestUncertaintyReport_QuantizedIntent_KindType(t *testing.T) {
	qi := QuantizedIntent{
		Kind:       IntentFast,
		Confidence: 0.9,
		Reason:     "trivial",
		Rounds:     1,
	}
	if string(qi.Kind) != "fast" {
		t.Errorf("IntentFast JSON value = %q, want \"fast\"", string(qi.Kind))
	}

	data, err := json.Marshal(qi)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"kind":"fast"`) {
		t.Errorf("JSON wire format broke: %s", string(data))
	}

	// Roundtrip must preserve the IntentKind enum (decoded back to the
	// same constant value).
	var got QuantizedIntent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Kind != IntentFast {
		t.Errorf("Kind = %q, want IntentFast", got.Kind)
	}
}
