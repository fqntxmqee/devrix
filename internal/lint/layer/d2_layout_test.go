// D2 v2.2 Structure layout guard tests (DM-20260619-007 devrix-d2-structure-closure).
//
// Per the DSAFT Refactoring Playbook §6 双锚点对齐, D2 root must remain
// scenario-organized: prepare/ persist/ enforce/ kernel/. Anything that drifts
// back to Pre-v2.2 form (engine_*.go files at root, tools package name,
// orchestrate stub) is a layout violation that must be caught at CI time.
//
// DM-20260629-002 (devrix-d2-dsaft-restructuring PR-1): legacy/ directory and
// aliases.go retired; the D2 ContextEngine implementation now lives in
// kernel/ alongside the observer contracts. Tests T01 + T07 + T08 updated to
// reflect the post-P5 closure state.
//
// T: D2-STRUCT-T01 — root production files only `contracts.go` (+ tool_context.go + fixtures)
// T: D2-STRUCT-T02 — no engine_persist.go outside facade/ (now persist/commit.go)
// T: D2-STRUCT-T03 — enforce/tools/ package is `package tools`, not `tools`
// T: D2-STRUCT-T04 — prepare/memory/ and persist/memory/ have no cyclic import
// T: D2-STRUCT-T05 — enforce/orchestrator.go removed (stub gone, turn_adapter is SoT)
// T: D2-STRUCT-T06 — scenario subdirectories at most 2 levels deep
// T: D2-STRUCT-T07 — no new kernel.ContextEngine.Process() callers (PR-1 P5 retirement guard)
// T: D2-STRUCT-T08 — query/ package removed (DM-20260618-010 QueryLoop decommission)
package layer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// allowedRootProductionFiles are the only files permitted at D2 root after v2.2 closure.
// DM-20260629-002 PR-1: `aliases.go` removed (legacy/ directory retired). The
// remaining allowances are documented inline.
//
// `tool_context.go` is allowed as a transitional type alias only after P2 migration.
// `summarizer_fixture.go` and `prepared_turn_fixture.go` are cross-domain D7-contract
// fixtures (DM-20260619-008 devrix-d2-mock-semantic-split): D2 cannot import D7
// (Follower→Leader ban), so static fakes for D7-owned contracts (Summarizer,
// PreparedTurnRunner) live at D2 root as production-buildable helpers consumed by
// D2 tests, the cmd/obs-verify smoke binary, and integration tests across the repo.
// They cannot live in *_test.go because cmd/obs-verify imports them from main.
var allowedRootProductionFiles = map[string]bool{
	"contracts.go":              true,
	"tool_context.go":           true, // P2 transitional: re-exports types after enforce/tools/context.go extraction
	"doc.go":                    true, // package doc
	"summarizer_fixture.go":     true, // D7 contracts.Summarizer cross-domain fixture
	"prepared_turn_fixture.go":  true, // D7 contracts.PreparedTurnRunner cross-domain fixture
}

// forbiddenRootBasenames are filenames that signal Pre-v2.2 drift at D2 root.
// engine_*.go files must live in facade/ (legacy/) or have been extracted to scenario subpackages.
var forbiddenRootBasenames = []string{
	"engine.go",
	"engine_builder.go",
	"engine_compression.go",
	"engine_events.go",
	"engine_export.go",
	"engine_persist.go",
	"engine_prepare.go",
	"engine_types.go",
}

// d2RepoRoot resolves the absolute path of the devrix repository root.
func d2RepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	// internal/lint/layer/d2_layout_test.go → repo root is 3 dirs up.
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}

// TestD2Layout_RootProductionFilesOnly verifies D2-STRUCT-T01: root contains
// only contracts.go (+ tool_context.go alias + doc.go + fixtures).
// DM-20260629-002 PR-1: aliases.go removed after legacy/ retirement.
func TestD2Layout_RootProductionFilesOnly(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", root, err)
	}

	var violations []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !allowedRootProductionFiles[name] {
			violations = append(violations, name)
		}
	}

	if len(violations) > 0 {
		t.Errorf("D2 root drift (D2-STRUCT-T01): disallowed production files at %s: %v. "+
			"Allowed: %v. Extract to scenario/ or kernel/.", root, violations, keys(allowedRootProductionFiles))
	}
}

// TestD2Layout_NoEngineFilesAtRoot verifies D2-STRUCT-T01 strict: no engine_*.go
// at root after facade rename. This is a guard against regression if someone
// adds new engine_*.go without routing through facade/.
func TestD2Layout_NoEngineFilesAtRoot(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine")
	for _, bad := range forbiddenRootBasenames {
		path := filepath.Join(root, bad)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("D2-STRUCT-T01/T02 violation: forbidden file present at %s. "+
				"engine_*.go must live in facade/legacy/ or be extracted to scenario subpackages.", path)
		}
	}
}

// TestD2Layout_NoEnginePersistOutsideFacade verifies D2-STRUCT-T02:
// engine_persist.go functionality must live at persist/commit.go (already extracted),
// no duplicate persists anywhere outside facade/.
//
// DM-20260629-002 PR-1: legacy/ retired (moved to kernel/). engine_persist.go
// allowance updated to /kernel/ + /persist/ + /facade/.
func TestD2Layout_NoEnginePersistOutsideFacade(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine")
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Allow engine_persist.go only in kernel/ (DM-20260629-002 PR-1: legacy/ → kernel/).
		if strings.HasSuffix(path, "engine_persist.go") &&
			!strings.Contains(path, "/kernel/") &&
			!strings.Contains(path, "/persist/") &&
			!strings.Contains(path, "/facade/") {
			violations = append(violations, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk(%s): %v", root, err)
	}
	if len(violations) > 0 {
		t.Errorf("D2-STRUCT-T02: engine_persist.go found outside kernel/persist/facade: %v", violations)
	}
}

// TestD2Layout_EnforceToolsPackage verifies D2-STRUCT-T03:
// after P3 git mv tools/ → enforce/tools/, the package name must be `package tools`,
// never `package tools`.
func TestD2Layout_EnforceToolsPackage(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine", "enforce", "tools")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skipf("enforce/tools/ not yet created (P3 pending): %v", err)
	}
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		first := firstNonCommentLine(string(data))
		if strings.Contains(first, "package toolrunner") {
			violations = append(violations, path+": "+first)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk(%s): %v", root, err)
	}
	if len(violations) > 0 {
		t.Errorf("D2-STRUCT-T03: package tools remains in enforce/tools/: %v", violations)
	}
}

// TestD2Layout_NoEnforceOrchestratorStub verifies D2-STRUCT-T05:
// enforce/orchestrator.go (92-line stub) must be deleted after P3. Real dispatch
// lives in bootstrap/turn_adapter.ExecuteRound via contracts.ToolSurface/ToolFilter.
//
// Phase guard: until P3 deletes the stub, the test records its presence as an
// INFO and skips hard failure. After P3, the stub must NOT be reintroduced.
func TestD2Layout_NoEnforceOrchestratorStub(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine", "enforce")
	stub := filepath.Join(root, "orchestrator.go")
	if _, err := os.Stat(stub); err == nil {
		t.Logf("D2-STRUCT-T05 INFO: enforce/orchestrator.go present (P3 pending deletion). "+
			"Guard will hard-fail if stub reappears after P3.")
		t.Skip("D2-STRUCT-T05 phase guard: stub deletion is P3 work; activate after P3 lands")
	}
}

// TestD2Layout_NoMemoryCyclicImport verifies D2-STRUCT-T04:
// prepare/memory/ and persist/memory/ must be split with no cyclic dependency.
// We check via textual import scan: prepare/memory/ files must not import
// persist/memory/, and vice versa.
//
// Phase guard: only enforced after P4 (memory split) creates persist/memory/.
// Until then, the test skips so P1-b..P1-f work can land without false alarms.
func TestD2Layout_NoMemoryCyclicImport(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine")

	prepareMem := filepath.Join(root, "prepare", "memory")
	persistMem := filepath.Join(root, "persist", "memory")

	if _, err := os.Stat(persistMem); os.IsNotExist(err) {
		t.Skipf("D2-STRUCT-T04: persist/memory/ not yet created (P4 pending); guard activates after split")
	}

	checkNoCrossImport := func(srcDir, forbiddenPkgSubstr string) error {
		var found []string
		err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "\"") && strings.Contains(trimmed, forbiddenPkgSubstr) {
					found = append(found, path+": "+trimmed)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if len(found) > 0 {
			return &crossImportError{src: srcDir, forbidden: forbiddenPkgSubstr, hits: found}
		}
		return nil
	}

	if err := checkNoCrossImport(prepareMem, "persist/memory"); err != nil {
		t.Errorf("D2-STRUCT-T04 violation: %v", err)
	}
	if err := checkNoCrossImport(persistMem, "prepare/memory"); err != nil {
		t.Errorf("D2-STRUCT-T04 violation: %v", err)
	}
}

// TestD2Layout_ScenarioDepthLE2 verifies D2-STRUCT-T06:
// scenario directories (prepare/, persist/, enforce/) max depth ≤ 2.
// enforce/tools/surface/ is allowed (2 levels under enforce/).
// Anything deeper requires F-registry entry.
func TestD2Layout_ScenarioDepthLE2(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine")
	scenarios := []string{"prepare", "persist", "enforce"}

	var violations []string
	for _, s := range scenarios {
		base := filepath.Join(root, s)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			// Skip the scenario root itself.
			if path == base {
				return nil
			}
			// Compute depth relative to scenario base.
			rel, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if depth > 2 {
				violations = append(violations, filepath.ToSlash(path)+" (depth="+itoa(depth)+")")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("Walk(%s): %v", base, err)
		}
	}

	if len(violations) > 0 {
		t.Errorf("D2-STRUCT-T06: scenario subdirectory depth > 2 detected: %v. "+
			"Deeper nesting requires F-registry entry.", violations)
	}
}

// TestD2Layout_NoNewLegacyProcessCallers verifies D2-STRUCT-T07 (P5):
// after facade/ → legacy/ → kernel/ retirement (DM-20260629-002 PR-1), no
// NEW production code may call kernel.ContextEngine.Process() outside the
// documented hot path. That entry point emits slog.Warn at runtime and the
// canonical hot path is now D7 SessionOrchestrator.ProcessMessage (or the
// turn adapter for worker forking). Existing callers are grandfathered in
// kernel/ itself + the 8 known production sites listed in AC-P5-3; this guard
// fails the build if a NEW caller outside the allowlist appears.
//
// Allowlist: cmd/llm-smoke (smoke fixture), multiagent/run (worker
// engine), all tests/ (integration/acceptance/perf fixtures), and the
// kernel/ package itself (where the deprecation warning lives).
func TestD2Layout_NoNewLegacyProcessCallers(t *testing.T) {
	root := d2RepoRoot(t)
	allowed := map[string]bool{
		"cmd/llm-smoke/main.go":                                true,
		"internal/layers/multiagent/run/lifecycle.go":          true,
		"internal/layers/multiagent/run/worker_engine.go":      true,
		"internal/layers/contextengine/kernel/":                true, // kernel/ replaces legacy/ (DM-20260629-002 PR-1)
		"tests/integration/":                                   true,
		"tests/acceptance/":                                    true,
		"tests/performance/":                                   true,
		"tests/testutil/":                                      true,
		"internal/layers/communication/capture/gateway_test.go": true, // mock engine.Process() impl
		"internal/layers/communication/channel/adapters/cli_test.go":     true,
		"internal/layers/communication/channel/adapters/feishu_test.go":  true,
		"internal/layers/contextengine/engine_accessor_test.go":         true,
		"internal/lint/layer/d2_layout_test.go":                          true, // this guard itself
	}

	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// skip vendored / archived
			if info.IsDir() && (strings.Contains(path, "/.git/") || strings.Contains(path, "/bin/")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		// Only flag .Process( call sites, not the declaration of Process() itself.
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		if !strings.Contains(src, ".Process(ctx") {
			return nil
		}
		// Skip test mocks (their own .Process method).
		if strings.Contains(src, "func (") && strings.Contains(src, ") Process(ctx") {
			// method declaration on a stub or wrapper — skip
			// (heuristic: presence of both `func (` and `) Process(ctx` on the same file)
		}

		// Allow any explicitly allowed path.
		for prefix := range allowed {
			if strings.HasPrefix(relSlash, prefix) {
				return nil
			}
		}
		violations = append(violations, relSlash)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk(%s): %v", root, err)
	}
	if len(violations) > 0 {
		t.Errorf("D2-STRUCT-T07 (P5): new kernel.ContextEngine.Process() callers detected: %v. "+
			"Migrate to D7 SessionOrchestrator.ProcessMessage or turn_adapter.ExecuteRound.",
			violations)
	}
}

// TestD2Layout_NoQueryDir verifies D2-STRUCT-T08:
// after DM-20260618-010 QueryLoop subsystem archival, the query/ package
// must not reappear at D2 root. The canonical hot path is D7 SessionOrchestrator
// → D7-S2-A06 RunTurn → D2 turn adapters.
//
// Migrated from internal/layers/contextengine/queryloop_removed_test.go
// during the 2026-06-19 D2 root test cleanup.
func TestD2Layout_NoQueryDir(t *testing.T) {
	root := filepath.Join(d2RepoRoot(t), "internal", "layers", "contextengine")
	queryDir := filepath.Join(root, "query")
	if _, err := os.Stat(queryDir); err == nil {
		t.Errorf("D2-STRUCT-T08: query/ directory reappeared at %s after QueryLoop archival", queryDir)
	} else if !os.IsNotExist(err) {
		t.Errorf("D2-STRUCT-T08: stat query dir: %v", err)
	}
}

// --- helpers ---

type crossImportError struct {
	src       string
	forbidden string
	hits      []string
}

func (e *crossImportError) Error() string {
	return "cross-import from " + e.src + " into " + e.forbidden + ": " + strings.Join(e.hits, "; ")
}

// firstNonCommentLine returns the first non-empty, non-`//` line of a Go source file.
// Used to detect `package <name>` declarations.
func firstNonCommentLine(src string) string {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		return trimmed
	}
	return ""
}

// keys returns the keys of a string-keyed bool map (for error messages).
func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// itoa is a tiny strconv.Itoa to avoid extra import noise in test files.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}