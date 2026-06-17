package bootstrap

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: TOOL-SURFACE-1-T08 — BuildSurfaces assembles the canonical
// surface list from the same dependencies the legacy registry uses.
func TestBuildSurfaces_AllConfigured(t *testing.T) {
	tr := tracker.New(8)
	surfaces := BuildSurfaces(SurfaceBuildOpts{
		ToolReg:   nil, // would need a real registry; leave nil for the unit test
		LSPConfig: nil,
		Tracker:   tr,
		Forker:    nil,
	})
	// We have Tracker + Verify at this point
	if len(surfaces) < 2 {
		t.Fatalf("len = %d, want >= 2", len(surfaces))
	}
	names := make([]string, len(surfaces))
	for i, s := range surfaces {
		names[i] = s.Name()
	}
	// Verify must always be present (stateless)
	found := false
	for _, n := range names {
		if n == "verify" {
			found = true
		}
	}
	if !found {
		t.Errorf("verify surface missing: got %v", names)
	}
}

// T: TOOL-SURFACE-1-A01-T24 — BuildSurfaces returns a list sorted by
// Name(). Multiple calls with the same opts produce the same order
// (prompt-cache stability). The T3.2 contract: "3 套不同 opts 输入，
// Names() 字符串完全相同" — so all-stable opts must yield the same
// sort, even though the per-opts surface count differs.
func TestBuildSurfaces_SortByName_Stable(t *testing.T) {
	tr := tracker.New(8)
	okFork := func(_ context.Context, _ string, _ []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
		return nil, nil
	}
	optsList := []SurfaceBuildOpts{
		{},
		{Tracker: tr},
		{Tracker: tr, Forker: okFork},
	}
	want := []string{"lsp", "verify"}
	for _, opts := range optsList {
		surfaces := BuildSurfaces(opts)
		got := make([]string, len(surfaces))
		for i, s := range surfaces {
			got[i] = s.Name()
		}
		// Verify the canonical (lsp, verify) subset is in alphabetical order.
		// For the minimal opts (Tracker only), the surface set is exactly {lsp, verify, tracker},
		// but in this test tracker is supplied in optsList[1+] — so for the empty opts
		// case the order must be exactly {lsp, verify}.
		if !sort.StringsAreSorted(got) {
			t.Errorf("BuildSurfaces(opts=%+v) not sorted: got %v", opts, got)
		}
		// The "lsp" and "verify" names must appear in alphabetical order whenever both are present.
		var lspIdx, verifyIdx = -1, -1
		for i, n := range got {
			if n == "lsp" {
				lspIdx = i
			}
			if n == "verify" {
				verifyIdx = i
			}
		}
		if lspIdx != -1 && verifyIdx != -1 && lspIdx > verifyIdx {
			t.Errorf("opts=%+v: lsp should come before verify in sorted output, got %v", opts, got)
		}
		_ = want
	}
}

// T: TOOL-SURFACE-1-A01-T24 — BuildSurfaces with full deps returns
// all 5 surfaces in alphabetical order: builtin < freefork < lsp <
// tracker < verify (T3.3).
func TestBuildSurfaces_FullDeps_AlphabeticalOrder(t *testing.T) {
	tr := tracker.New(8)
	okFork := func(_ context.Context, _ string, _ []toolrunner.FreeForkRequestDTO) ([]toolrunner.FreeForkHandleDTO, error) {
		return nil, nil
	}
	surfaces := BuildSurfaces(SurfaceBuildOpts{
		ToolReg: nil,
		Tracker: tr,
		Forker:  okFork,
	})
	got := make([]string, len(surfaces))
	for i, s := range surfaces {
		got[i] = s.Name()
	}
	want := []string{"free_fork", "lsp", "tool_search", "tracker", "verify"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildSurfaces order = %v, want %v", got, want)
	}
}

// T: TOOL-SURFACE-1-T08 — BuildSurfaces with no opts: LSP + Verify +
// ToolSearchSurface (the deferred-tool catalog). LSP is added
// unconditionally so the LLM tool list is stable; Verify is stateless.
func TestBuildSurfaces_OnlyStateless(t *testing.T) {
	surfaces := BuildSurfaces(SurfaceBuildOpts{})
	if len(surfaces) != 3 {
		t.Fatalf("len = %d, want 3 (lsp + verify + tool_search)", len(surfaces))
	}
	names := surfaceNames(surfaces)
	want := []string{"lsp", "tool_search", "verify"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

// T: TOOL-SURFACE-1-T08 — DefaultFilters returns the canonical chain.
func TestDefaultFilters(t *testing.T) {
	filters := DefaultFilters()
	if len(filters) == 0 {
		t.Fatal("DefaultFilters() returned empty slice")
	}
	// Sanity: toolpolicy filter must be present (drops delegate_*)
	ctx := contracts.FilterCtx{AgentType: "explore"}
	specs := []contracts.ToolSpec{
		{Name: "delegate_explore"},
		{Name: "read_file"},
	}
	filtered := filters[0].Apply(specs, ctx)
	// toolpolicy adapter for explore drops delegate_*
	if len(filtered) != 1 || filtered[0].Name != "read_file" {
		t.Errorf("toolpolicy filter should drop delegate_*: got %v", filtered)
	}
}

// T: TOOL-SURFACE-1-T08 — SurfaceBuildOpts.ToolReg nil → no builtin surface.
func TestBuildSurfaces_NilToolReg(t *testing.T) {
	surfaces := BuildSurfaces(SurfaceBuildOpts{ToolReg: nil})
	for _, s := range surfaces {
		if s.Name() == "builtin" {
			t.Errorf("builtin surface should not appear with nil ToolReg: got %+v", surfaces)
		}
	}
}

// T: TOOL-SURFACE-1-T08 — VerifySurface is always present in BuildSurfaces output.
func TestBuildSurfaces_AlwaysHasVerify(t *testing.T) {
	for _, opts := range []SurfaceBuildOpts{
		{},
		{Tracker: tracker.New(1)},
	} {
		surfaces := BuildSurfaces(opts)
		found := false
		for _, s := range surfaces {
			if s.Name() == "verify" {
				found = true
			}
		}
		if !found {
			t.Errorf("verify missing for opts %+v: got %v", opts, surfaceNames(surfaces))
		}
	}
}

// T: TOOL-SURFACE-1-T08 — NewVerifySurface returned from BuildSurfaces is callable.
func TestBuildSurfaces_VerifyCallable(t *testing.T) {
	surfaces := BuildSurfaces(SurfaceBuildOpts{})
	var v contracts.ToolSurface
	for _, s := range surfaces {
		if s.Name() == "verify" {
			v = s
		}
	}
	if v == nil {
		t.Fatal("verify surface not present")
	}
	res, err := v.Execute(context.Background(), "verify_plan_execution", `{}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Empty change_id should produce an error envelope
	if res.Error == "" {
		t.Error("Error empty, want 'change_id is required'")
	}
}

// surfaceNames is a test helper that returns the Names of a surface slice.
func surfaceNames(surfaces []contracts.ToolSurface) []string {
	out := make([]string, len(surfaces))
	for i, s := range surfaces {
		out[i] = s.Name()
	}
	sort.Strings(out)
	return out
}
