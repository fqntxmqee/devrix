package evaluate

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent/observability"
)

// T: D4-S3-A02-T05 (SessionIsolationProbe registration + scoring)
func TestSessionIsolationProbe_is_registered(t *testing.T) {
	p := GetProbe("session_isolation")
	if p == nil {
		t.Fatal("session_isolation probe is not registered")
	}
	if p.ID() != "session_isolation" {
		t.Errorf("ID() = %q, want session_isolation", p.ID())
	}
}

func TestSessionIsolationProbe_perfect_score_on_clean_run(t *testing.T) {
	observability.Reset()
	observability.IncForkSessionViewPolicy(observability.PolicyCow)
	observability.IncForkSessionViewPolicy(observability.PolicyCow)
	observability.IncForkSessionViewPolicy(observability.PolicyCow)

	probe := &SessionIsolationProbe{}
	item := EvalItem{
		ID:        "si-perfect",
		Bucket:    "production",
		Dimension: "session_isolation",
		Input: map[string]any{
			"fork_count":           3,
			"join_count":           3,
			"metadata_writes":      12,
			"isolation_violations": 0,
		},
	}
	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if score.Score < 0.99 {
		t.Errorf("Score = %v, want >= 0.99", score.Score)
	}
	if score.Details["isolation_rate"] != 1.0 {
		t.Errorf("isolation_rate = %v, want 1.0", score.Details["isolation_rate"])
	}
	if score.Details["join_consistency"] != 1.0 {
		t.Errorf("join_consistency = %v, want 1.0", score.Details["join_consistency"])
	}
	if score.Details["metric_ok"] != 1.0 {
		t.Errorf("metric_ok = %v, want 1.0", score.Details["metric_ok"])
	}
}

func TestSessionIsolationProbe_drops_score_on_violation(t *testing.T) {
	observability.Reset()
	observability.IncForkSessionViewPolicy(observability.PolicyCow)

	probe := &SessionIsolationProbe{}
	item := EvalItem{
		ID:     "si-bad",
		Bucket: "adversarial",
		Input: map[string]any{
			"fork_count":           1,
			"join_count":           1,
			"metadata_writes":      4,
			"isolation_violations": 2,
		},
	}
	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if score.Details["isolation_rate"] != 0.5 {
		t.Errorf("isolation_rate = %v, want 0.5", score.Details["isolation_rate"])
	}
	// (0.5 + 1.0 + 1.0) / 3 = 0.8333...
	if score.Score < 0.8 || score.Score > 0.9 {
		t.Errorf("Score = %v, want in [0.8, 0.9]", score.Score)
	}
}

func TestSessionIsolationProbe_drops_score_on_join_mismatch(t *testing.T) {
	observability.Reset()
	observability.IncForkSessionViewPolicy(observability.PolicyCow)
	observability.IncForkSessionViewPolicy(observability.PolicyCow)

	probe := &SessionIsolationProbe{}
	item := EvalItem{
		ID:     "si-join",
		Bucket: "edge",
		Input: map[string]any{
			"fork_count":           2,
			"join_count":           1, // mismatch
			"metadata_writes":      4,
			"isolation_violations": 0,
		},
	}
	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if score.Details["join_consistency"] != 0.0 {
		t.Errorf("join_consistency = %v, want 0.0", score.Details["join_consistency"])
	}
}
