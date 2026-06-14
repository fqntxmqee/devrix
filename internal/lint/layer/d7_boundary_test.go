// D7↔D2 ingress boundary regression (DM-20260614-009 v1.1).
//
// T: d7-boundary §7 — D1 must not import D2 Process from capture layer.
package layer

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestD7Boundary_CapturePackage_NoDirectContextEngineImport verifies D1 capture
// does not re-introduce D1→D2 direct ingress (DM-20260614-007).
func TestD7Boundary_CapturePackage_NoDirectContextEngineImport(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	captureRoot := filepath.Join(repoRoot, "internal", "layers", "communication", "capture")

	pkgs, err := ParseImportGraph(captureRoot)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", captureRoot, err)
	}

	for _, p := range pkgs {
		for _, imp := range p.Imports {
			if strings.Contains(imp, "/internal/layers/contextengine") {
				t.Errorf("D7 ingress violation: capture %s imports contextengine directly: %s", p.File, imp)
			}
		}
	}
}
