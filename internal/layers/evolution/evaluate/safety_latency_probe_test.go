package evaluate

import (
	"context"
	"testing"
)

// T: D6-S3-A01-T22
func TestSafetyLatencyProbe_p99_under_1ms_passes(t *testing.T) {
	probe := &SafetyLatencyProbe{}
	// 100 samples all at 0.5ms; P99 = 0.5ms < 1ms target.
	durs := make([]float64, 100)
	for i := range durs {
		durs[i] = 0.5
	}
	item := EvalItem{
		ID:        "s-1",
		Dimension: "safety_latency",
		Input:     map[string]any{"durations_ms": durs},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 (P99 < 1ms)", score.Score)
	}
	if got := score.Details["p99_ms"]; got >= 1.0 {
		t.Errorf("p99_ms = %v, want < 1.0", got)
	}
}

func TestSafetyLatencyProbe_p99_in_yellow_band(t *testing.T) {
	probe := &SafetyLatencyProbe{}
	// 98 samples at 0ms + 2 at 1.5ms → sorted P99 = sample at index int(99*0.99)=98
	// = the first 1.5ms sample → P99 = 1.5ms (in [1, 2) yellow band).
	durs := make([]float64, 100)
	for i := 0; i < 98; i++ {
		durs[i] = 0.0
	}
	durs[98] = 1.5
	durs[99] = 1.5
	item := EvalItem{
		ID:    "s-2",
		Input: map[string]any{"durations_ms": durs},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 0.5 {
		t.Errorf("Score = %v, want 0.5 (yellow)", score.Score)
	}
	if got := score.Details["severity"]; got != 1.0 {
		t.Errorf("severity = %v, want 1.0", got)
	}
}

func TestSafetyLatencyProbe_p99_above_2ms_red(t *testing.T) {
	probe := &SafetyLatencyProbe{}
	// 98 samples at 0ms + 2 at 5ms → P99 = 5ms ≥ 2ms (red).
	durs := make([]float64, 100)
	for i := 0; i < 98; i++ {
		durs[i] = 0.0
	}
	durs[98] = 5.0
	durs[99] = 5.0
	item := EvalItem{
		ID:    "s-3",
		Input: map[string]any{"durations_ms": durs},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0 (red)", score.Score)
	}
	if got := score.Details["severity"]; got != 2.0 {
		t.Errorf("severity = %v, want 2.0", got)
	}
}

func TestSafetyLatencyProbe_insufficient_samples_passes_with_warning(t *testing.T) {
	probe := &SafetyLatencyProbe{}
	durs := []float64{0.5, 1.0, 0.7} // only 3 samples
	item := EvalItem{
		ID:    "s-4",
		Input: map[string]any{"durations_ms": durs},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 (insufficient samples → pass with low confidence)", score.Score)
	}
	if got := score.Details["insufficient_samples"]; got != 1.0 {
		t.Errorf("insufficient_samples marker = %v, want 1.0", got)
	}
}

func TestSafetyLatencyProbe_robust_to_unsorted_input(t *testing.T) {
	probe := &SafetyLatencyProbe{}
	// Intentionally unsorted to verify percentile sorts internally.
	durs := []float64{5.0, 0.1, 0.3, 0.0, 0.2, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 0.05, 0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 100.0}
	// expand to ≥100 samples
	for len(durs) < 100 {
		durs = append(durs, 0.0)
	}
	item := EvalItem{ID: "s-5", Input: map[string]any{"durations_ms": durs}}
	score, _ := probe.Run(context.Background(), item, nil)
	// Sorted P99 with one 100ms outlier + 99 zero samples → P99 = 100ms (red)
	if score.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0 (P99 includes the 100ms outlier)", score.Score)
	}
}
