package toolpolicy_test

import (
	"reflect"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func mkSpec(name string, risk types.RiskLevel) contracts.ToolSpec {
	return contracts.ToolSpec{Name: name, Risk: risk}
}

func specNames(specs []contracts.ToolSpec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}

// fullSet is the "everything the LLM could possibly see" input used by
// most tests. The expected outputs are the residual after the adapter
// has applied its policy.
func fullSet() []contracts.ToolSpec {
	return []contracts.ToolSpec{
		mkSpec("read_file", types.RiskLevelLow),
		mkSpec("glob", types.RiskLevelLow),
		mkSpec("grep", types.RiskLevelLow),
		mkSpec("list_dir", types.RiskLevelLow),
		mkSpec("bash", types.RiskLevelHigh),
		mkSpec("edit_file", types.RiskLevelMedium),
		mkSpec("enter_plan_mode", types.RiskLevelLow),
		mkSpec("exit_plan_mode", types.RiskLevelLow),
		mkSpec("todo_write", types.RiskLevelLow),
		mkSpec("task_create", types.RiskLevelLow),
		mkSpec("task_get", types.RiskLevelLow),
		mkSpec("task_list", types.RiskLevelLow),
		mkSpec("task_update", types.RiskLevelLow),
		mkSpec("delegate_explore", types.RiskLevelLow),
		mkSpec("delegate_plan", types.RiskLevelLow),
		mkSpec("delegate_implement", types.RiskLevelLow),
		mkSpec("delegate_status", types.RiskLevelLow),
		mkSpec("free_fork", types.RiskLevelHigh),
	}
}

// T: TOOL-FILTER-1-T04 — main/fix/empty: pass-through (no filter applied).
func TestAdapter_Main_PassThrough(t *testing.T) {
	f := toolpolicy.AsToolFilter()
	specs := fullSet()
	got := f.Apply(specs, contracts.FilterCtx{AgentType: "main"})
	if !reflect.DeepEqual(specNames(got), specNames(specs)) {
		t.Errorf("main: got %v, want %v", specNames(got), specNames(specs))
	}
	got2 := f.Apply(specs, contracts.FilterCtx{AgentType: "fix"})
	if !reflect.DeepEqual(specNames(got2), specNames(specs)) {
		t.Errorf("fix: got %v, want %v", specNames(got2), specNames(specs))
	}
	got3 := f.Apply(specs, contracts.FilterCtx{})
	if !reflect.DeepEqual(specNames(got3), specNames(specs)) {
		t.Errorf("empty: got %v, want %v", specNames(got3), specNames(specs))
	}
}

// T: TOOL-FILTER-1-T04 — delegate: only delegate_* tools.
func TestAdapter_Delegate_OnlyDelegates(t *testing.T) {
	f := toolpolicy.AsToolFilter()
	got := f.Apply(fullSet(), contracts.FilterCtx{AgentType: "delegate"})
	want := []string{"delegate_explore", "delegate_plan", "delegate_implement", "delegate_status"}
	if !reflect.DeepEqual(specNames(got), want) {
		t.Errorf("delegate: got %v, want %v", specNames(got), want)
	}
}

// T: TOOL-FILTER-1-T04 — explore/plan: read-only worker set, no delegate_*.
func TestAdapter_Explore_ReadOnlyNoDelegate(t *testing.T) {
	f := toolpolicy.AsToolFilter()
	for _, role := range []string{"explore", "plan"} {
		got := f.Apply(fullSet(), contracts.FilterCtx{AgentType: role})
		for _, s := range got {
			if s.Name == "delegate_explore" || s.Name == "delegate_plan" || s.Name == "delegate_implement" || s.Name == "delegate_status" {
				t.Errorf("%s: leaked %s", role, s.Name)
			}
		}
		// Must include at least one read-only tool
		found := false
		for _, s := range got {
			if s.Name == "read_file" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: read_file missing", role)
		}
	}
}

// T: TOOL-FILTER-1-T04 — worker: hides delegate_*, keeps full worker set.
func TestAdapter_Worker_NoDelegate(t *testing.T) {
	f := toolpolicy.AsToolFilter()
	got := f.Apply(fullSet(), contracts.FilterCtx{AgentType: "worker"})
	for _, s := range got {
		if s.Name == "delegate_explore" || s.Name == "delegate_plan" || s.Name == "delegate_implement" || s.Name == "delegate_status" {
			t.Errorf("worker: leaked %s", s.Name)
		}
	}
	// Worker keeps edit_file (full worker set, not explore/plan restricted)
	found := false
	for _, s := range got {
		if s.Name == "edit_file" {
			found = true
		}
	}
	if !found {
		t.Error("worker: edit_file missing (full worker set expected)")
	}
}

// T: TOOL-FILTER-1-T04 — unknown agent: conservative (hide delegate_* + read-only).
func TestAdapter_Unknown_Conservative(t *testing.T) {
	f := toolpolicy.AsToolFilter()
	got := f.Apply(fullSet(), contracts.FilterCtx{AgentType: "hypothetical"})
	for _, s := range got {
		if s.Name == "delegate_explore" || s.Name == "delegate_plan" || s.Name == "delegate_implement" || s.Name == "delegate_status" {
			t.Errorf("unknown: leaked %s", s.Name)
		}
	}
	// Unknown gets the worker read-only set
	found := false
	for _, s := range got {
		if s.Name == "read_file" {
			found = true
		}
	}
	if !found {
		t.Error("unknown: read_file missing (conservative read-only expected)")
	}
}

// T: TOOL-FILTER-1-T04 — Adapter is a valid contracts.ToolFilter.
func TestAdapter_InterfaceCompliance(t *testing.T) {
	var _ contracts.ToolFilter = toolpolicy.AsToolFilter()
}
