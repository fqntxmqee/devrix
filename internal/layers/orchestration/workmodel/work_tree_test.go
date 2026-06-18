package workmodel

import (
	"errors"
	"os"
	"testing"
)

func TestWorkItem_ToTaskRoundTrip(t *testing.T) {
	item := NewWorkItem(WorkKindImplement, "Fix bug", "Fix auth bug")
	task := item.ToTask()
	back := WorkItemFromTask(task)
	if back.Title != item.Title {
		t.Fatalf("title mismatch")
	}
}

func TestWorkTree_CreateAndHierarchy(t *testing.T) {
	tree := NewWorkTree()
	goal, _ := tree.EnsureGoal("s1", "Build system")
	child, err := tree.Create("s1", CreateWorkItemInput{
		ParentID: goal.ID, Kind: WorkKindChecklist, Title: "Step 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.ListChildren("s1", goal.ID)) != 1 {
		t.Fatal("expected 1 child")
	}
	if len(tree.ListSubtree("s1", goal.ID)) != 2 {
		t.Fatal("expected subtree size 2")
	}
	_ = child
}

func TestWorkTree_AddDependencyCycle(t *testing.T) {
	tree := NewWorkTree()
	a, _ := tree.Create("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "A"})
	b, _ := tree.Create("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "B"})
	_ = tree.AddDependency("s1", b.ID, a.ID)
	err := tree.AddDependency("s1", a.ID, b.ID)
	if !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestWorkTree_RemoveCascade(t *testing.T) {
	tree := NewWorkTree()
	goal, _ := tree.EnsureGoal("s1", "G")
	child, _ := tree.Create("s1", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindImplement, Title: "C"})
	if err := tree.Remove("s1", goal.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := tree.Get("s1", child.ID); ok {
		t.Fatal("child should be removed")
	}
}

func TestWorkTree_GetFocusTiebreak(t *testing.T) {
	tree := NewWorkTree()
	a, _ := tree.Create("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "A", Uncertainty: 0.5})
	b, _ := tree.Create("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "B", Uncertainty: 0.5})
	focus, _ := tree.GetFocus("s1")
	if focus == nil {
		t.Fatal("expected focus")
	}
	if focus.ID != a.ID && focus.ID != b.ID {
		t.Fatalf("unexpected focus %s", focus.ID)
	}
}

func TestWorkTree_LockedItem(t *testing.T) {
	tree := NewWorkTree()
	item, _ := tree.Create("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "X"})
	_ = tree.UpdateStatus("s1", item.ID, TaskStatusInProgress)
	_ = tree.UpdateStatus("s1", item.ID, TaskStatusCompleted)
	err := tree.SetOwner("s1", item.ID, "agent")
	if !errors.Is(err, ErrWorkItemLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
}

func TestWorkTree_UpsertChecklist(t *testing.T) {
	tree := NewWorkTree()
	goal, _ := tree.EnsureGoal("s1", "Goal")
	_ = tree.UpsertChecklist("s1", goal.ID, []ChecklistEntry{
		{Content: "Step 1", Status: TaskStatusPending},
		{Content: "Step 2", Status: TaskStatusInProgress},
	})
	children := tree.ListChildren("s1", goal.ID)
	if len(children) != 2 {
		t.Fatalf("expected 2 checklist items, got %d", len(children))
	}
}

func TestDiskWorkItemStore_V1Migration(t *testing.T) {
	dir := t.TempDir()
	v1, _ := NewDiskTaskStore(dir)
	task := NewTask("Legacy", "desc")
	if err := v1.Save("sess", []*Task{task}); err != nil {
		t.Fatal(err)
	}
	v2, _ := NewDiskWorkItemStore(dir)
	items, err := v2.Load("sess")
	if err != nil || len(items) != 1 {
		t.Fatalf("load: %v len=%d", err, len(items))
	}
}

func TestDiskWorkItemStore_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewDiskWorkItemStore(dir)
	path := store.path("empty")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	items, err := store.Load("empty")
	if err != nil || len(items) != 0 {
		t.Fatalf("expected empty, got %v items=%d", err, len(items))
	}
}

func TestTaskManager_DelegatesToWorkTree(t *testing.T) {
	m := NewTaskManager()
	task := m.Create("s1", "Task", "Desc")
	item, ok := m.Tree().Get("s1", task.ID)
	if !ok || item.Kind != WorkKindImplement {
		t.Fatal("expected implement work item")
	}
	if item.ParentID == "" {
		t.Fatal("expected parent goal")
	}
}
