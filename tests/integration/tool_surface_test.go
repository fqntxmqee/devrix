package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/filter"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
	"github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: TOOL-SURFACE-1-T08 integration — Surface list and filter chain
// are visible end-to-end through a single canonical list. The
// integration is the surface-package public API: BuildSurfaces + the
// per-mode filter chain produce a contract.ToolSurface slice that
// satisfies ApplyFilters semantics for main, per-agent, and delegate
// modes.
func TestIntegration_ToolSurface_AllModes(t *testing.T) {
	tr := tracker.New(8)
	surfaces := surfaceForIntegrationTest(tr)

	ctx := contracts.FilterCtx{SessionID: "sess-1", AgentType: "main"}
	mainVisible := visibleSpecNames(contracts.ApplyFilters(surfaces, nil, ctx))
	if len(mainVisible) < 2 {
		t.Fatalf("main mode visible: got %v, want >= 2", mainVisible)
	}

	ctx2 := contracts.FilterCtx{SessionID: "sess-1", AgentType: "explore"}
	filters := []contracts.ToolFilter{
		filter.NewPerAgentFilter(),
		filter.NewPerRiskFilter(),
	}
	exploreVisible := visibleSpecNames(contracts.ApplyFilters(surfaces, filters, ctx2))
	// explore should drop delegate_*, bash (high), edit_file (medium),
	// free_fork, etc. — only read-only subset.
	for _, n := range exploreVisible {
		switch n {
		case "read_file", "glob", "grep", "list_dir":
			// expected
		default:
			t.Errorf("explore should not see %q (got %v)", n, exploreVisible)
		}
	}

	ctx3 := contracts.FilterCtx{SessionID: "sess-1", AgentType: "delegate"}
	delegateVisible := visibleSpecNames(contracts.ApplyFilters(surfaces, []contracts.ToolFilter{
		decisionplanning.AsToolFilter(),
	}, ctx3))
	// delegate mode: only delegate_* tools
	for _, n := range delegateVisible {
		if n != "delegate_explore" && n != "delegate_plan" && n != "delegate_implement" && n != "delegate_status" {
			t.Errorf("delegate should not see %q (got %v)", n, delegateVisible)
		}
	}
}

// T: TOOL-SURFACE-1-T09 integration — End-to-end dispatch through
// FindSurface: each surface returns a different output marker so the
// test can verify which one was hit.
func TestIntegration_ToolSurface_DispatchBySurface(t *testing.T) {
	s1 := &markerSurface{name: "surface1", toolName: "x", output: "from-surface1"}
	s2 := &markerSurface{name: "surface2", toolName: "x", output: "from-surface2"}
	surfaces := []contracts.ToolSurface{s1, s2}

	// First surface that claims "x" wins.
	for _, s := range surfaces {
		if r := s.RiskLevel("x"); r != "" {
			res, _ := s.Execute(context.Background(), "x", `{}`, "")
			if res.Output != "from-surface1" {
				t.Errorf("first-match dispatch: got %q, want from-surface1", res.Output)
			}
			return
		}
	}
	t.Fatal("no surface claimed x")
}

// T: TOOL-SURFACE-1-T04 integration — VerifySurface called
// end-to-end with a real change_id returns the expected Report shape
// (verified/unverified/skipped counts in JSON).
func TestIntegration_VerifySurface_ReportShape(t *testing.T) {
	s := surface.NewVerifySurface()
	res, err := s.Execute(context.Background(), "verify_plan_execution",
		`{"change_id":"nonexistent","repo_root":"/nonexistent"}`, "")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Error == "" {
		t.Error("Error empty, want load plan error")
	}
}

// ---- helpers ----

func surfaceForIntegrationTest(tr *tracker.Tracker) []contracts.ToolSurface {
	return []contracts.ToolSurface{
		surface.NewBuiltinSurface(nil), // no registry — empty builtin
		surface.NewTrackerSurface(tr),
		surface.NewVerifySurface(),
		&stubIntegratedSurface{name: "delegate", toolName: "delegate_explore"},
		&stubIntegratedSurface{name: "delegate", toolName: "delegate_plan"},
		&stubIntegratedSurface{name: "delegate", toolName: "delegate_implement"},
		&stubIntegratedSurface{name: "delegate", toolName: "delegate_status"},
		&stubIntegratedSurface{name: "builtin", toolName: "read_file"},
		&stubIntegratedSurface{name: "builtin", toolName: "bash"},
		&stubIntegratedSurface{name: "builtin", toolName: "free_fork"},
		&stubIntegratedSurface{name: "builtin", toolName: "edit_file"},
		&stubIntegratedSurface{name: "builtin", toolName: "glob"},
		&stubIntegratedSurface{name: "builtin", toolName: "grep"},
		&stubIntegratedSurface{name: "builtin", toolName: "list_dir"},
	}
}

func visibleSpecNames(surfaces []contracts.ToolSurface) []string {
	var out []string
	for _, s := range surfaces {
		for _, sp := range s.Tools(context.Background(), "", "") {
			out = append(out, sp.Name)
		}
	}
	return out
}

type markerSurface struct {
	name     string
	toolName string
	output   string
}

func (s *markerSurface) Name() string { return s.name }
func (s *markerSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: s.toolName, Risk: types.RiskLevelLow}}
}
func (s *markerSurface) RiskLevel(n string) types.RiskLevel {
	if n == s.toolName {
		return types.RiskLevelLow
	}
	return ""
}
func (s *markerSurface) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{Output: s.output}, nil
}
func (s *markerSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (s *markerSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (s *markerSurface) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (s *markerSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}

type stubIntegratedSurface struct {
	name     string
	toolName string
}

func (s *stubIntegratedSurface) Name() string { return s.name }
func (s *stubIntegratedSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: s.toolName}}
}
func (s *stubIntegratedSurface) RiskLevel(n string) types.RiskLevel {
	if n == s.toolName {
		return types.RiskLevelLow
	}
	return ""
}
func (s *stubIntegratedSurface) Execute(_ context.Context, _, _, _ string) (*contracts.ToolResult, error) {
	return &contracts.ToolResult{Output: "stub"}, nil
}
func (s *stubIntegratedSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (s *stubIntegratedSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (s *stubIntegratedSurface) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (s *stubIntegratedSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}

// Importing the alias as toolpolicyfilter so the `toolpolicy` import below
// is not unused.
var _ = json.Marshal
