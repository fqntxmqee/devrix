package workmodel

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
)

func TestAwaitRunningChildren_BlocksUntilTerminal(t *testing.T) {
	reg := runregistry.NewRegistry("")
	runregistry.SetGlobal(reg)
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "p", Directive: "p"})
	child, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindExplore, Title: "c", Directive: "c"})
	runID, _ := SpawnForWorkItem("s1", child.ID, "explore", tm)

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		reg.SetTerminal(runID, runregistry.StatusCompleted, "explored", "")
		close(done)
	}()

	awaiter := &ResolveAwaiter{Manager: tm, Timeout: 2 * time.Second}
	summary := awaiter.AwaitRunningChildren(context.Background(), "s1")
	<-done

	if !strings.Contains(summary, "explored") {
		t.Fatalf("summary = %q", summary)
	}
	wi, _ := tm.GetWorkItem("s1", child.ID)
	if wi.Status != TaskStatusCompleted {
		t.Fatalf("child status = %s", wi.Status)
	}
}

func TestAwaitRunningChildren_NoRunningChildren(t *testing.T) {
	runregistry.SetGlobal(runregistry.NewRegistry(""))
	tm := NewTaskManager()
	_, _ = tm.EnsureGoal("s1", "g")
	awaiter := &ResolveAwaiter{Manager: tm}
	if got := awaiter.AwaitRunningChildren(context.Background(), "s1"); got != "" {
		t.Fatalf("summary = %q, want empty", got)
	}
}

func TestRunningChildrenWithRun_SkipsPending(t *testing.T) {
	runregistry.SetGlobal(runregistry.NewRegistry(""))
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("s1", "g")
	parent, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindPlan, Title: "p", Directive: "p"})
	pending, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindImplement, Title: "pending", Directive: "x"})
	running, _ := tm.CreateWorkItem("s1", CreateWorkItemInput{ParentID: parent.ID, Kind: WorkKindExplore, Title: "running", Directive: "x"})
	_, _ = SpawnForWorkItem("s1", running.ID, "explore", tm)

	got := runningChildrenWithRun(tm, "s1", parent.ID)
	if len(got) != 1 || got[0].ID != running.ID {
		t.Fatalf("got %d children, want only running id=%s (pending=%s)", len(got), running.ID, pending.ID)
	}
}
