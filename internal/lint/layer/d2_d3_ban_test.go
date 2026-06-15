// D2→D3 import ban tests (DM-020 D-d / D2-THIN-T01).
//
// Per the D7 Turn 编排上移 design, D2 must not import or call D3 directly.
// LLM calling right belongs to D7 (Turn Leader); D2 provides ContextPreparer,
// ToolRoundExecutor, and SessionPersister as pure execution primitives.
//
// T: D2-THIN-T01 — import lint gate, CI hard block after v2.0-f.
package layer

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// d2ToD3ForbiddenPrefixes are import path segments that D2 must not use.
var d2ToD3ForbiddenPrefixes = []string{
	"/internal/layers/llmgateway",
	"/internal/bridges/llm",
}

// TestD2_D3Ban_NoImports verifies zero D2→D3 imports in production packages.
func TestD2_D3Ban_NoImports(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	ctxRoot := filepath.Join(repoRoot, "internal", "layers", "contextengine")

	pkgs, err := ParseProductionImportGraph(ctxRoot)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", ctxRoot, err)
	}

	var violations []string
	for _, p := range pkgs {
		for _, imp := range p.Imports {
			for _, prefix := range d2ToD3ForbiddenPrefixes {
				if strings.Contains(imp, prefix) {
					violations = append(violations, p.File+" → "+imp)
				}
			}
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		for _, v := range violations {
			t.Errorf("D2→D3 violation: %s", v)
		}
	}
}
