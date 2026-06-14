package eval

import (
	"context"
	"testing"
)

// T: D6-S3-A01-T21
func TestBreakerAnomaly_no_traffic_passes(t *testing.T) {
	probe := &BreakerAnomalyTransitionProbe{}
	item := EvalItem{ID: "b-1", Input: map[string]any{}}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 (no traffic)", score.Score)
	}
}

func TestBreakerAnomaly_quiet_trajectory_passes(t *testing.T) {
	probe := &BreakerAnomalyTransitionProbe{}
	// 1 closed→open, 1 open→half-open, 1 half-open→closed over 5min — clean.
	item := EvalItem{
		ID: "b-2",
		Input: map[string]any{
			"transitions": []any{
				map[string]any{"provider": "deepseek", "from": "closed", "to": "open", "at": int64(0)},
				map[string]any{"provider": "deepseek", "from": "open", "to": "half-open", "at": int64(60)},
				map[string]any{"provider": "deepseek", "from": "half-open", "to": "closed", "at": int64(120)},
			},
		},
	}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 1.0 {
		t.Errorf("Score = %v, want 1.0", score.Score)
	}
}

func TestBreakerAnomaly_frequent_flip_yellow(t *testing.T) {
	probe := &BreakerAnomalyTransitionProbe{}
	// 4 transitions in 100s window — above the 3-flip yellow limit,
	// but NOT alternating and NOT half_open→open streaks (those are Red).
	ts := []any{
		map[string]any{"provider": "deepseek", "from": "closed", "to": "open", "at": int64(0)},
		map[string]any{"provider": "deepseek", "from": "open", "to": "half-open", "at": int64(30)},
		map[string]any{"provider": "deepseek", "from": "half-open", "to": "closed", "at": int64(60)},
		map[string]any{"provider": "deepseek", "from": "closed", "to": "open", "at": int64(90)},
	}
	item := EvalItem{ID: "b-3", Input: map[string]any{"transitions": ts}}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 0.5 {
		t.Errorf("Score = %v, want 0.5 (yellow)", score.Score)
	}
	if got := score.Details["severity"]; got != 1.0 {
		t.Errorf("severity = %v, want 1.0", got)
	}
}

func TestBreakerAnomaly_rapid_alternate_red(t *testing.T) {
	probe := &BreakerAnomalyTransitionProbe{}
	// closed→open, open→closed, closed→open within 30s of each other.
	ts := []any{
		map[string]any{"provider": "deepseek", "from": "closed", "to": "open", "at": int64(0)},
		map[string]any{"provider": "deepseek", "from": "open", "to": "closed", "at": int64(10)},
		map[string]any{"provider": "deepseek", "from": "closed", "to": "open", "at": int64(20)},
	}
	item := EvalItem{ID: "b-4", Input: map[string]any{"transitions": ts}}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0 (red)", score.Score)
	}
	if got := score.Details["severity"]; got != 2.0 {
		t.Errorf("severity = %v, want 2.0", got)
	}
}

func TestBreakerAnomaly_half_open_reopen_streak_red(t *testing.T) {
	probe := &BreakerAnomalyTransitionProbe{}
	// half_open→open twice in a row without a closed in between.
	ts := []any{
		map[string]any{"provider": "deepseek", "from": "closed", "to": "half-open", "at": int64(0)},
		map[string]any{"provider": "deepseek", "from": "half-open", "to": "open", "at": int64(30)},
		map[string]any{"provider": "deepseek", "from": "open", "to": "half-open", "at": int64(60)},
		map[string]any{"provider": "deepseek", "from": "half-open", "to": "open", "at": int64(90)},
	}
	item := EvalItem{ID: "b-5", Input: map[string]any{"transitions": ts}}
	score, _ := probe.Run(context.Background(), item, nil)
	if score.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0 (red: half_open→open x2)", score.Score)
	}
}
