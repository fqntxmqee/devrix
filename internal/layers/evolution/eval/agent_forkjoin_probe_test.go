package eval

import (
	"context"
	"testing"
)

// Covers: L5-6-3-10

func TestAgentForkJoinProbe_IsolationCorrect(t *testing.T) {
	probe := &AgentForkJoinProbe{}
	judge := &mockJudge{score: 0.95}

	item := EvalItem{
		ID:        "fj-1",
		Bucket:    "production",
		Dimension: "agent_forkjoin",
		Input: map[string]any{
			"agent_a_messages": []any{"Analyzing order schema in MongoDB."},
			"agent_b_messages": []any{"Drafting SendGrid email template."},
			"join_result":      "Order schema uses MongoDB. Email template ready via SendGrid.",
		},
		Expectation: map[string]any{
			"must_include_in_join": []any{"MongoDB", "SendGrid"},
			"agent_a_forbidden":    []any{"SendGrid"},
			"agent_b_forbidden":    []any{"MongoDB"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["isolation_a"] != 1.0 {
		t.Errorf("isolation_a = %v, want 1.0", score.Details["isolation_a"])
	}
	if score.Details["isolation_b"] != 1.0 {
		t.Errorf("isolation_b = %v, want 1.0", score.Details["isolation_b"])
	}
	if score.Score < 0.9 {
		t.Errorf("Score = %v, want >= 0.9", score.Score)
	}
}

func TestAgentForkJoinProbe_CrossContamination(t *testing.T) {
	probe := &AgentForkJoinProbe{}
	judge := &mockJudge{score: 0.9}

	item := EvalItem{
		ID:        "fj-2",
		Bucket:    "adversarial",
		Dimension: "agent_forkjoin",
		Input: map[string]any{
			"agent_a_messages": []any{"Order uses MongoDB and SendGrid for emails."},
			"agent_b_messages": []any{"Email template only."},
			"join_result":      "MongoDB orders and SendGrid emails.",
		},
		Expectation: map[string]any{
			"must_include_in_join": []any{"MongoDB", "SendGrid"},
			"agent_a_forbidden":    []any{"SendGrid"},
			"agent_b_forbidden":    []any{"MongoDB"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["isolation_a"] != 0.0 {
		t.Errorf("isolation_a = %v, want 0.0", score.Details["isolation_a"])
	}
	if score.Score != 0.0 {
		t.Errorf("Score = %v, want 0.0 due to isolation failure", score.Score)
	}
}

func TestAgentForkJoinProbe_IncompleteJoin(t *testing.T) {
	probe := &AgentForkJoinProbe{}
	judge := &mockJudge{score: 0.85}

	item := EvalItem{
		ID:        "fj-3",
		Bucket:    "edge",
		Dimension: "agent_forkjoin",
		Input: map[string]any{
			"agent_a_messages": []any{"MongoDB order schema ready."},
			"agent_b_messages": []any{"SendGrid template drafted."},
			"join_result":      "Order schema uses MongoDB only.",
		},
		Expectation: map[string]any{
			"must_include_in_join": []any{"MongoDB", "SendGrid"},
			"agent_a_forbidden":    []any{"SendGrid"},
			"agent_b_forbidden":    []any{"MongoDB"},
		},
	}

	score, err := probe.Run(context.Background(), item, judge)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if score.Details["join_completeness"] != 0.5 {
		t.Errorf("join_completeness = %v, want 0.5", score.Details["join_completeness"])
	}
	if score.Score > 0.55 {
		t.Errorf("Score = %v, want <= 0.55", score.Score)
	}
}
