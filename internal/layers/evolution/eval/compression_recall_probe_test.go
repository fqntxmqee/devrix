package eval

import (
	"context"
	"testing"
)

type mockJudge struct {
	score float64
}

func (m *mockJudge) Score(_ context.Context, _ EvalItem, _ ScoreRubric) (*JudgeScore, error) {
	return &JudgeScore{
		Score:      m.score,
		Confidence: 0.9,
		Reasoning:  "mock evaluation",
		TokenUsage: TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}, nil
}

func (m *mockJudge) Calibrate(_ context.Context, _ []GoldLabel, _ ScoreRubric) (*CalibrationReport, error) {
	return &CalibrationReport{Kappa: 1.0, Passed: true}, nil
}

func (m *mockJudge) RegisterRubric(_ ScoreRubric) {}

func (m *mockJudge) Disputes() []ScoreDispute { return nil }

func (m *mockJudge) ResolveDispute(_ string, _ *JudgeScore) {}

func TestCompressionRecallProbe_AllFactsPreserved(t *testing.T) {
	probe := &CompressionRecallProbe{}
	judge := &mockJudge{score: 0.95}

	item := EvalItem{
		ID:        "cr-1",
		Bucket:    "production",
		Dimension: "compression_recall",
		Input: map[string]any{
			"original":   "The user wants to add user authentication with JWT tokens. They use PostgreSQL database. The project is a Go web API.",
			"compressed": "User authentication with JWT tokens using PostgreSQL. Go web API project.",
		},
		Expectation: map[string]any{
			"must_keep": []any{"JWT", "PostgreSQL", "Go web API"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Domain != "d2" {
		t.Errorf("Domain = %s, want d2", score.Domain)
	}
	if score.Dimension != "compression_recall" {
		t.Errorf("Dimension = %s, want compression_recall", score.Dimension)
	}
	if score.Score <= 0 {
		t.Errorf("Score = %v, want > 0", score.Score)
	}
	if len(score.JudgeLogs) != 1 {
		t.Errorf("len(JudgeLogs) = %d, want 1", len(score.JudgeLogs))
	}
}

func TestCompressionRecallProbe_PartialPreservation(t *testing.T) {
	probe := &CompressionRecallProbe{}
	judge := &mockJudge{score: 0.5}

	item := EvalItem{
		ID:        "cr-2",
		Bucket:    "adversarial",
		Dimension: "compression_recall",
		Input: map[string]any{
			"original":   "fact A, fact B, fact C, fact D, fact E",
			"compressed": "fact A, fact C",
		},
		Expectation: map[string]any{
			"must_keep": []any{"fact A", "fact B", "fact C", "fact D", "fact E"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if score.Details["recall"] != 0.4 {
		t.Errorf("recall = %v, want 0.4", score.Details["recall"])
	}
	if score.Score > 0.45 {
		t.Errorf("Score = %v, want <= 0.45 (conservative estimate)", score.Score)
	}
}

func TestCompressionRecallProbe_AllLost(t *testing.T) {
	probe := &CompressionRecallProbe{}
	judge := &mockJudge{score: 0.1}

	item := EvalItem{
		ID:        "cr-3",
		Bucket:    "edge",
		Dimension: "compression_recall",
		Input: map[string]any{
			"original":   "important fact X, important fact Y",
			"compressed": "nothing relevant preserved",
		},
		Expectation: map[string]any{
			"must_keep": []any{"fact X", "fact Y"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Score > 0.3 {
		t.Errorf("Score = %v, want <= 0.3 (most facts lost)", score.Score)
	}
}

func TestCompressionRecallProbe_NoExpectation(t *testing.T) {
	probe := &CompressionRecallProbe{}
	judge := &mockJudge{score: 0.8}

	item := EvalItem{
		ID:        "cr-4",
		Bucket:    "production",
		Dimension: "compression_recall",
		Input: map[string]any{
			"original":   "some text",
			"compressed": "some text",
		},
		Expectation: map[string]any{},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Score != 0.8 {
		t.Errorf("Score = %v, want 0.8 (judge score only)", score.Score)
	}
}
