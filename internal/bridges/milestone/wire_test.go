package milestonebridge_test

import (
	"testing"

	milestonebridge "github.com/devrix/devrix/internal/bridges/milestone"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestPlannerAdapter_should_create_batch_and_return_execution_order(t *testing.T) {
	svc := milestone.NewMilestoneService(nil)
	adapter := milestonebridge.NewPlannerAdapter(svc)

	ms := []*types.Milestone{
		types.NewMilestone("ms_1", "task_1", "first"),
	}
	ms = append(ms, func() *types.Milestone {
		m := types.NewMilestone("ms_2", "task_1", "second")
		m.AddDependency("ms_1")
		return m
	}())

	if err := adapter.CreateBatch("task_1", ms); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	order, err := adapter.GetExecutionOrder("task_1")
	if err != nil {
		t.Fatalf("GetExecutionOrder: %v", err)
	}
	if len(order) != 2 || order[0].ID != "ms_1" || order[1].ID != "ms_2" {
		t.Fatalf("unexpected order: %+v", order)
	}
}
