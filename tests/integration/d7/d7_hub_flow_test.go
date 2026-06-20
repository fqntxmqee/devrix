//go:build integration && d7

package d7integration

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/hub"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S4-T02, D7-S4-T03, D7-S1-T05
func TestIntegration_D7HubFlow_PublishLinksTaskAndQueue(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{ExecutionFlow: true})

	task, err := stack.TaskManager.Create("sess_hub", "explore module", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task == nil {
		t.Fatal("expected task")
	}

	hub, ok := stack.FlowHub.(*hub.Hub)
	if !ok {
		t.Fatalf("expected wired hub.Hub, got %T", stack.FlowHub)
	}
	_ = hub

	stack.FlowHub.Publish(context.Background(), contracts.FlowEvent{
		SessionID: "sess_hub",
		FlowID:    "flow-1",
		WorkerID:  "worker-1",
		TaskID:    task.ID,
		Source:    contracts.ExecutionSourceSubQuery,
		Role:      "explore",
		Kind:      contracts.FlowStarted,
		Summary:   "started explore worker",
	})

	got, ok := stack.TaskManager.Get("sess_hub", task.ID)
	if !ok {
		t.Fatal("task not found after FlowStarted")
	}
	if got.Owner != "worker-1" {
		t.Fatalf("owner = %q, want worker-1", got.Owner)
	}
	if got.Status != workmodel.TaskStatusInProgress {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}

	if stack.SessionQueue == nil {
		t.Fatal("expected session queue wired with execution flow hub")
	}
	drained := stack.SessionQueue.Drain("sess_hub", "", true)
	if len(drained) != 1 || drained[0].Mode != contracts.ModeDelegateProgress {
		t.Fatalf("delegate-progress queue = %+v", drained)
	}
}

// T: D7-S2-A04-T02, D7-S4-T08
func TestIntegration_D7Delegate_WiresHubSpokeDispatcher(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{Delegate: true})

	schemas, err := stack.Engine.ToolRegistry().ListTools(context.Background(), stack.WorkDir)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var hasDelegate bool
	for _, s := range schemas {
		if strings.HasPrefix(s.Name, "delegate_") {
			hasDelegate = true
			break
		}
	}
	if !hasDelegate {
		names := make([]string, 0, len(schemas))
		for _, s := range schemas {
			names = append(names, s.Name)
		}
		t.Fatalf("expected delegate tool registered, got: %v", names)
	}
}
