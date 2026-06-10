package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEvalEngine_Disabled(t *testing.T) {
	config := EvalConfig{Enabled: false}
	jm := NewJudgeManager(nil, nil, JudgeConfig{})
	engine := NewEvalEngine(config, jm)

	report, err := engine.Run(context.Background(), EvalOpts{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report != nil {
		t.Error("expected nil report when disabled")
	}
}

func TestEvalEngine_EnabledButNoDataset(t *testing.T) {
	config := EvalConfig{Enabled: true, Dataset: DatasetConfig{Path: "/nonexistent"}}
	jm := NewJudgeManager(nil, nil, JudgeConfig{})
	engine := NewEvalEngine(config, jm)

	_, err := engine.Run(context.Background(), EvalOpts{})
	if err == nil {
		t.Fatal("expected error for nonexistent dataset")
	}
}

func TestEvalEngine_FullFlow(t *testing.T) {
	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "dataset.yaml")

	// Create test dataset
	datasetYAML := `
id: test-v1
version: v1
created_at: 2026-06-10T00:00:00Z
buckets:
  - name: production
    weight: 1.0
items:
  - id: t1
    bucket: production
    domain: d2
    dimension: compression_recall
    input:
      original: "The user wants JWT auth with PostgreSQL."
      compressed: "JWT auth with PostgreSQL."
    expectation:
      must_keep: ["JWT", "PostgreSQL"]
  - id: t2
    bucket: adversarial
    domain: d2
    dimension: compression_recall
    input:
      original: "Fact A, Fact B, Fact C."
      compressed: "Fact A."
    expectation:
      must_keep: ["Fact A"]
  - id: t3
    bucket: edge
    domain: d2
    dimension: compression_recall
    input:
      original: "Some important detail X and Y."
      compressed: "Nothing useful."
    expectation:
      must_keep: ["X", "Y"]
`
	if err := os.WriteFile(datasetPath, []byte(datasetYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Fix time for deterministic testing
	now = func() time.Time {
		return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	}
	defer func() { now = timeNow }()

	config := EvalConfig{
		Enabled:  true,
		Dataset:  DatasetConfig{Path: datasetPath},
		Judge:    JudgeConfig{Model: "mock", Temperature: 0},
		Sampling: SamplingConfig{Enabled: false},
	}

	client := &mockLLMClient{
		response: "Reasoning: Facts preserved\nScore: 0.85\nConfidence: 0.8\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	jm := NewJudgeManager(client, nil, JudgeConfig{Model: "mock", Temperature: 0})
	jm.RegisterRubric(ScoreRubric{Dimension: "compression_recall", Instruction: "test", Scale: "0-1"})

	engine := NewEvalEngine(config, jm)

	report, err := engine.Run(context.Background(), EvalOpts{DatasetPath: datasetPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.DatasetID != "test-v1" {
		t.Errorf("DatasetID = %s, want test-v1", report.DatasetID)
	}
	if len(report.Scores) == 0 {
		t.Fatal("Scores is empty")
	}
	if report.Dashboard.DimensionCount == 0 {
		t.Error("DimensionCount is 0")
	}
	if report.Dashboard.ItemCount != 3 {
		t.Errorf("ItemCount = %d, want 3", report.Dashboard.ItemCount)
	}
	if report.JudgeModel != "mock" {
		t.Errorf("JudgeModel = %s, want mock", report.JudgeModel)
	}
}

func TestEvalEngine_WithBaseline(t *testing.T) {
	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "dataset.yaml")

	datasetYAML := `
id: test-v1
version: v1
created_at: 2026-06-10T00:00:00Z
buckets:
  - name: production
    weight: 1.0
items:
  - id: t1
    bucket: production
    domain: d2
    dimension: compression_recall
    input: {}
    expectation: {}
`
	if err := os.WriteFile(datasetPath, []byte(datasetYAML), 0644); err != nil {
		t.Fatal(err)
	}

	baseline := &EvalReport{
		ID: "baseline",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85, Confidence: 0.8},
		},
	}

	config := EvalConfig{Enabled: true, Judge: JudgeConfig{Model: "mock", Temperature: 0}}
	client := &mockLLMClient{
		response: "Reasoning: OK\nScore: 0.85\nConfidence: 0.8\n",
		cost:     TokenCost{TotalTokens: 15},
	}
	jm := NewJudgeManager(client, nil, JudgeConfig{Model: "mock", Temperature: 0})
	jm.RegisterRubric(ScoreRubric{Dimension: "compression_recall", Instruction: "test", Scale: "0-1"})

	engine := NewEvalEngine(config, jm)
	engine.WithBaseline(baseline)

	report, err := engine.Run(context.Background(), EvalOpts{DatasetPath: datasetPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Delta == nil {
		t.Fatal("Delta is nil")
	}
	if report.Delta.BaselineID != "baseline" {
		t.Errorf("BaselineID = %s, want baseline", report.Delta.BaselineID)
	}
}

func TestEvalEngine_EmptyDataset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")

	emptyYAML := `
id: test-empty
version: v1
created_at: 2026-06-10T00:00:00Z
buckets: []
items: []
`
	if err := os.WriteFile(path, []byte(emptyYAML), 0644); err != nil {
		t.Fatal(err)
	}

	config := EvalConfig{Enabled: true, Judge: JudgeConfig{Model: "mock"}}
	jm := NewJudgeManager(nil, nil, JudgeConfig{})
	engine := NewEvalEngine(config, jm)

	_, err := engine.Run(context.Background(), EvalOpts{DatasetPath: path})
	if err == nil || !strings.Contains(err.Error(), "at least one item") {
		t.Errorf("expected 'at least one item' error, got %v", err)
	}
}

func TestProbeRegistry(t *testing.T) {
	p := GetProbe("compression_recall")
	if p == nil {
		t.Fatal("compression_recall probe not registered")
	}
	if p.ID() != "compression_recall" {
		t.Errorf("ID = %s, want compression_recall", p.ID())
	}
}

func TestEvalEngine_IntegrationWithRealDataset(t *testing.T) {
	datasetPath := "../../../../openspec/eval-datasets/v1/dataset.yaml"
	absPath, err := filepath.Abs(datasetPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(absPath); err != nil {
		t.Skip("dataset.yaml not found at", absPath)
	}

	ds, err := LoadDataset(absPath)
	if err != nil {
		t.Fatalf("LoadDataset() error = %v", err)
	}
	if len(ds.Items) != 10 {
		t.Errorf("dataset items = %d, want 10", len(ds.Items))
	}

	now = func() time.Time {
		return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	}
	defer func() { now = timeNow }()

	config := EvalConfig{
		Enabled: true,
		Judge:   JudgeConfig{Model: "mock", Temperature: 0},
	}
	client := &mockLLMClient{
		response: "Reasoning: Facts preserved\nScore: 0.85\nConfidence: 0.8\n",
		cost:     TokenCost{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
	jm := NewJudgeManager(client, nil, JudgeConfig{Model: "mock", Temperature: 0})
	jm.RegisterRubric(ScoreRubric{
		Dimension:   "compression_recall",
		Instruction: "Evaluate whether ALL key facts from the original context are preserved in the compressed version.",
		Scale:       "0-1",
	})

	engine := NewEvalEngine(config, jm)

	report, err := engine.Run(context.Background(), EvalOpts{DatasetPath: absPath})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.DatasetID != "compression-recall-v1" {
		t.Errorf("DatasetID = %s, want compression-recall-v1", report.DatasetID)
	}
	if len(report.Scores) == 0 {
		t.Fatal("Scores is empty — no probes matched items")
	}
	if report.Dashboard.ItemCount != 10 {
		t.Errorf("ItemCount = %d, want 10", report.Dashboard.ItemCount)
	}
	if report.Dashboard.DimensionCount < 1 {
		t.Error("DimensionCount < 1")
	}
	if report.Dashboard.OverallScore <= 0 {
		t.Errorf("OverallScore = %v, want > 0", report.Dashboard.OverallScore)
	}
	if len(report.Scores[0].JudgeLogs) != 10 {
		t.Errorf("JudgeLogs = %d, want 10", len(report.Scores[0].JudgeLogs))
	}
	if report.Dashboard.JudgeCost.TotalTokens <= 0 {
		t.Error("JudgeCost.TotalTokens should be > 0")
	}

	// Verify bucket scores exist in details
	score := report.Scores[0]
	if score.Details == nil {
		t.Fatal("Details is nil")
	}
	if _, ok := score.Details["must_keep_count"]; !ok {
		t.Error("Details missing must_keep_count")
	}
}
