// D4→D2 import ban tests (DM-020 boundary closure).
//
// D2 must only be consumed by D7 (and bootstrap composition root). D4 must not
// import contextengine packages directly.
package layer

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var d4ForbiddenImportPrefix = "/internal/layers/contextengine"

// TestD4_D2Ban_NoContextEngineImports verifies D4 does not import D2 directly.
func TestD4_D2Ban_NoContextEngineImports(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	maRoot := filepath.Join(repoRoot, "internal", "layers", "multiagent")

	pkgs, err := ParseImportGraph(maRoot)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", maRoot, err)
	}

	for _, p := range pkgs {
		for _, imp := range p.Imports {
			if strings.Contains(imp, d4ForbiddenImportPrefix) {
				t.Errorf("D4→D2 violation: %s imports %s", p.File, imp)
			}
		}
	}
}
