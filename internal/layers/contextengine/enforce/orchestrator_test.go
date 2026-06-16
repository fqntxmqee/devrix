package enforce_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubPermissionGate struct{ allowed bool }

func (s *stubPermissionGate) Check(_ enforce.ToolCall) (bool, string) {
	return s.allowed, ""
}

type stubToolSandbox struct{}

func (s *stubToolSandbox) Execute(_ context.Context, call enforce.ToolCall) enforce.ToolResult {
	return enforce.ToolResult{CallID: call.ID, Output: call.Name + "_output"}
}

// T: D2-S18-A01-T51
func TestPolicyOrchestrator_EnforceToolRound_allows_when_permission_passes(t *testing.T) {
	orch := enforce.NewPolicyOrchestrator(enforce.PolicyDeps{
		PermissionGate: &stubPermissionGate{allowed: true},
		ToolSandbox:    &stubToolSandbox{},
	})
	results := orch.EnforceToolRound(context.Background(), []enforce.ToolCall{
		{ID: "1", Name: "read", RiskLevel: types.RiskLevelLow},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Output != "read_output" {
		t.Fatalf("expected 'read_output', got %q", results[0].Output)
	}
}

// T: D2-S18-A01-T52
func TestPolicyOrchestrator_EnforceToolRound_denies_when_permission_fails(t *testing.T) {
	orch := enforce.NewPolicyOrchestrator(enforce.PolicyDeps{
		PermissionGate: &stubPermissionGate{allowed: false},
	})
	results := orch.EnforceToolRound(context.Background(), []enforce.ToolCall{
		{ID: "1", Name: "write", RiskLevel: types.RiskLevelHigh},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != "permission denied" {
		t.Fatalf("expected 'permission denied', got %q", results[0].Error)
	}
}

// T: D2-S18-A01-T53
func TestPolicyOrchestrator_EnforceToolRound_empty_calls_returns_empty(t *testing.T) {
	orch := enforce.NewPolicyOrchestrator(enforce.PolicyDeps{})
	results := orch.EnforceToolRound(context.Background(), nil)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// T: D2-S18-A01-T54
func TestPolicyOrchestrator_EnforceToolRound_multiple_mixed_permissions(t *testing.T) {
	orch := enforce.NewPolicyOrchestrator(enforce.PolicyDeps{
		PermissionGate: &stubPermissionGate{allowed: true},
		ToolSandbox:    &stubToolSandbox{},
	})
	results := orch.EnforceToolRound(context.Background(), []enforce.ToolCall{
		{ID: "1", Name: "read"},
		{ID: "2", Name: "grep"},
		{ID: "3", Name: "write"},
	})
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.CallID == "" {
			t.Fatalf("result[%d] has empty CallID", i)
		}
	}
}

// T: D2-S18-A01-T55
func TestNewPolicyOrchestrator_nil_deps_does_not_panic(t *testing.T) {
	orch := enforce.NewPolicyOrchestrator(enforce.PolicyDeps{})
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	results := orch.EnforceToolRound(context.Background(), []enforce.ToolCall{
		{ID: "1", Name: "read"},
	})
	// With nil deps, no gate to allow and no sandbox to execute → 0 results.
	if len(results) != 0 {
		t.Fatalf("expected 0 results with nil deps, got %d", len(results))
	}
}
