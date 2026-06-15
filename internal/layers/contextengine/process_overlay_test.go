package contextengine

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D4-S10-A01-T02
func TestForkWorkerSessionContext_should_isolate_worker_identity(t *testing.T) {
	parent := &types.SessionContext{
		SessionID:    "sess_1",
		WorkDir:      "/tmp",
		Model:        "gpt-test",
		QueryChainID: "sess_1",
		QueryDepth:   0,
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "leader msg"}},
	}
	child := forkWorkerSessionContext(parent, contracts.ProcessOverlay{
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
