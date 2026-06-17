package contracts_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func specs(names ...string) []contracts.ToolSpec {
	out := make([]contracts.ToolSpec, len(names))
	for i, n := range names {
		out[i] = contracts.ToolSpec{Name: n, Risk: types.RiskLevelLow}
	}
	return out
}

// T: TOOL-SURFACE-1-T02 — Composite applies filters FIFO.
func TestComposite_FIFO(t *testing.T) {
	chain := contracts.Composite(
		contracts.Allow("a", "b", "c"),
		contracts.Allow("b", "c", "d"),
	)
	got := chain.Apply(specs("a", "b", "c", "d", "e"), contracts.FilterCtx{})
	want := []string{"b", "c"}
	if !equalNames(got, want) {
		t.Errorf("Composite FIFO = %v, want %v", namesOf(got), want)
	}
}

// T: TOOL-SURFACE-1-T02 — Empty composite passes through unchanged.
func TestComposite_Empty(t *testing.T) {
	got := contracts.Composite().Apply(specs("a", "b"), contracts.FilterCtx{})
	if !equalNames(got, []string{"a", "b"}) {
		t.Errorf("empty composite = %v, want [a b]", namesOf(got))
	}
}

// T: TOOL-SURFACE-1-T02 — Nil filter in composite is skipped.
func TestComposite_NilSkipped(t *testing.T) {
	chain := contracts.Composite(nil, contracts.Allow("a"))
	got := chain.Apply(specs("a", "b", "c"), contracts.FilterCtx{})
	if !equalNames(got, []string{"a"}) {
		t.Errorf("composite with nil = %v, want [a]", namesOf(got))
	}
}

// T: TOOL-SURFACE-1-T02 — Allow keeps only listed names.
func TestAllow(t *testing.T) {
	got := contracts.Allow("read_file", "bash").Apply(specs("read_file", "write_file", "bash"), contracts.FilterCtx{})
	if !equalNames(got, []string{"read_file", "bash"}) {
		t.Errorf("Allow = %v, want [read_file bash]", namesOf(got))
	}
}

// T: TOOL-SURFACE-1-T02 — Allow with empty list keeps nothing.
func TestAllow_Empty(t *testing.T) {
	got := contracts.Allow().Apply(specs("a", "b"), contracts.FilterCtx{})
	if len(got) != 0 {
		t.Errorf("Allow() = %v, want empty", namesOf(got))
	}
}

// T: TOOL-SURFACE-1-T02 — Deny removes listed names.
func TestDeny(t *testing.T) {
	got := contracts.Deny("write_file").Apply(specs("read_file", "write_file", "bash"), contracts.FilterCtx{})
	if !equalNames(got, []string{"read_file", "bash"}) {
		t.Errorf("Deny = %v, want [read_file bash]", namesOf(got))
	}
}

// T: TOOL-SURFACE-1-T02 — Deny with empty list passes everything through.
func TestDeny_Empty(t *testing.T) {
	got := contracts.Deny().Apply(specs("a", "b"), contracts.FilterCtx{})
	if !equalNames(got, []string{"a", "b"}) {
		t.Errorf("Deny() = %v, want [a b]", namesOf(got))
	}
}

// T: TOOL-SURFACE-1-T02 — ApplyFilters wraps each surface in a
// filteredSurface; Execute on a non-visible tool returns error without
// touching the parent.
func TestApplyFilters_BlockedNotExecutable(t *testing.T) {
	parent := &stubSurface{
		name:   "free_fork",
		tools:  specs("free_fork", "verify"),
		risk:   types.RiskLevelHigh,
		output: "ok",
	}
	wrapped := contracts.ApplyFilters(
		[]contracts.ToolSurface{parent},
		[]contracts.ToolFilter{contracts.Allow("verify")},
		contracts.FilterCtx{AgentType: "explore"},
	)
	if len(wrapped) != 1 {
		t.Fatalf("wrapped len = %d, want 1", len(wrapped))
	}
	visible := wrapped[0].Tools(context.Background(), "", "")
	if !equalNames(visible, []string{"verify"}) {
		t.Errorf("visible = %v, want [verify]", namesOf(visible))
	}
	// free_fork is not visible → Execute should fail
	res, err := wrapped[0].Execute(context.Background(), "free_fork", "{}", "/tmp")
	if err != nil {
		t.Fatalf("Execute err = %v, want nil (ToolResult.Error)", err)
	}
	if res.Error == "" {
		t.Error("Execute Error empty, want not-visible error")
	}
	// verify is visible → Execute delegates to parent (output "ok")
	res, err = wrapped[0].Execute(context.Background(), "verify", "{}", "/tmp")
	if err != nil {
		t.Fatalf("Execute verify err = %v", err)
	}
	if res.Error != "" {
		t.Errorf("verify Execute Error = %q, want empty", res.Error)
	}
	if res.Output != "ok" {
		t.Errorf("verify Execute Output = %q, want ok", res.Output)
	}
}

// T: TOOL-SURFACE-1-T02 — ApplyFilters with no filters returns surfaces
// unchanged.
func TestApplyFilters_NoFilters(t *testing.T) {
	parent := &stubSurface{name: "x", tools: specs("a")}
	got := contracts.ApplyFilters([]contracts.ToolSurface{parent}, nil, contracts.FilterCtx{})
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Name() != "x" {
		t.Errorf("Name = %q, want x", got[0].Name())
	}
}

// T: TOOL-SURFACE-1-T02 — FilterCtx zero value is valid.
func TestFilterCtx_Zero(t *testing.T) {
	ctx := contracts.FilterCtx{}
	if ctx.SessionID != "" || ctx.AgentType != "" || ctx.Mode != "" {
		t.Errorf("zero ctx = %+v, want all empty", ctx)
	}
}

// T: TOOL-SURFACE-1-T02 — Filter does not mutate input slice.
func TestFilter_NoMutation(t *testing.T) {
	in := specs("a", "b", "c")
	saved := append([]contracts.ToolSpec(nil), in...)
	contracts.Allow("a").Apply(in, contracts.FilterCtx{})
	for i, s := range in {
		if s.Name != saved[i].Name {
			t.Errorf("input mutated at %d: %q -> %q", i, saved[i].Name, s.Name)
		}
	}
}

// helpers

func namesOf(specs []contracts.ToolSpec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}

func equalNames(specs []contracts.ToolSpec, want []string) bool {
	got := namesOf(specs)
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
