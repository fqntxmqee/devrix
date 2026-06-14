// D2 Thin boundary tests (DM-20260614-009 v1.1).
//
// T: D2-S16-A01-T03 — query package must not import orchestration or multiagent.
package layer

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var d2ThinForbiddenImportPrefixes = []string{
	"/internal/layers/orchestration/",
	"/internal/layers/multiagent/",
}

func isD2ThinExcludedPath(dir string) bool {
	return false
}

// TestD2Thin_QueryPackage_NoOrchestrationOrMultiAgent verifies the D2-S16
// execution primitive stays free of D7/D4 orchestration imports.
func TestD2Thin_QueryPackage_NoOrchestrationOrMultiAgent(t *testing.T) {
	assertNoForbiddenImports(t, filepath.Join(repoRootFromTest(t), "internal", "layers", "contextengine", "query"))
}

// TestD2Thin_NestedPackage_NoOrchestrationOrMultiAgent verifies D2-S19 nested
// execution stays free of D7/D4 orchestration imports.
func TestD2Thin_NestedPackage_NoOrchestrationOrMultiAgent(t *testing.T) {
	assertNoForbiddenImports(t, filepath.Join(repoRootFromTest(t), "internal", "layers", "contextengine", "nested"))
}

// TestD2Thin_ContextEngine_NoOrchestrationOrMultiAgent verifies D2 root packages
// do not import D7/D4 orchestration surfaces.
func TestD2Thin_ContextEngine_NoOrchestrationOrMultiAgent(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	ctxRoot := filepath.Join(repoRoot, "internal", "layers", "contextengine")

	pkgs, err := ParseImportGraph(ctxRoot)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", ctxRoot, err)
	}

	for _, p := range pkgs {
		if isD2ThinExcludedPath(p.File) {
			continue
		}
		for _, imp := range p.Imports {
			for _, prefix := range d2ThinForbiddenImportPrefixes {
				if strings.Contains(imp, prefix) {
					t.Errorf("D2 Thin violation: %s imports %s (forbidden prefix %s)", p.File, imp, prefix)
				}
			}
		}
	}
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func assertNoForbiddenImports(t *testing.T, root string) {
	t.Helper()
	pkgs, err := ParseImportGraph(root)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", root, err)
	}
	for _, p := range pkgs {
		for _, imp := range p.Imports {
			for _, prefix := range d2ThinForbiddenImportPrefixes {
				if strings.Contains(imp, prefix) {
					t.Errorf("D2 Thin violation: %s imports %s (forbidden prefix %s)", p.File, imp, prefix)
				}
			}
		}
	}
}
