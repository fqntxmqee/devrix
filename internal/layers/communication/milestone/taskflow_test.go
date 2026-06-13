package milestone

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestTaskFlowService_CreateStartComplete(t *testing.T) {
	dag := types.NewMilestoneDAG("task-1", "m1")
	m1 := types.NewMilestone("m1", "task-1", "step-1")
	if err := dag.AddMilestone(m1); err != nil {
		t.Fatalf("AddMilestone() error = %v", err)
	}

	ms := NewMilestoneService(dag)
	tfSvc := NewTaskFlowService(ms)

	tf, err := tfSvc.Create("flow-1", dag)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := tfSvc.Start(tf.ID); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := tfSvc.CompleteMilestone(tf.ID, "m1"); err != nil {
		t.Fatalf("CompleteMilestone() error = %v", err)
	}

	progress, err := tfSvc.GetProgress(tf.ID)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}
	if progress < 1.0 {
		t.Fatalf("progress = %v", progress)
	}
}

// T: D1-S5-A01-T02
func TestTaskFlowService_MultiMilestoneChain(t *testing.T) {
	dag := types.NewMilestoneDAG("task-1", "m1")
	m1 := types.NewMilestone("m1", "task-1", "step-1")
	m2 := types.NewMilestone("m2", "task-1", "step-2")
	m2.AddDependency("m1")
	if err := dag.AddMilestone(m1); err != nil {
		t.Fatalf("AddMilestone(m1) error = %v", err)
	}
	if err := dag.AddMilestone(m2); err != nil {
		t.Fatalf("AddMilestone(m2) error = %v", err)
	}

	ms := NewMilestoneService(dag)
	tfSvc := NewTaskFlowService(ms)

	tf, err := tfSvc.Create("flow-chain", dag)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := tfSvc.Start(tf.ID); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := tfSvc.CompleteMilestone(tf.ID, "m1"); err != nil {
		t.Fatalf("CompleteMilestone(m1) error = %v", err)
	}
	if err := tfSvc.CompleteMilestone(tf.ID, "m2"); err != nil {
		t.Fatalf("CompleteMilestone(m2) error = %v", err)
	}

	got, err := tfSvc.Get(tf.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != types.TaskFlowStatusCompleted {
		t.Fatalf("status = %s, want %s", got.Status, types.TaskFlowStatusCompleted)
	}
	progress, err := tfSvc.GetProgress(tf.ID)
	if err != nil {
		t.Fatalf("GetProgress() error = %v", err)
	}
	if progress < 1.0 {
		t.Fatalf("progress = %v", progress)
	}
}
