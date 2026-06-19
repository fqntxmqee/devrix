// Package contextengine — queryloop decommission guard tests.
//
// P2-T5 status: TestD2_RootProductionFiles_ThinFacade and TestD2_NoQueryLoopProductionReferences
// consolidated into internal/lint/layer/d2_layout_test.go (D2-STRUCT-T01/T02/T04/T05/T06).
// Remaining: TestD2_QueryLoopRemoved (query/ dir absence) and
// TestD2_EngineUsesPreparedTurnRunner (engine smoke) are kept here as
// domain-level smoke checks.
package contextengine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestD2_QueryLoopRemoved(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	queryDir := filepath.Join(filepath.Dir(file), "query")
	if _, err := os.Stat(queryDir); err == nil {
		t.Fatalf("query package directory still present: %s", queryDir)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat query dir: %v", err)
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

type staticSummarizer struct{}

func (staticSummarizer) Summarize(context.Context, string, string, int) (string, error) {
	return "summary", nil
}
