package workmodel

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
)

func TestSpawnForWorkItem_SyncTerminal(t *testing.T) {
	reg := runregistry.NewRegistry("")
	tm := NewTaskManager().SetRegistry(reg)
	goal, _ := tm.EnsureGoal("s1", "g")
	item, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindExplore, Title: "x", Directive: "x"})
	runID, _ := SpawnForWorkItem("s1", item.ID, "explore", tm)
	if runID == "" {
		t.Fatal("expected run id")
	}
	wi, _ := tm.GetWorkItem("s1", item.ID)
	if wi.RunRef != runID || wi.Status != TaskStatusInProgress {
		t.Fatalf("run_ref=%q status=%s", wi.RunRef, wi.Status)
	}
	reg.SetTerminal(runID, runregistry.StatusCompleted, "done", "")
	wi, _ = tm.GetWorkItem("s1", item.ID)
	if wi.Status != TaskStatusCompleted {
		t.Fatalf("status = %s, want completed", wi.Status)
	}
}
