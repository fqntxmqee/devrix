package eval

import (
	"context"
	"testing"
)

// T: D6-S3-A01-T06

func TestToolAccuracyProbe_AllCorrect(t *testing.T) {
	probe := &ToolAccuracyProbe{}
	item := EvalItem{
		ID:        "tool-acc-1",
		Bucket:    "production",
		Dimension: "tool_accuracy",
		Input: map[string]any{
			"expected_tools": []any{"read_file", "grep"},
			"actual_tools":   []any{"read_file", "grep"},
		},
	}

	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["precision"] != 1.0 {
		t.Errorf("precision = %v, want 1.0", score.Details["precision"])
	}
	if score.Details["recall"] != 1.0 {
		t.Errorf("recall = %v, want 1.0", score.Details["recall"])
	}
	if score.Score != 1.0 {
		t.Errorf("F1 = %v, want 1.0", score.Score)
	}
}

func TestToolAccuracyProbe_PartialWrong(t *testing.T) {
	probe := &ToolAccuracyProbe{}
	item := EvalItem{
		ID:        "tool-acc-2",
		Bucket:    "adversarial",
		Dimension: "tool_accuracy",
		Input: map[string]any{
			"expected_tools": []any{"read_file"},
			"actual_tools":   []any{"write_file"},
		},
	}

	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["precision"] != 0 {
		t.Errorf("precision = %v, want 0", score.Details["precision"])
	}
	if score.Details["recall"] != 0 {
		t.Errorf("recall = %v, want 0", score.Details["recall"])
	}
	if score.Score != 0 {
		t.Errorf("F1 = %v, want 0", score.Score)
	}
}

func TestToolAccuracyProbe_PartialMatch(t *testing.T) {
	probe := &ToolAccuracyProbe{}
	item := EvalItem{
		ID:        "tool-acc-3",
		Bucket:    "edge",
		Dimension: "tool_accuracy",
		Input: map[string]any{
			"expected_tools": []any{"read_file", "grep"},
			"actual_tools":   []any{"read_file", "write_file"},
		},
	}

	score, err := probe.Run(context.Background(), item, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["precision"] != 0.5 {
		t.Errorf("precision = %v, want 0.5", score.Details["precision"])
	}
	if score.Details["recall"] != 0.5 {
		t.Errorf("recall = %v, want 0.5", score.Details["recall"])
	}
	wantF1 := 0.5
	if score.Score != wantF1 {
		t.Errorf("F1 = %v, want %v", score.Score, wantF1)
	}
}

func TestToolSelectionMetrics_EmptyBoth(t *testing.T) {
	p, r, f := toolSelectionMetrics(nil, nil)
	if p != 1 || r != 1 || f != 1 {
		t.Errorf("metrics = %v %v %v, want 1 1 1", p, r, f)
	}
}
