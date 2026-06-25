package orchtypes

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
)

func TestAnomalyDetector_Detect_Baseline_Triggered(t *testing.T) {
	d := NewAnomalyDetector()
	anomalies := []Anomaly{
		{Category: AnomalyCategoryRate, Severity: 0.7, Evidence: "100 msg/s"},
		{Category: AnomalyCategoryPattern, Severity: 0.3, Evidence: "repeated"},
	}
	r, err := d.Detect(context.Background(), anomalies)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !r.TriggeredSystemAnomaly {
		t.Errorf("TriggeredSystemAnomaly = false, want true (0.7 >= 0.5)")
	}
	if r.Severity != 0.7 {
		t.Errorf("Severity = %.3f, want 0.7", r.Severity)
	}
	if r.Threshold != 0.5 {
		t.Errorf("Threshold = %.3f, want 0.5 (baseline)", r.Threshold)
	}
}

func TestAnomalyDetector_Detect_Baseline_NotTriggered(t *testing.T) {
	d := NewAnomalyDetector()
	anomalies := []Anomaly{
		{Category: AnomalyCategoryRate, Severity: 0.3, Evidence: "mild"},
	}
	r, err := d.Detect(context.Background(), anomalies)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if r.TriggeredSystemAnomaly {
		t.Errorf("TriggeredSystemAnomaly = true, want false (0.3 < 0.5)")
	}
}

func TestAnomalyDetector_Detect_EmptyInput(t *testing.T) {
	d := NewAnomalyDetector()
	r, err := d.Detect(context.Background(), []Anomaly{})
	if err != nil {
		t.Fatalf("Detect empty: %v", err)
	}
	if r.TriggeredSystemAnomaly {
		t.Errorf("TriggeredSystemAnomaly = true, want false (no anomalies)")
	}
	if r.Severity != 0 {
		t.Errorf("Severity = %.3f, want 0", r.Severity)
	}
}

func TestAnomalyDetector_Detect_InvalidSeverity(t *testing.T) {
	d := NewAnomalyDetector()
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 1.5, Evidence: "bad"}}
	_, err := d.Detect(context.Background(), anomalies)
	if err == nil {
		t.Fatal("Detect with severity > 1: expected error, got nil")
	}
}

func TestAnomalyDetector_DetectWithPrior_HighMean_HigherThreshold(t *testing.T) {
	d := NewAnomalyDetector()
	// prior Beta(8,1) → Mean = 0.889 → threshold = 0.5 * 0.889 = 0.4445
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.4, Evidence: "mild"}}
	r, err := d.DetectWithPrior(context.Background(), anomalies, prior)
	if err != nil {
		t.Fatalf("DetectWithPrior: %v", err)
	}
	want := 0.5 * 8.0 / 9.0
	if r.Threshold < want-0.01 || r.Threshold > want+0.01 {
		t.Errorf("Threshold = %.3f, want %.3f (0.5 * Beta(8,1).Mean)", r.Threshold, want)
	}
	// 0.4 < 0.4445 → not triggered (compared to baseline would have triggered since 0.4 < 0.5 → also not triggered here, but
	// for a Severity 0.45, baseline would not trigger (0.45 < 0.5), but with higher prior, threshold is 0.4445 → still not triggered.
	// Test 0.45: baseline not triggered; with prior Beta(8,1) threshold 0.4445 → 0.45 >= 0.4445 → TRIGGERED.
	anomalies2 := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.45, Evidence: "borderline"}}
	r2, _ := d.DetectWithPrior(context.Background(), anomalies2, prior)
	if !r2.TriggeredSystemAnomaly {
		t.Errorf("With prior Beta(8,1) threshold=0.4445, Severity 0.45 should trigger; got false")
	}
	// Sanity: baseline (threshold 0.5) would NOT trigger 0.45
	rBase, _ := d.Detect(context.Background(), anomalies2)
	if rBase.TriggeredSystemAnomaly {
		t.Errorf("Baseline threshold 0.5 should not trigger Severity 0.45; got true")
	}
}

func TestAnomalyDetector_DetectWithPrior_LowMean_LowerThreshold(t *testing.T) {
	d := NewAnomalyDetector()
	// prior Beta(2,8) → Mean = 0.2 → threshold = 0.5 * 0.2 = 0.1
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	prior.PriorBeta = learn.BetaPrior{Alpha: 2, Beta: 8}
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.15, Evidence: "mild"}}
	r, err := d.DetectWithPrior(context.Background(), anomalies, prior)
	if err != nil {
		t.Fatalf("DetectWithPrior: %v", err)
	}
	want := 0.5 * 2.0 / 10.0
	if r.Threshold < want-0.01 || r.Threshold > want+0.01 {
		t.Errorf("Threshold = %.3f, want %.3f (0.5 * Beta(2,8).Mean)", r.Threshold, want)
	}
	if !r.TriggeredSystemAnomaly {
		t.Errorf("With prior Beta(2,8) threshold=0.1, Severity 0.15 should trigger; got false")
	}
}

func TestAnomalyDetector_DetectWithPrior_NilPrior_Baseline(t *testing.T) {
	d := NewAnomalyDetector()
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.7, Evidence: "x"}}
	r, err := d.DetectWithPrior(context.Background(), anomalies, nil)
	if err != nil {
		t.Fatalf("DetectWithPrior nil: %v", err)
	}
	if r.Threshold != 0.5 {
		t.Errorf("Threshold = %.3f, want 0.5 (baseline)", r.Threshold)
	}
}

func TestAnomalyDetector_DetectWithPrior_ColdStart_UsesDefaultDeveloperPrior(t *testing.T) {
	d := NewAnomalyDetector()
	// Cold start: BuildAdaptivePrior(nil, developer) = Beta(5,3) → Mean = 0.625
	// → threshold = 0.5 * 0.625 = 0.3125
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.7, Evidence: "x"}}
	r, err := d.DetectWithPrior(context.Background(), anomalies, prior)
	if err != nil {
		t.Fatalf("DetectWithPrior cold: %v", err)
	}
	want := 0.5 * 5.0 / 8.0
	if r.Threshold < want-0.01 || r.Threshold > want+0.01 {
		t.Errorf("Threshold = %.3f, want %.3f (0.5 × DefaultDeveloperPrior Mean)", r.Threshold, want)
	}
}

func TestAnomalyDetector_DetectWithPrior_ZeroMean_Baseline(t *testing.T) {
	d := NewAnomalyDetector()
	// Manually constructed prior with Mean=0 (e.g. adversarial injection)
	prior := &learn.AdaptivePrior{
		PriorBeta: learn.BetaPrior{Alpha: 0, Beta: 0},
	}
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.7, Evidence: "x"}}
	r, err := d.DetectWithPrior(context.Background(), anomalies, prior)
	if err != nil {
		t.Fatalf("DetectWithPrior zero: %v", err)
	}
	if r.Threshold != 0.5 {
		t.Errorf("Threshold = %.3f, want 0.5 (zero-mean → baseline)", r.Threshold)
	}
}

func TestAnomalyDetector_DetectWithPrior_NoPriorMutation(t *testing.T) {
	d := NewAnomalyDetector()
	prior := learn.BuildAdaptivePrior(nil, learn.TrackModeOperator)
	alphaBefore := prior.PriorBeta.Alpha
	betaBefore := prior.PriorBeta.Beta
	anomalies := []Anomaly{{Category: AnomalyCategoryRate, Severity: 0.7, Evidence: "x"}}
	_, _ = d.DetectWithPrior(context.Background(), anomalies, prior)
	if prior.PriorBeta.Alpha != alphaBefore {
		t.Errorf("Prior.Alpha mutated: %d → %d", alphaBefore, prior.PriorBeta.Alpha)
	}
	if prior.PriorBeta.Beta != betaBefore {
		t.Errorf("Prior.Beta mutated: %d → %d", betaBefore, prior.PriorBeta.Beta)
	}
}

func TestAnomalyDetector_HistoricalDetector_Alias(t *testing.T) {
	d := NewAnomalyDetector()
	hd := d.HistoricalDetector()
	if hd != d {
		t.Errorf("HistoricalDetector() should return the same AnomalyDetector pointer (alias)")
	}
}
