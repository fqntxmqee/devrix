package toolpolicy_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/orchestration/toolpolicy"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D4-S10-A01-T03
func TestFilterToolsForAgentRole_should_hide_delegate_from_worker(t *testing.T) {
	tools := []tools.ToolSchema{
		{Name: "read_file"},
		{Name: "delegate_explore"},
		{Name: "delegate_plan"},
	}
	sc := &types.SessionContext{IsWorker: true, WorkerRole: "explore", AgentID: "w1"}
	filtered := toolpolicy.FilterToolsForAgentRole(sc, tools)
	for _, tool := range filtered {
		if tool.Name == "delegate_explore" || tool.Name == "delegate_plan" {
			t.Fatalf("delegate tool %q should be hidden from worker", tool.Name)
		}
	}
}

func TestFilterToolsForAgentRole_should_keep_delegate_for_leader(t *testing.T) {
	tools := []tools.ToolSchema{
		{Name: "read_file"},
		{Name: "delegate_explore"},
	}
	sc := &types.SessionContext{SessionID: "sess1"}
	filtered := toolpolicy.FilterToolsForAgentRole(sc, tools)
	if len(filtered) != 2 {
		t.Fatalf("leader should see all tools, got %d", len(filtered))
	}
}
