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

// d2ToD3KnownViolations lists directories (relative to contextengine/) that
// currently import D3 and are grandfathered until the respective migration
// slice removes them.
//
//	.                  → engine.go, llm_logger.go (D-d: ILLMGateway from EngineDeps)
//	mock               → mock/llm.go (D-f: mock adapter for legacy path)
//	query              → query/adapters.go (D-d: LLMCaller wraps ILLMGateway)
//	prepare/compression → llm_summarizer.go, query_loop_factory.go (D-e: Autocompact)
//
// Test-only directories (token) are excluded via _test.go skip.
var d2ToD3KnownViolations = map[string]string{
	".":                    "D-d: ILLMGateway + ITierResolver in EngineDeps + llm_logger.go",
	"mock":                 "D-f: mock adapter for legacy ILLMGateway path",
	"query":                "D-d: LLMCaller adapts ILLMGateway to LLMCaller interface",
	"prepare/compression":  "D-e: Autocompact LLMSummarizer + CompressFactory move to D7",
}

// TestD2_D3Ban_NoNewImports verifies that no new D2 source directories import D3.
// Existing violations must be listed in d2ToD3KnownViolations.
func TestD2_D3Ban_NoNewImports(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	ctxRoot := filepath.Join(repoRoot, "internal", "layers", "contextengine")

	pkgs, err := ParseImportGraph(ctxRoot)
	if err != nil {
		t.Fatalf("ParseImportGraph(%s): %v", ctxRoot, err)
	}

	var unexpected []string
	for _, p := range pkgs {
		// p.File is the directory relative to ctxRoot (e.g. ".", "query", "prepare/compression").
		dir := p.File

		// Skip test-only directories by checking against import paths.
		// Directories whose ONLY Go files are _test.go will still show up
		// in ParseImportGraph, but we skip the "token" directory explicitly.
		if dir == "token" {
			continue
		}

		for _, imp := range p.Imports {
			for _, prefix := range d2ToD3ForbiddenPrefixes {
				if strings.Contains(imp, prefix) {
					if _, known := d2ToD3KnownViolations[dir]; !known {
						unexpected = append(unexpected, dir+" → "+imp)
					}
				}
			}
		}
	}

	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		for _, v := range unexpected {
			t.Errorf("D2→D3 violation (new): %s", v)
		}
		t.Errorf("%d new D2→D3 import(s) detected. "+
			"Add to d2ToD3KnownViolations ONLY if justified by a migration plan. "+
			"Otherwise, remove the import and use the D7→D3 path (LLMInvoker) instead.", len(unexpected))
	}
}

// TestD2_D3Ban_KnownViolationsMax verifies the known-violations list
// does not exceed the expected maximum. The count must decrease as
// migration slices complete:
//
//	After D-d: ≤ 3 (., mock, query)
//	After D-e: ≤ 2 (., mock)
//	After D-f: ≤ 0
func TestD2_D3Ban_KnownViolationsMax(t *testing.T) {
	const maxExpected = 4
	if len(d2ToD3KnownViolations) > maxExpected {
		t.Errorf("d2ToD3KnownViolations is %d, expected ≤ %d. "+
			"The list must shrink with each migration slice.",
			len(d2ToD3KnownViolations), maxExpected)
	}
}
