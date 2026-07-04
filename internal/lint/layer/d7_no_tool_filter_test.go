package layer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// T: D7-S2-A91-T01 — sessionorchestrator must not import enforce/tools/filter.
func TestD7SessionOrchestrator_NoToolFilterImport(t *testing.T) {
	forbidden := []string{
		"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/filter",
	}
	root := filepath.Join(repoRootFromTest(t), "internal", "layers", "orchestration", "sessionorchestrator")
	pkgs, err := ParseProductionImportGraph(root)
	if err != nil {
		t.Fatalf("ParseImportGraph: %v", err)
	}
	for _, p := range pkgs {
		for _, imp := range p.Imports {
			for _, f := range forbidden {
				if imp == f {
					t.Errorf("D7 boundary violation: %s imports %s", p.File, imp)
				}
			}
		}
	}
}

// T: D7-S2-A91-T02 — no toolsForProfile / filterPipelineTools in target packages.
func TestD7NoDeadToolFilterHelpers(t *testing.T) {
	repo := repoRootFromTest(t)
	checks := []struct {
		dir     string
		forbidden []string
	}{
		{
			dir: filepath.Join(repo, "internal", "layers", "contextengine", "materialize"),
			forbidden: []string{"toolsForProfile", "filterPipelineTools"},
		},
		{
			dir: filepath.Join(repo, "internal", "layers", "orchestration", "sessionorchestrator"),
			forbidden: []string{"filterPipelineTools", "pipelineBlockedTools"},
		},
	}
	for _, c := range checks {
		err := filepath.Walk(c.dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			content := string(b)
			for _, token := range c.forbidden {
				if strings.Contains(content, token) {
					t.Errorf("%s contains forbidden token %q", path, token)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func repoRootFromTestD7(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}
