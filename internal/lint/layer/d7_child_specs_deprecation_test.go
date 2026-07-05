// Package layer — d7_child_specs_deprecation_test.go
//
// DM-20260704-006 Phase 5 CI guard for the deprecated `child_specs[]`
// field on StrategicPlanProposal.
//
// The legacy `execution_mode: "decompose"` + `child_specs[]` carrier
// silently dropped from Decide (it was narrative intent, not a typed
// contract). ResolutionStrategy[] (RC-1) is the new Obs→Resolution
// contract that Decide actually reads via the ResolutionReport.
//
// This test scans the sessionorchestrator package for places that
// WRITE rawStrategicPlan.ChildSpecs / prop.ChildSpecs and emits a
// warning (not a failure) so the migration to ResolutionStrategies[]
// is tracked over time. Run on every CI build.
//
// T: D7-S16-A110-T01 (Phase 5)

package layer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestD7ChildSpecsDeprecationGuard scans sessionorchestrator for
// assignment sites of the deprecated ChildSpecs[] field on strategic
// plans. Emits t.Logf warning lines so CI surfaces the count without
// failing the build (the field is still functional in Phase 4+).
//
// Threshold: any single file with more than 3 direct write sites of
// `ChildSpecs = ...` triggers a t.Errorf so a future migration PR
// can spot the regression easily.
func TestD7ChildSpecsDeprecationGuard(t *testing.T) {
	repo := repoRootFromTestD7(t)
	target := filepath.Join(repo, "internal", "layers", "orchestration", "sessionorchestrator")

	type writeSite struct {
		file  string
		line  int
		ident string
	}
	var writes []writeSite
	threshold := 3

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(b)
		// Identify writes to the deprecated field. Matches both
		// `prop.ChildSpecs =` (StrategicPlanProposal) and
		// `rawStrategicPlan{ChildSpecs:` / `&rawStrategicPlan{...ChildSpecs:`
		// (raw JSON carrier).
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "prop.ChildSpecs") && strings.Contains(trimmed, "=") && !strings.Contains(trimmed, "==") {
				writes = append(writes, writeSite{file: path, line: i + 1, ident: "prop.ChildSpecs"})
			}
			if strings.Contains(trimmed, "ChildSpecs:") && (strings.Contains(trimmed, "rawStrategicPlan") || strings.HasPrefix(trimmed, "ChildSpecs:") || strings.Contains(trimmed, "ChildSpecs []")) {
				// rawStrategicPlan literal or struct literal initializer.
				writes = append(writes, writeSite{file: path, line: i + 1, ident: "rawStrategicPlan.ChildSpecs"})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Group by file for the per-file threshold check.
	byFile := map[string]int{}
	for _, w := range writes {
		rel, _ := filepath.Rel(repo, w.file)
		t.Logf("[DEPRECATION WARNING] %s:%d writes to deprecated %s (DM-20260704-006 RC-5 migration)", rel, w.line, w.ident)
		byFile[w.file]++
	}

	for file, count := range byFile {
		if count > threshold {
			rel, _ := filepath.Rel(repo, file)
			t.Errorf("%s has %d ChildSpecs[] write sites (>%d threshold). Migrate to ResolutionStrategies[] (DM-20260704-006).", rel, count, threshold)
		}
	}
}