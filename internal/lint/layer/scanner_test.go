// Tests for the layer-lint scanner.
//
// Covers: L5-0-0-01  (layer-lint detects reverse D{N}→D{N} imports)
// Domain: shared/lint
// Stage: s0_unit
package layer

import (
	"strings"
	"testing"
)

// TestDefaultMatrix_DepictsReverseImport verifies that the default matrix flags
// D2 importing D1 as a violation. This is the canonical reverse-import case
// from DM-20260611-002.
func TestDefaultMatrix_DepictsReverseImport(t *testing.T) {
	matrix := DefaultMatrix()

	// Reverse import: D2 (contextengine) → D1 (communication) must be flagged.
	if !matrix.IsForbidden("D2", "D1") {
		t.Fatal("expected D2 -> D1 to be forbidden by the default matrix")
	}

	// Forward import: D1 → D2 must remain allowed.
	if matrix.IsForbidden("D1", "D2") {
		t.Fatal("expected D1 -> D2 to remain allowed")
	}
}

// TestDefaultMatrix_DepictsHigherToLower verifies the rest of the matrix:
// higher-numbered layers may NOT depend on lower-numbered ones.
//
// L5-0-0-01: every D{N}→D{M} with N>M must be a violation.
func TestDefaultMatrix_DepictsHigherToLower(t *testing.T) {
	matrix := DefaultMatrix()
	order := []Layer{D1, D2, D3, D4, D5, D6}
	for i, hi := range order {
		for j, lo := range order {
			if i <= j {
				continue
			}
			if !matrix.IsForbidden(hi, lo) {
				t.Errorf("expected %s -> %s to be forbidden (i=%d, j=%d)", hi, lo, i, j)
			}
		}
	}
}

// TestScan_ParsesImportsOnSyntheticFile verifies the scanner parses the import
// block of a single Go source file (no filesystem access) and returns the
// layers referenced.
func TestScan_ParsesImportsOnSyntheticFile(t *testing.T) {
	src := `package foo
import (
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
)
func _() {}
`
	pkgs, err := parseImportGraphFromSources(map[string]string{
		"internal/layers/contextengine/engine.go": src,
	})
	if err != nil {
		t.Fatalf("parseImportGraphFromSources: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("want 1 package, got %d", len(pkgs))
	}
	p := pkgs[0]
	if p.Layer != D2 {
		t.Errorf("want D2 (contextengine), got %s", p.Layer)
	}
	if len(p.Imports) != 2 {
		t.Fatalf("want 2 imports, got %d", len(p.Imports))
	}
	foundComm := false
	for _, imp := range p.Imports {
		if strings.Contains(imp, "communication/gateway") {
			foundComm = true
		}
	}
	if !foundComm {
		t.Fatal("expected scanner to capture communication/gateway import")
	}
}

// TestScan_ReportsViolationForReverseImport verifies that a synthetic package
// at D2 importing D1 produces a single violation pointing at the offending file.
func TestScan_ReportsViolationForReverseImport(t *testing.T) {
	src := `package foo
import (
	"github.com/devrix/devrix/internal/layers/communication/gateway"
)
func _() {}
`
	pkgs, err := parseImportGraphFromSources(map[string]string{
		"internal/layers/contextengine/engine.go": src,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	matrix := DefaultMatrix()
	violations := ScanPackages(pkgs, matrix)
	if len(violations) != 1 {
		t.Fatalf("want 1 violation, got %d: %+v", len(violations), violations)
	}
	v := violations[0]
	if v.From != D2 || v.To != D1 {
		t.Errorf("want D2 -> D1, got %s -> %s", v.From, v.To)
	}
	if v.File != "internal/layers/contextengine" {
		t.Errorf("want file internal/layers/contextengine, got %s", v.File)
	}
}

// TestScan_AllowsForwardDependency verifies that D1 importing D2 does NOT
// produce a violation (forward direction is allowed).
func TestScan_AllowsForwardDependency(t *testing.T) {
	src := `package foo
import (
	"github.com/devrix/devrix/internal/layers/contextengine"
)
func _() {}
`
	pkgs, err := parseImportGraphFromSources(map[string]string{
		"internal/layers/communication/gateway/gateway.go": src,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	matrix := DefaultMatrix()
	violations := ScanPackages(pkgs, matrix)
	if len(violations) != 0 {
		t.Fatalf("want 0 violations, got %d: %+v", len(violations), violations)
	}
}

// TestFormat_TextAndJSON verifies the dual output formats used by the CLI.
func TestFormat_TextAndJSON(t *testing.T) {
	v := Violation{From: "D2", To: "D1", File: "x/foo.go", Import: "foo/bar"}
	text := FormatText([]Violation{v})
	if !strings.Contains(text, "D2 -> D1") {
		t.Errorf("text format missing arrow: %q", text)
	}
	if !strings.Contains(text, "x/foo.go") {
		t.Errorf("text format missing file: %q", text)
	}
	json := FormatJSON([]Violation{v})
	if !strings.Contains(json, `"from":"D2"`) || !strings.Contains(json, `"to":"D1"`) {
		t.Errorf("json format unexpected: %q", json)
	}
}
