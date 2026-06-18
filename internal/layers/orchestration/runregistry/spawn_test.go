package runregistry

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestSpawnForWorkItem_SyncTerminal(t *testing.T) {
	reg := NewRegistry("")
	SetGlobal(reg)
	tm := workmodel.NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	item, _ := tm.CreateWorkItem("s1", workmodel.CreateWorkItemInput{
		ParentID: goal.ID, Kind: workmodel.WorkKindExplore, Title: "x", Directive: "x",
	})
	runID, _ := SpawnForWorkItem("s1", item.ID, "explore", tm)
	if runID == "" {
		t.Fatal("expected run id")
	}
	wi, _ := tm.GetWorkItem("s1", item.ID)
	if wi.RunRef != runID || wi.Status != workmodel.TaskStatusInProgress {
		t.Fatalf("run_ref=%q status=%s", wi.RunRef, wi.Status)
	}
	reg.SetTerminal(runID, StatusCompleted, "done", "")
	wi, _ = tm.GetWorkItem("s1", item.ID)
	if wi.Status != workmodel.TaskStatusCompleted {
		t.Fatalf("status = %s, want completed", wi.Status)
	}
}
