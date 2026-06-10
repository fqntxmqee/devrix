package evalcli_test

import (
	"os"
	"path/filepath"
	"testing"

	evalcli "github.com/devrix/devrix/internal/cli/eval"
)

func TestRunEval_should_write_report_json(t *testing.T) {
	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "dataset.yaml")
	datasetYAML := `
id: cli-test-v1
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
      original: "JWT auth with PostgreSQL"
      compressed: "JWT auth with PostgreSQL"
    expectation:
      must_keep: ["JWT", "PostgreSQL"]
`
	if err := os.WriteFile(datasetPath, []byte(datasetYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "report.json")

	if err := evalcli.RunEval([]string{
		"--dataset", datasetPath,
		"--output", outPath,
	}); err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty report")
	}
}

func TestRunEval_should_fail_gate_on_regression(t *testing.T) {
	dir := t.TempDir()
	datasetPath := filepath.Join(dir, "dataset.yaml")
	baselinePath := filepath.Join(dir, "baseline.yaml")
	datasetYAML := `
id: cli-gate-v1
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
      original: "fact A fact B fact C"
      compressed: "fact A"
    expectation:
      must_keep: ["fact A", "fact B", "fact C"]
`
	baselineYAML := `
id: baseline-high
dataset_id: cli-gate-v1
run_at: 2026-06-10T00:00:00Z
judge_model: mock
scores:
  - domain: d2
    dimension: compression_recall
    score: 0.99
    confidence: 0.9
dashboard:
  overall_score: 0.99
  dimension_count: 1
  item_count: 1
`
	if err := os.WriteFile(datasetPath, []byte(datasetYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(baselinePath, []byte(baselineYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	err := evalcli.RunEval([]string{
		"--dataset", datasetPath,
		"--baseline", baselinePath,
		"--gate",
		"--output", filepath.Join(dir, "out.json"),
	})
	if err == nil {
		t.Fatal("expected gate error")
	}
}
