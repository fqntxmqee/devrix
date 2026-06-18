package surface

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/ltllite"
)

// T: W15 — LSPToolSurface 4 条 invariant 在 init 阶段正确 parse。
func TestLSPInvariants_Parse_4Entries(t *testing.T) {
	if got := len(lspSurfaceInvariantSet.Invariants); got != 4 {
		t.Errorf("LSP invariant count = %d, want 4 (typed/readonly/concurrency/lowrisk)", got)
	}
}

// T: W15 — 所有 invariant 的 Pre/Post 非空。
func TestLSPInvariants_AllHavePrePost(t *testing.T) {
	for _, inv := range lspSurfaceInvariantSet.Invariants {
		if inv.Pre == "" || inv.Post == "" {
			t.Errorf("invariant %s has empty pre/post: %+v", inv.Name, inv)
		}
	}
}

// T: W15 — 全部命题为 true 时无 violation (LSP 满足所有 invariant)。
func TestCheckLSPInvariants_AllSatisfied_NoViolation(t *testing.T) {
	state := ltllite.MapState{
		"is_typed_method":           true,
		"typed_only":                true,
		"read_only":                 true,
		"no_destructive":            true,
		"is_concurrent_safe":        true,
		"single_call_idempotent":    true,
		"low_risk":                  true,
	}
	vs := CheckLSPInvariants(state)
	if len(vs) != 0 {
		t.Errorf("expected 0 violations when all props true, got %d: %+v", len(vs), vs)
	}
}

// T: W15 — 一条 invariant 违规时返回 1 violation (e.g. lsp_workspace_symbol 实际并发不安全)。
func TestCheckLSPInvariants_OneViolated(t *testing.T) {
	state := ltllite.MapState{
		"is_typed_method":        true,
		"typed_only":             true,
		"read_only":              true,
		"no_destructive":         true,
		"is_concurrent_safe":     true,  // pre=true
		"single_call_idempotent": false, // post=false → 违规
		"low_risk":               true,
	}
	vs := CheckLSPInvariants(state)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(vs))
	}
	if vs[0].Invariant.Name != "ConcurrencySafety" {
		t.Errorf("violation Name = %q, want ConcurrencySafety", vs[0].Invariant.Name)
	}
}
