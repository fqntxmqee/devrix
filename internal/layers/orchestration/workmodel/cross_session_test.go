package workmodel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/shared/config"
)

func TestDiskWorkItemStore_FindByItemID(t *testing.T) {
	dir := t.TempDir()
	store, err := NewDiskWorkItemStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	item := NewWorkItem(WorkKindImplement, "cross", "find me")
	if err := store.Save("sess-a", []*WorkItem{item}); err != nil {
		t.Fatal(err)
	}
	found, sid, ok := store.FindByItemID(item.ID)
	if !ok || sid != "sess-a" || found.ID != item.ID {
		t.Fatalf("FindByItemID = %+v %q %v", found, sid, ok)
	}
}

func TestTaskManager_InheritFromSession(t *testing.T) {
	tm := NewTaskManager()
	goal, _ := tm.EnsureGoal("src", "goal")
	src, _ := tm.CreateWorkItem("src", CreateWorkItemInput{ParentID: goal.ID, Kind: WorkKindImplement, Title: "old", Directive: "continue"})
	child, err := tm.InheritFromSession("src", "dst", src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if child.SourceSession != "src" {
		t.Fatalf("source_session = %q, want src", child.SourceSession)
	}
}

func TestTaskManager_QueryHistoricalWorkItem(t *testing.T) {
	dir := t.TempDir()
	tm := NewTaskManagerFromConfig(config.TasksConfig{Mode: "v2", StoreDir: dir}, nil)
	item, err := tm.Tree().Create("hist", CreateWorkItemInput{
		Kind:      WorkKindImplement,
		Title:     "subject",
		Directive: "desc",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	path := filepath.Join(dir, "hist.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted file: %v", err)
	}
	tm2 := NewTaskManagerFromConfig(config.TasksConfig{Mode: "v2", StoreDir: dir}, nil)
	got, sid, err := tm2.QueryHistoricalWorkItem(item.ID)
	if err != nil || sid != "hist" || got.ID != item.ID {
		t.Fatalf("QueryHistoricalWorkItem = %+v %q err=%v", got, sid, err)
	}
}
