package surface_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// T: TOOL-SURFACE-1-A01-T22 — OrthogonalFlagFor returns the 4 bool flags
// for every tool name declared in the orthogonal_flags.go truth table.
// The "conservative default" rule says: at least one bool must be true
// for any known tool name; unknown names get all-false.
func TestOrthogonalFlagFor_KnownTools(t *testing.T) {
	type want struct {
		readOnly        bool
		destructive     bool
		openWorld       bool
		concurrencySafe bool
	}
	cases := []struct {
		name string
		want want
	}{
		{"read_file", want{true, false, false, true}},
		{"write_file", want{false, true, false, false}},
		{"edit_file", want{false, true, false, false}},
		{"bash", want{false, true, false, true}},
		{"grep", want{true, false, false, true}},
		{"glob", want{true, false, false, true}},
		{"lsp", want{true, false, false, false}},
		{"free_fork", want{false, false, true, false}},
		{"query_diagnostics", want{true, false, false, true}},
		{"verify_plan_execution", want{true, false, false, false}},
		{"delegate_explore", want{false, false, true, false}},
		{"task_output", want{true, false, false, true}},
		{"task_list_background", want{true, false, false, true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, d, ow, cs := surface.OrthogonalFlagFor(c.name)
			if r != c.want.readOnly || d != c.want.destructive ||
				ow != c.want.openWorld || cs != c.want.concurrencySafe {
				t.Errorf("OrthogonalFlagFor(%q) = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
					c.name, r, d, ow, cs,
					c.want.readOnly, c.want.destructive, c.want.openWorld, c.want.concurrencySafe)
			}
		})
	}
}

// T: TOOL-SURFACE-1-A01-T22 — OrthogonalFlagFor for unknown names returns
// all-false (the conservative default).
func TestOrthogonalFlagFor_UnknownReturnsAllFalse(t *testing.T) {
	r, d, ow, cs := surface.OrthogonalFlagFor("definitely_not_a_tool")
	if r || d || ow || cs {
		t.Errorf("unknown tool: expected all-false, got (%v, %v, %v, %v)", r, d, ow, cs)
	}
}

// T: TOOL-SURFACE-1-A01-T23 — InterruptBehaviorFor: free_fork and
// delegate_* return InterruptCancel; everything else returns
// InterruptBlock.
func TestInterruptBehaviorFor(t *testing.T) {
	cancelTools := []string{"free_fork", "delegate_explore", "delegate_plan", "delegate_implement"}
	for _, n := range cancelTools {
		if got := surface.InterruptBehaviorFor(n); got != contracts.InterruptCancel {
			t.Errorf("InterruptBehaviorFor(%q) = %q, want cancel", n, got)
		}
	}
	blockTools := []string{
		"read_file", "write_file", "edit_file", "bash",
		"grep", "glob", "lsp",
		"query_diagnostics", "verify_plan_execution",
		"task_output", "task_list_background",
		"unknown_tool",
	}
	for _, n := range blockTools {
		if got := surface.InterruptBehaviorFor(n); got != contracts.InterruptBlock {
			t.Errorf("InterruptBehaviorFor(%q) = %q, want block", n, got)
		}
	}
}

// allSurfacesUnderTest returns one instance of every surface in the
// canonical list, for the table-driven orthogonal flags / interrupt
// behavior tests. Each surface that needs a dependency is wired with a
// minimal safe-default (nil-reg for builtins → no tools; nil-tracker
// reports "not initialized" at Execute but Tools() still returns the
// spec, which is what the orthogonal-flag tests assert against).
func allSurfacesUnderTest(t *testing.T) []contracts.ToolSurface {
	t.Helper()
	return []contracts.ToolSurface{
		surface.NewBuiltinSurface(nil),
		surface.NewLSPToolSurface(nil),
		surface.NewTrackerSurface(nil),
		surface.NewFreeForkSurface(nil),
		surface.NewVerifySurface(),
		surface.NewDelegateSurface(),
		surface.NewBackgroundTaskSurface(),
	}
}

// T: TOOL-SURFACE-1-A01-T22 — All 7 surface Tools() methods populate
// the 4 orthogonal bools for every tool they expose. Uses a
// tab-driven subtest per surface (T5.1 in tasks.md).
func TestAllSurfaces_PopulateOrthogonalFlags(t *testing.T) {
	for _, s := range allSurfacesUnderTest(t) {
		t.Run(s.Name(), func(t *testing.T) {
			specs := s.Tools(context.Background(), "", "")
			if len(specs) == 0 {
				t.Skipf("surface %q exposes no tools", s.Name())
			}
			for _, sp := range specs {
				r, d, ow, cs := surface.OrthogonalFlagFor(sp.Name)
				if sp.ReadOnly != r || sp.Destructive != d ||
					sp.OpenWorld != ow || sp.ConcurrencySafe != cs {
					t.Errorf("surface %q tool %q: got flags (%v,%v,%v,%v), want (%v,%v,%v,%v)",
						s.Name(), sp.Name,
						sp.ReadOnly, sp.Destructive, sp.OpenWorld, sp.ConcurrencySafe,
						r, d, ow, cs)
				}
			}
		})
	}
}

// T: TOOL-SURFACE-1-A01-T23 — All 7 surfaces expose a working
// InterruptBehavior. The 6 short-run surfaces return InterruptBlock;
// FreeForkSurface returns InterruptCancel.
func TestAllSurfaces_HaveInterruptBehavior(t *testing.T) {
	for _, s := range allSurfacesUnderTest(t) {
		t.Run(s.Name(), func(t *testing.T) {
			specs := s.Tools(context.Background(), "", "")
			if len(specs) == 0 {
				t.Skipf("surface %q exposes no tools", s.Name())
			}
			for _, sp := range specs {
				got := s.InterruptBehavior(sp.Name)
				want := surface.InterruptBehaviorFor(sp.Name)
				if got != want {
					t.Errorf("surface %q tool %q: InterruptBehavior = %q, want %q",
						s.Name(), sp.Name, got, want)
				}
			}
		})
	}
}

// T: TOOL-SURFACE-1-A01-T23 — FreeForkSurface returns InterruptCancel
// specifically (the long-run opt-in). This is the hot test that the
// surface wires the cancel behavior.
func TestFreeForkSurface_InterruptBehavior_ReturnsCancel(t *testing.T) {
	s := surface.NewFreeForkSurface(nil)
	if got := s.InterruptBehavior("free_fork"); got != contracts.InterruptCancel {
		t.Errorf("InterruptBehavior(free_fork) = %q, want cancel", got)
	}
}

// T: TOOL-SURFACE-1-A01-T23 — Short-run surfaces (5 of them) all return
// InterruptBlock for their own tool. Verifies the conservative default.
// (delegate_* and free_fork are explicitly InterruptCancel — they have
// their own dedicated test cases below.)
func TestShortRunSurfaces_InterruptBehavior_ReturnsBlock(t *testing.T) {
	cases := []struct {
		name string
		surf contracts.ToolSurface
		tool string
	}{
		{"builtin/read_file", surface.NewBuiltinSurface(nil), "read_file"},
		{"lsp/lsp", surface.NewLSPToolSurface(nil), "lsp"},
		{"tracker/query_diagnostics", surface.NewTrackerSurface(nil), "query_diagnostics"},
		{"verify/verify_plan_execution", surface.NewVerifySurface(), "verify_plan_execution"},
		{"background/task_output", surface.NewBackgroundTaskSurface(), "task_output"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.surf.InterruptBehavior(c.tool); got != contracts.InterruptBlock {
				t.Errorf("InterruptBehavior(%q) on %q = %q, want block",
					c.tool, c.surf.Name(), got)
			}
		})
	}
}

// T: TOOL-SURFACE-1-A01-T23 — DelegateSurface returns InterruptCancel
// for delegate_* (long-run sub-agent spawns).
func TestDelegateSurface_InterruptBehavior_ReturnsCancel(t *testing.T) {
	s := surface.NewDelegateSurface()
	for _, n := range []string{"delegate_explore", "delegate_plan", "delegate_implement", "delegate_status"} {
		if got := s.InterruptBehavior(n); got != contracts.InterruptCancel {
			t.Errorf("InterruptBehavior(%q) = %q, want cancel", n, got)
		}
	}
}

// T: TOOL-SURFACE-1-A01-T22 — BuiltinSurface.Tools pulls 4 bool flags
// from OrthogonalFlagFor for each builtin tool, including the
// non-obvious cases (write_file = destructive but not concurrencySafe).
func TestBuiltinSurface_OrthogonalFlags_PerTool(t *testing.T) {
	reg, err := toolrunner.NewBuiltinToolRegistry(config.DefaultToolConfig())
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	s := surface.NewBuiltinSurface(reg)
	specs := s.Tools(context.Background(), "", "")
	if len(specs) == 0 {
		t.Fatal("BuiltinSurface.Tools returned no specs")
	}
	for _, sp := range specs {
		r, d, ow, cs := surface.OrthogonalFlagFor(sp.Name)
		if sp.ReadOnly != r || sp.Destructive != d ||
			sp.OpenWorld != ow || sp.ConcurrencySafe != cs {
			t.Errorf("builtin %q: got (%v,%v,%v,%v), want (%v,%v,%v,%v)",
				sp.Name,
				sp.ReadOnly, sp.Destructive, sp.OpenWorld, sp.ConcurrencySafe,
				r, d, ow, cs)
		}
	}
}
