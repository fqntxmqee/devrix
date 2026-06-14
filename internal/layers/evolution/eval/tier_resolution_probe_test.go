package eval

import (
	"context"
	"testing"
)

// T: D6-S3-A01-T20
func TestTierResolutionProbe_healthy_hit_above_99pct(t *testing.T) {
	probe := &TierResolutionProbe{}
	item := EvalItem{
		ID:        "tier-1",
		Bucket:    "production",
		Dimension: "tier_resolution",
		Input: map[string]any{
			"tier_hit":      int64(995),
			"tier_fallback": int64(4),
			"tier_error":    int64(1),
		},
	}
	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0", score.Score)
	}
	// Error > 0 still scores 1.0 (above 99%) but flags severity
	// via the 99% threshold alone — error triggers Red only when
	// hit_ratio is also below 99% per spec.
}

func TestTierResolutionProbe_just_below_99pct_yellow(t *testing.T) {
	probe := &TierResolutionProbe{}
	item := EvalItem{
		ID:        "tier-2",
		Bucket:    "production",
		Dimension: "tier_resolution",
		Input: map[string]any{
			"tier_hit":      int64(985),
			"tier_fallback": int64(15),
			"tier_error":    int64(0),
		},
	}
	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// hit_ratio = 985 / 1000 = 0.985 < 0.99 → Score = 0.985 (yellow)
	if score.Score >= 0.99 {
		t.Errorf("Score = %v, want < 0.99", score.Score)
	}
	if got := score.Details["severity"]; got != 1.0 {
		t.Errorf("severity = %v, want 1.0 (Yellow)", got)
	}
}

func TestTierResolutionProbe_error_above_99pct_still_passes(t *testing.T) {
	probe := &TierResolutionProbe{}
	item := EvalItem{
		ID:        "tier-3",
		Bucket:    "production",
		Dimension: "tier_resolution",
		Input: map[string]any{
			"tier_hit":      int64(1000),
			"tier_fallback": int64(0),
			"tier_error":    int64(5),
		},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 (hit_ratio = 1000/1005 ≈ 0.995 > 0.99)", score.Score)
	}
}

func TestTierResolutionProbe_no_traffic_passes_with_warning(t *testing.T) {
	probe := &TierResolutionProbe{}
	item := EvalItem{
		ID:        "tier-4",
		Dimension: "tier_resolution",
		Input:     map[string]any{},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 (no traffic → pass with warning)", score.Score)
	}
	if got := score.Details["no_traffic"]; got != 1.0 {
		t.Errorf("no_traffic marker = %v, want 1.0", got)
	}
}
