package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDataset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dataset.yaml")

	yamlContent := `
id: test-v1
version: v1
created_at: 2026-06-10T00:00:00Z
buckets:
  - name: production
    weight: 0.6
items:
  - id: item-1
    bucket: production
    domain: d2
    dimension: compression_recall
    input:
      original: "fact a, fact b"
      compressed: "fact a"
    expectation:
      must_keep: ["fact a", "fact b"]
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	ds, err := LoadDataset(path)
	if err != nil {
		t.Fatalf("LoadDataset() error = %v", err)
	}
	if ds.ID != "test-v1" {
		t.Errorf("ID = %s, want test-v1", ds.ID)
	}
	if len(ds.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(ds.Items))
	}
}

func TestLoadDataset_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing id",
			yaml:    "version: v1\nitems:\n  - id: a\n    domain: d2\n    dimension: test\n",
			wantErr: "ID is required",
		},
		{
			name:    "empty items",
			yaml:    "id: test\nversion: v1\nitems: []\n",
			wantErr: "at least one item",
		},
		{
			name:    "missing item domain",
			yaml:    "id: test\nversion: v1\nitems:\n  - id: a\n    dimension: test\n",
			wantErr: "domain is required",
		},
		{
			name:    "missing item dimension",
			yaml:    "id: test\nversion: v1\nitems:\n  - id: a\n    domain: d2\n",
			wantErr: "dimension is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "dataset.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadDataset(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestStratifiedSample(t *testing.T) {
	items := make([]EvalItem, 100)
	for i := range items {
		bucket := "production"
		if i < 20 {
			bucket = "adversarial"
		}
		items[i] = EvalItem{ID: string(rune('a' + i)), Bucket: bucket, Domain: "d2", Dimension: "test"}
	}

	// maxItems >= total → no sampling
	all := StratifiedSample(items, 200)
	if len(all) != 100 {
		t.Errorf("no sampling: got %d, want 100", len(all))
	}

	// stratified sampling
	sampled := StratifiedSample(items, 30)
	if len(sampled) == 0 || len(sampled) > 30 {
		t.Errorf("sampled count = %d, want [1, 30]", len(sampled))
	}
}

func TestStratifiedSample_Empty(t *testing.T) {
	result := StratifiedSample(nil, 10)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestStratifiedSample_ZeroMax(t *testing.T) {
	items := []EvalItem{{ID: "a", Domain: "d2", Dimension: "test"}}
	result := StratifiedSample(items, 0)
	if len(result) != 1 {
		t.Errorf("expected 1, got %d", len(result))
	}
}

func TestSaveLoadBaseline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.yaml")

	report := &EvalReport{
		ID:        "report-1",
		DatasetID: "test-v1",
		JudgeModel: "mock",
		Scores: []DomainScore{
			{Domain: "d2", Dimension: "compression_recall", Score: 0.85, Confidence: 0.8},
		},
	}

	if err := SaveBaseline(path, report); err != nil {
		t.Fatalf("SaveBaseline() error = %v", err)
	}

	loaded, err := LoadBaseline(path)
	if err != nil {
		t.Fatalf("LoadBaseline() error = %v", err)
	}
	if loaded.ID != report.ID {
		t.Errorf("ID = %s, want %s", loaded.ID, report.ID)
	}
	if len(loaded.Scores) != len(report.Scores) {
		t.Errorf("len(Scores) = %d, want %d", len(loaded.Scores), len(report.Scores))
	}
}
