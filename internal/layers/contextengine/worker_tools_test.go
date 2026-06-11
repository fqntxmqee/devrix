package contextengine

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-4-10-02
func TestForkWorkerSessionContext_should_isolate_worker_identity(t *testing.T) {
	parent := &types.SessionContext{
		SessionID:    "sess_1",
		WorkDir:      "/tmp",
		Model:        "gpt-test",
		QueryChainID: "sess_1",
		QueryDepth:   0,
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "leader msg"}},
	}
	child := forkWorkerSessionContext(parent, ProcessOverlay{
		AgentID:      "worker_a",
		IsWorker:     true,
		WorkerRole:   "explore",
		SystemPrompt: "explore only",
	})
	if child.AgentID != "worker_a" || !child.IsWorker || child.WorkerRole != "explore" {
		t.Fatalf("worker identity not set: %+v", child)
	}
	if len(child.Messages) != 0 {
		t.Fatalf("worker should start with empty messages, got %d", len(child.Messages))
	}
	if child.QueryDepth != parent.QueryDepth+1 {
		t.Fatalf("QueryDepth = %d, want %d", child.QueryDepth, parent.QueryDepth+1)
	}
	if child.SystemPrompt != "explore only" {
		t.Fatalf("SystemPrompt = %q", child.SystemPrompt)
	}
}

// Covers: L5-4-10-03
func TestFilterToolsForAgentRole_should_hide_delegate_from_worker(t *testing.T) {
	tools := []ToolSchema{
		{Name: "read_file"},
		{Name: "delegate_explore"},
		{Name: "delegate_plan"},
	}
	sc := &types.SessionContext{IsWorker: true, WorkerRole: "explore", AgentID: "w1"}
	filtered := FilterToolsForAgentRole(sc, tools)
	for _, tool := range filtered {
		if tool.Name == "delegate_explore" || tool.Name == "delegate_plan" {
			t.Fatalf("delegate tool %q should be hidden from worker", tool.Name)
		}
	}
}
