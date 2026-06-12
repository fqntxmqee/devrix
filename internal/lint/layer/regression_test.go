// Regression test: scanning the real internal/layers/ tree must NOT report
// the canonical D2→D1 reverse import (DM-20260611-002 / L5-0-0-01). Any
// regression here would re-introduce the layering violation that the
// S4 implementation eliminated.
package layer

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestScan_RealCodebase_NoD2ToD1 verifies that after the layer-isolation
// refactor, no source file in internal/layers/contextengine/ imports
// internal/layers/communication/. This is the load-bearing assertion for
// L5-0-0-01.
func TestScan_RealCodebase_NoD2ToD1(t *testing.T) {
	// Walk relative to this test file so the test is location-independent.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	root := filepath.Join(repoRoot, "internal", "layers")

	pkgs, err := ParseImportGraph(root)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", root, err)
	}

	matrix := DefaultMatrix()
	violations := ScanPackages(pkgs, matrix)

	for _, v := range violations {
		if v.From == D2 && v.To == D1 {
			t.Errorf("D2 -> D1 reverse import reintroduced: file=%s import=%s", v.File, v.Import)
		}
	}
}
