package flow_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type captureIM struct {
	events []contracts.FlowEvent
}

func (c *captureIM) EmitWorkerProgress(ev contracts.FlowEvent) {
	c.events = append(c.events, ev)
}

func TestHub_should_dual_publish_queue_and_im(t *testing.T) {
	q := queue.NewSessionQueue()
	im := &captureIM{}
	hub := flow.NewHub(flow.HubDeps{
		Config: config.ExecutionFlowConfig{Enabled: true, LinkTasks: false, IMProgress: true},
		Queue:  q,
		IM:     im,
	})

	hub.Publish(context.Background(), contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "explore_1",
		WorkerID:  "explore_1",
		Source:    contracts.ExecutionSourceSubQuery,
		Role:      "explore",
		Kind:      contracts.FlowStarted,
		Summary:   "started explore",
	})

	drained := q.Drain("sess1", "", true)
	if len(drained) != 1 || drained[0].Mode != queue.ModeDelegateProgress {
		t.Fatalf("leader queue drain = %+v", drained)
	}
	if len(im.events) != 1 || im.events[0].Kind != contracts.FlowStarted {
		t.Fatalf("im events = %+v", im.events)
	}
}

func TestHub_should_link_task_owner_on_started(t *testing.T) {
	q := queue.NewSessionQueue()
	tm := tasks.NewTaskManager()
	task := tm.Create("sess1", "explore auth", "")
	hub := flow.NewHub(flow.HubDeps{
		Config: config.ExecutionFlowConfig{Enabled: true, LinkTasks: true, IMProgress: false},
		Queue:  q,
		Tasks:  tm,
	})

	hub.Publish(context.Background(), contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		TaskID:    task.ID,
		Source:    contracts.ExecutionSourceSubQuery,
		Kind:      contracts.FlowStarted,
	})

	got, ok := tm.Get("sess1", task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if got.Owner != "w1" || got.Status != tasks.TaskStatusInProgress {
		t.Fatalf("task = %+v", got)
	}
}

// T: D4-S10-A02-T07
func TestHub_should_mark_task_completed_on_flow_done(t *testing.T) {
	tm := tasks.NewTaskManager()
	task := tm.Create("sess1", "implement feature", "")
	hub := flow.NewHub(flow.HubDeps{
		Config: config.ExecutionFlowConfig{Enabled: true, LinkTasks: true},
		Queue:  queue.NewSessionQueue(),
		Tasks:  tm,
	})

	hub.Publish(context.Background(), contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		TaskID:    task.ID,
		Kind:      contracts.FlowStarted,
	})
	hub.Publish(context.Background(), contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		TaskID:    task.ID,
		Kind:      contracts.FlowCompleted,
	})

	got, ok := tm.Get("sess1", task.ID)
	if !ok {
		t.Fatal("task not found")
	}
	if got.Status != tasks.TaskStatusCompleted {
		t.Fatalf("status = %q, want completed", got.Status)
	}
}

// T: D4-S10-A02-T04 (WorkPlan includes task projection)
func TestHub_snapshot_should_include_tasks(t *testing.T) {
	tm := tasks.NewTaskManager()
	task := tm.Create("sess1", "auth audit", "")
	hub := flow.NewHub(flow.HubDeps{
		Config: config.ExecutionFlowConfig{Enabled: true, LinkTasks: true},
		Queue:  queue.NewSessionQueue(),
		Tasks:  tm,
	})
	hub.Publish(context.Background(), contracts.FlowEvent{
		SessionID: "sess1",
		FlowID:    "w1",
		WorkerID:  "w1",
		TaskID:    task.ID,
		Kind:      contracts.FlowStarted,
	})

	snap := hub.Snapshot("sess1")
	if len(snap.Tasks) != 1 || snap.Tasks[0].ID != task.ID {
		t.Fatalf("tasks = %+v", snap.Tasks)
	}
	if len(snap.ExecutionFlows) != 1 || snap.ExecutionFlows[0].TaskID != task.ID {
		t.Fatalf("flows = %+v", snap.ExecutionFlows)
	}
}

func TestHub_should_not_emit_tool_call_to_im(t *testing.T) {
	im := &captureIM{}
	q := queue.NewSessionQueue()
	hub := flow.NewHub(flow.HubDeps{
		Config: config.ExecutionFlowConfig{
			Enabled:               true,
			IMProgress:            true,
			ToolSummaryThrottleMs: 60000,
		},
		Queue: q,
		IM:    im,
	})
	ev := contracts.FlowEvent{
		SessionID: "sess1",
		WorkerID:  "w1",
		FlowID:    "w1",
		Kind:      contracts.FlowToolCall,
		Summary:   "grep",
	}
	hub.Publish(context.Background(), ev)
	hub.Publish(context.Background(), ev)
	if len(im.events) != 0 {
		t.Fatalf("tool_call should not go to IM, got %d", len(im.events))
	}
	if drained := q.Drain("sess1", "", true); len(drained) != 1 {
		t.Fatalf("leader queue should receive 1 throttled tool event, got %d", len(drained))
	}
}
