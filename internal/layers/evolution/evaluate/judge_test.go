package evaluate

import (
	"context"
	"strings"
	"testing"
)

// mockLLMClient 模拟 LLM 响应。
type mockLLMClient struct {
	response string
	cost     TokenCost
}

func (m *mockLLMClient) Chat(_ context.Context, _ string, _ string, _ string, _ float64, _ int) (string, TokenCost, error) {
	return m.response, m.cost, nil
}

func TestJudgeManager_Score(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantMin  float64
		wantMax  float64
	}{
		{
			name:     "high score response",
			response: "Reasoning: The response meets all criteria\nScore: 0.9\nConfidence: 0.8\n",
			wantMin:  0.7,
			wantMax:  1.0,
		},
		{
			name:     "low score response",
			response: "Reasoning: Missing key information\nScore: 0.2\nConfidence: 0.7\n",
			wantMin:  0.0,
			wantMax:  0.4,
		},
		{
			name:     "default score when no score line",
			response: "Reasoning: Not sure\n",
			wantMin:  0.0,
			wantMax:  1.0,
		},
	}

	rubric := ScoreRubric{
		Dimension:   "compression_recall",
		Instruction: "Evaluate if all key facts are preserved",
		Scale:       "0-1",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockLLMClient{response: tt.response, cost: TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}
			jm := NewJudgeManager(client, nil, JudgeConfig{Model: "mock", Temperature: 0})
			jm.RegisterRubric(rubric)

			item := EvalItem{
				ID:        "test-1",
				Bucket:    "production",
				Domain:    "d2",
				Dimension: "compression_recall",
				Input:     map[string]any{"original": "fact a, fact b, fact c", "compressed": "fact a, fact c"},
				Expectation: map[string]any{"must_keep": []string{"fact a", "fact b", "fact c"}},
			}

			score, err := jm.Score(context.Background(), item, rubric)
			if err != nil {
				t.Fatalf("Score() error = %v", err)
			}
			if score.Score < tt.wantMin || score.Score > tt.wantMax {
				t.Errorf("Score() = %v, want between [%v, %v]", score.Score, tt.wantMin, tt.wantMax)
			}
			if score.TokenUsage.TotalTokens == 0 {
				t.Error("Score() TokenUsage should not be zero")
			}
		})
	}
}

func TestJudgeManager_DisputeDetection(t *testing.T) {
	primary := &mockLLMClient{
		response: "Reasoning: Good\nScore: 0.9\nConfidence: 0.8\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	secondary := &mockLLMClient{
		response: "Reasoning: Poor\nScore: 0.2\nConfidence: 0.7\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	jm := NewJudgeManager(primary, secondary, JudgeConfig{Model: "mock", Temperature: 0})
	jm.RegisterRubric(ScoreRubric{Dimension: "test", Instruction: "test", Scale: "0-1"})

	item := EvalItem{ID: "dispute-1", Dimension: "test"}
	_, err := jm.Score(context.Background(), item, ScoreRubric{Dimension: "test", Instruction: "test", Scale: "0-1"})
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}

	disputes := jm.Disputes()
	if len(disputes) == 0 {
		t.Error("expected dispute but got none")
	}
}

func TestJudgeManager_NoDispute(t *testing.T) {
	primary := &mockLLMClient{
		response: "Reasoning: Good\nScore: 0.85\nConfidence: 0.8\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	secondary := &mockLLMClient{
		response: "Reasoning: Also good\nScore: 0.80\nConfidence: 0.7\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	jm := NewJudgeManager(primary, secondary, JudgeConfig{Model: "mock", Temperature: 0})
	jm.RegisterRubric(ScoreRubric{Dimension: "test", Instruction: "test", Scale: "0-1"})

	item := EvalItem{ID: "no-dispute-1", Dimension: "test"}
	_, err := jm.Score(context.Background(), item, ScoreRubric{Dimension: "test", Instruction: "test", Scale: "0-1"})
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}

	disputes := jm.Disputes()
	if len(disputes) != 0 {
		t.Errorf("expected no dispute, got %d", len(disputes))
	}
}

func TestJudgeManager_Calibrate(t *testing.T) {
	client := &mockLLMClient{
		response: "Reasoning: OK\nScore: 0.5\nConfidence: 0.6\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}

	jm := NewJudgeManager(client, nil, JudgeConfig{Model: "mock", Temperature: 0.6})

	goldSet := []GoldLabel{
		{ItemID: "g1", HumanScore: 0.5, Reason: "acceptable"},
		{ItemID: "g2", HumanScore: 0.5, Reason: "acceptable"},
		{ItemID: "g3", HumanScore: 0.5, Reason: "acceptable"},
		{ItemID: "g4", HumanScore: 0.5, Reason: "acceptable"},
		{ItemID: "g5", HumanScore: 0.5, Reason: "acceptable"},
	}

	report, err := jm.Calibrate(context.Background(), goldSet, ScoreRubric{Dimension: "test", Instruction: "test", Scale: "0-1"})
	if err != nil {
		t.Fatalf("Calibrate() error = %v", err)
	}
	if report.GoldSetSize != len(goldSet) {
		t.Errorf("GoldSetSize = %d, want %d", report.GoldSetSize, len(goldSet))
	}
	if report.JudgeModel != "mock" {
		t.Errorf("JudgeModel = %s, want mock", report.JudgeModel)
	}
}

func TestParseJudgeResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantS float64
	}{
		{"full response", "Reasoning: Good\nScore: 0.85\nConfidence: 0.9\n", 0.85},
		{"no score", "Reasoning: Not sure\n", 0.5},
		{"score out of range", "Reasoning: Terrible\nScore: 1.5\nConfidence: 0.9\n", 1.0},
		{"negative score", "Reasoning: Bad\nScore: -0.5\nConfidence: 0.9\n", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJudgeResponse(tt.input, TokenCost{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8})
			if got.Score != tt.wantS {
				t.Errorf("Score = %v, want %v", got.Score, tt.wantS)
			}
		})
	}
}

func TestBuildJudgePrompt(t *testing.T) {
	rubric := ScoreRubric{Dimension: "test", Instruction: "check quality", Scale: "0-1", Reference: "good example"}
	item := EvalItem{
		ID:        "t1",
		Input:     map[string]any{"key": "value"},
		Expectation: map[string]any{"expected": "result"},
	}

	prompt := buildJudgePrompt(rubric, item, false)
	if !strings.Contains(prompt, "check quality") {
		t.Error("prompt missing instruction")
	}
	if !strings.Contains(prompt, "value") {
		t.Error("prompt missing input")
	}
	if !strings.Contains(prompt, "result") {
		t.Error("prompt missing expectation")
	}
}

func TestCohensKappa(t *testing.T) {
	// perfect agreement
	human := []float64{0.8, 0.9, 0.7, 0.6, 0.85}
	judge := []float64{0.8, 0.9, 0.7, 0.6, 0.85}
	k := cohensKappa(human, judge, 5)
	if k < 0.9 {
		t.Errorf("perfect agreement kappa = %v, want >= 0.9", k)
	}

	// no agreement (all same = high pe)
	human2 := []float64{0.5, 0.5, 0.5, 0.5, 0.5}
	judge2 := []float64{0.5, 0.5, 0.5, 0.5, 0.5}
	k2 := cohensKappa(human2, judge2, 5)
	if k2 < 0.9 {
		t.Errorf("all same kappa = %v, want >= 0.9", k2)
	}

	// empty
	k3 := cohensKappa(nil, nil, 5)
	if k3 != 0 {
		t.Errorf("empty kappa = %v, want 0", k3)
	}
}
