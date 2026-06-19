package contextengine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestD2_QueryLoopRemoved(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "query")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read query dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "loop") {
			t.Fatalf("query loop file still present: %s", e.Name())
		}
	}
}

func TestD2_EngineUsesPreparedTurnRunner(t *testing.T) {
	e := NewContextEngine(EngineDeps{
		Summarizer: &staticSummarizer{},
	})
	if e.PreparedTurnRunner() != nil {
		t.Fatal("expected nil prepared turn runner before wiring")
	}
	var _ contracts.PreparedTurnRunner
	_ = e.SetPreparedTurnRunner
}

func TestD2_NoQueryLoopProductionReferences(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..")
	var offenders []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				switch d.Name() {
				case "vendor", ".git":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		for _, needle := range []string{"QueryLLMCaller", "query.Loop.Run", "Loop.Run("} {
			if strings.Contains(content, needle) {
				offenders = append(offenders, path+": contains "+needle)
			}
		}
		return nil
	})
	if len(offenders) > 0 {
		t.Fatalf("production references to removed QueryLoop API:\n%s", strings.Join(offenders, "\n"))
	}
}

type staticSummarizer struct{}

func (staticSummarizer) Summarize(context.Context, string, string, int) (string, error) {
	return "summary", nil
}
