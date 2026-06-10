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
