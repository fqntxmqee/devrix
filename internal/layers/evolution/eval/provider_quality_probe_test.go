package eval

import (
	"context"
	"testing"
)

// T: D6-S3-A01-T09

func TestProviderQualityProbe_SameProviderConsistent(t *testing.T) {
	probe := &ProviderQualityProbe{}
	judge := &mockJudge{score: 0.95}

	item := EvalItem{
		ID:        "pq-1",
		Bucket:    "production",
		Dimension: "provider_quality",
		Input: map[string]any{
			"prompt":     "Summarize the auth flow in 3 bullets.",
			"response_a": "JWT auth with PostgreSQL. Refresh token rotation. Password reset via email.",
			"response_b": "JWT auth with PostgreSQL. Refresh token rotation. Password reset via email.",
		},
		Expectation: map[string]any{
			"must_follow": []any{"JWT", "PostgreSQL", "refresh token"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["semantic_similarity"] != 1.0 {
		t.Errorf("semantic_similarity = %v, want 1.0", score.Details["semantic_similarity"])
	}
	if score.Score < 0.9 {
		t.Errorf("Score = %v, want >= 0.9", score.Score)
	}
}

func TestProviderQualityProbe_CrossProviderComparable(t *testing.T) {
	probe := &ProviderQualityProbe{}
	judge := &mockJudge{score: 0.85}

	item := EvalItem{
		ID:        "pq-2",
		Bucket:    "production",
		Dimension: "provider_quality",
		Input: map[string]any{
			"provider_a": "deepseek",
			"provider_b": "minimax",
			"prompt":     "List the three microservices.",
			"response_a": "User Service, Order Service, Notification Service communicate via RabbitMQ.",
			"response_b": "Three services: User, Order, Notification. They use RabbitMQ messaging.",
		},
		Expectation: map[string]any{
			"must_follow": []any{"User Service", "Order Service", "RabbitMQ"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["instruction_following_a"] < 1.0 {
		t.Errorf("instruction_following_a = %v", score.Details["instruction_following_a"])
	}
	if score.Details["semantic_similarity"] <= 0 {
		t.Errorf("semantic_similarity = %v, want > 0", score.Details["semantic_similarity"])
	}
}

func TestProviderQualityProbe_MissingInstructions(t *testing.T) {
	probe := &ProviderQualityProbe{}
	judge := &mockJudge{score: 0.8}

	item := EvalItem{
		ID:        "pq-3",
		Bucket:    "adversarial",
		Dimension: "provider_quality",
		Input: map[string]any{
			"response_a": "Uses caching for sessions.",
			"response_b": "Redis stores session data.",
		},
		Expectation: map[string]any{
			"must_follow": []any{"JWT", "PostgreSQL", "refresh token"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Score > 0.5 {
		t.Errorf("Score = %v, want <= 0.5", score.Score)
	}
}
