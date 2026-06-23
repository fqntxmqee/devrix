package workmodel

import (
	"testing"
)

// fakeCategory is a minimal AnomalyCategory implementation for tests.
type fakeCategory struct {
	cat uint8
}

func (f fakeCategory) GetCategory() uint8 { return f.cat }

func TestSystemAnomalyAggregator_BelowThreshold_NoTrigger(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{}) // defaults: 3 anomalies, 0.5 ratio
	// Only 2 anomalies — below threshold.
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
	}
	if agg.Evaluate(anomalies) {
		t.Error("Evaluate should return false when anomalies < threshold")
	}
}

func TestSystemAnomalyAggregator_AboveThreshold_AllCatSystem_Triggers(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
	// 4 anomalies, all CatSystem → 100% ratio ≥ 0.5 → true.
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
	}
	if !agg.Evaluate(anomalies) {
		t.Error("Evaluate should return true: 4 CatSystem anomalies ≥ threshold AND ratio")
	}
}

func TestSystemAnomalyAggregator_AboveThreshold_MostlyCatBusiness_NoTrigger(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
	// 4 anomalies, 1 CatSystem → 25% ratio < 0.5 → false.
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatBusinessValue},
		fakeCategory{cat: CatBusinessValue},
		fakeCategory{cat: CatBusinessValue},
		fakeCategory{cat: CatSystemValue},
	}
	if agg.Evaluate(anomalies) {
		t.Error("Evaluate should return false: 25% CatSystem ratio < 0.5")
	}
}

func TestSystemAnomalyAggregator_AboveThreshold_HalfHalf_DefaultTriggers(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
	// 4 anomalies, 2 CatSystem → 50% ratio = 0.5 → true (boundary inclusive).
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatBusinessValue},
		fakeCategory{cat: CatBusinessValue},
	}
	if !agg.Evaluate(anomalies) {
		t.Error("Evaluate should return true at boundary 50% ratio")
	}
}

func TestSystemAnomalyAggregator_EmptyInput_NoTrigger(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
	if agg.Evaluate(nil) {
		t.Error("Evaluate(nil) should return false")
	}
	if agg.Evaluate([]AnomalyCategory{}) {
		t.Error("Evaluate([]) should return false")
	}
}

func TestSystemAnomalyAggregator_RecordCatSystem_Accumulates(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
	if agg.CatSystemCount() != 0 {
		t.Errorf("initial CatSystemCount = %d, want 0", agg.CatSystemCount())
	}
	agg.RecordCatSystem(3)
	agg.RecordCatSystem(2)
	if got := agg.CatSystemCount(); got != 5 {
		t.Errorf("CatSystemCount after Record(3) + Record(2) = %d, want 5", got)
	}
}

func TestSystemAnomalyAggregator_Reset_ClearsState(t *testing.T) {
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{})
	agg.RecordCatSystem(10)
	if agg.CatSystemCount() != 10 {
		t.Errorf("CatSystemCount = %d, want 10", agg.CatSystemCount())
	}
	agg.Reset()
	if agg.CatSystemCount() != 0 {
		t.Errorf("CatSystemCount after Reset = %d, want 0", agg.CatSystemCount())
	}
	// Config preserved.
	agg.RecordCatSystem(1)
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
	}
	if !agg.Evaluate(anomalies) {
		t.Error("Evaluate should still trigger after Reset (config preserved)")
	}
}

func TestSystemAnomalyAggregator_CustomThresholds(t *testing.T) {
	// Custom: AnomalyThreshold=5, MinCatSystemRatio=0.8.
	agg := NewSystemAnomalyAggregator(SystemAnomalyConfig{
		AnomalyThreshold:  5,
		MinCatSystemRatio: 0.8,
	})
	// 4 anomalies → below custom threshold 5 → false.
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
	}
	if agg.Evaluate(anomalies) {
		t.Error("Evaluate should return false: 4 < custom threshold 5")
	}
	// 5 anomalies, 4 CatSystem (80%) → triggers.
	anomalies = []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatBusinessValue},
	}
	if !agg.Evaluate(anomalies) {
		t.Error("Evaluate should return true: 5 anomalies ≥ 5 AND 80% CatSystem ≥ 0.8")
	}
}

func TestEvaluateSystemAnomalyFromCategories_Defaults(t *testing.T) {
	// Stateless convenience wrapper should behave like default-config aggregator.
	anomalies := []AnomalyCategory{
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
		fakeCategory{cat: CatSystemValue},
	}
	if !EvaluateSystemAnomalyFromCategories(anomalies) {
		t.Error("stateless wrapper should trigger for 3 CatSystem anomalies")
	}
	anomalies = []AnomalyCategory{
		fakeCategory{cat: CatBusinessValue},
		fakeCategory{cat: CatBusinessValue},
	}
	if EvaluateSystemAnomalyFromCategories(anomalies) {
		t.Error("stateless wrapper should not trigger for 2 CatBusiness anomalies")
	}
}
