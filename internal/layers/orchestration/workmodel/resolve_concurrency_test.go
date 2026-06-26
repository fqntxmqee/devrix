package workmodel

import (
	"sync"
	"testing"
)

func TestReevaluateParentAfterChild_ConcurrentTerminal(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "parent goal")
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: goal.ID, Kind: WorkKindPlan, Title: "parent", Directive: "parent",
	})
	c1, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: parent.ID, Kind: WorkKindImplement, Title: "c1", Directive: "c1",
	})
	c2, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{
		ParentID: parent.ID, Kind: WorkKindImplement, Title: "c2", Directive: "c2",
	})

	_ = tm.UpdateStatus("s1", c1.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", c2.ID, TaskStatusInProgress)
	_ = tm.UpdateStatus("s1", c1.ID, TaskStatusCompleted)
	_ = tm.UpdateStatus("s1", c2.ID, TaskStatusCompleted)

	var wg sync.WaitGroup
	for _, id := range []string{c1.ID, c2.ID} {
		wg.Add(1)
		go func(childID string) {
			defer wg.Done()
			ReevaluateParentAfterChild("s1", childID, tm)
		}(id)
	}
	wg.Wait()

	got, _ := tm.GetWorkItem("s1", parent.ID)
	if got.Status != TaskStatusCompleted {
		t.Fatalf("parent status = %q, want completed", got.Status)
	}
}
