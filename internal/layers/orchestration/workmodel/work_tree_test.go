package workmodel

import (
	"errors"
	"os"
	"strings"
	"testing"
)

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

func TestDiskWorkItemStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewDiskWorkItemStore(dir)
	want := NewWorkItem(WorkKindImplement, "Build feature", "Direct")
	if err := store.Save("sess", []*WorkItem{want}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("sess")
	if err != nil || len(got) != 1 {
		t.Fatalf("load: %v len=%d", err, len(got))
	}
	if got[0].ID != want.ID || got[0].Title != want.Title || got[0].Directive != want.Directive {
		t.Fatalf("round-trip mismatch: %+v", got[0])
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
	goal, err := m.Tree().EnsureGoal("s1", "build something")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	item, err := m.Tree().Create("s1", CreateWorkItemInput{
		ParentID:  goal.ID,
		Kind:      WorkKindImplement,
		Title:     "Task",
		Directive: "Desc",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, ok := m.Tree().Get("s1", item.ID)
	if !ok || got.Kind != WorkKindImplement {
		t.Fatal("expected implement work item")
	}
	if got.ParentID != goal.ID {
		t.Fatalf("expected parent goal %q, got %q", goal.ID, got.ParentID)
	}
}

// TestEnsureGoal_RefreshesDirectiveOnPivot reproduces the focus-hint
// context-bleed (2026-06-20): when the user issues multiple turns with
// different directives, EnsureGoal must update the existing unlocked
// goal's Title/Directive so downstream prompts reflect the current intent
// instead of remaining stuck at the first message. Children stay attached
// to the same parent ID across the refresh.
func TestEnsureGoal_RefreshesDirectiveOnPivot(t *testing.T) {
	tree := NewWorkTree()
	first, err := tree.EnsureGoal("s1", "你好")
	if err != nil {
		t.Fatal(err)
	}
	if first.Title != "你好" || first.Directive != "你好" {
		t.Fatalf("initial goal mismatch: title=%q directive=%q", first.Title, first.Directive)
	}
	child, err := tree.Create("s1", CreateWorkItemInput{
		ParentID: first.ID, Kind: WorkKindChecklist, Title: "step-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// User pivots to a new intent on a later turn.
	second, err := tree.EnsureGoal("s1", "请尝试多轮对话")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same goal ID, got first=%s second=%s", first.ID, second.ID)
	}
	if second.Title != "请尝试多轮对话" || second.Directive != "请尝试多轮对话" {
		t.Fatalf("goal not refreshed: title=%q directive=%q", second.Title, second.Directive)
	}
	// Child must remain under the same parent.
	if got := tree.ListChildren("s1", second.ID); len(got) != 1 || got[0].ID != child.ID {
		t.Fatalf("expected child to remain attached, got %#v", got)
	}
}

// TestEnsureGoal_TruncatesLongTitle mirrors the 80-char truncate logic so a
// long directive doesn't blow up the focus hint.
func TestEnsureGoal_TruncatesLongTitle(t *testing.T) {
	tree := NewWorkTree()
	long := ""
	for i := 0; i < 120; i++ {
		long += "x"
	}
	goal, err := tree.EnsureGoal("s1", long)
	if err != nil {
		t.Fatal(err)
	}
	if len(goal.Title) > 83 || !strings.HasSuffix(goal.Title, "...") {
		t.Fatalf("expected truncated title, got len=%d title=%q", len(goal.Title), goal.Title)
	}
	if goal.Directive != long {
		t.Fatalf("directive must keep full text, got %q", goal.Directive)
	}
}

// TestEnsureGoal_LockedGoalGetsFreshRoot verifies that a terminal
// (completed/failed/cancelled) goal is preserved as historical record and a
// new root goal is created above it. The original goal's children stay
// attached to the original parent — only a new sibling root is added.
func TestEnsureGoal_LockedGoalGetsFreshRoot(t *testing.T) {
	tree := NewWorkTree()
	first, err := tree.EnsureGoal("s1", "first goal")
	if err != nil {
		t.Fatal(err)
	}
	child, err := tree.Create("s1", CreateWorkItemInput{
		ParentID: first.ID, Kind: WorkKindImplement, Title: "child-of-first",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.UpdateStatus("s1", first.ID, TaskStatusInProgress); err != nil {
		t.Fatal(err)
	}
	if err := tree.UpdateStatus("s1", first.ID, TaskStatusCompleted); err != nil {
		t.Fatal(err)
	}
	// After locking, calling EnsureGoal with a new directive must mint a
	// fresh root instead of mutating the terminal goal.
	second, err := tree.EnsureGoal("s1", "second goal")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected new goal, got same ID %s", first.ID)
	}
	if second.ParentID != "" {
		t.Fatalf("expected root goal, got parent=%s", second.ParentID)
	}
	if second.Title != "second goal" || second.Directive != "second goal" {
		t.Fatalf("new goal not seeded: title=%q directive=%q", second.Title, second.Directive)
	}
	// Original child must still be under the original parent.
	if got := tree.ListChildren("s1", first.ID); len(got) != 1 || got[0].ID != child.ID {
		t.Fatalf("original child detached: %#v", got)
	}
	// Roots: original locked + new fresh.
	roots := tree.ListRoots("s1")
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
}
