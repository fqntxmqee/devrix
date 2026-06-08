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
