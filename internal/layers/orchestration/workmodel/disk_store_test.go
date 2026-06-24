package workmodel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
)

// T: D2-S10-A01-T38
func TestTaskManager_disk_persist_and_list_consistent(t *testing.T) {
	dir := t.TempDir()
	cfg := config.TasksConfig{Mode: "v2", StoreDir: dir}
	m1 := workmodel.NewTaskManagerFromConfig(cfg, nil)
	created, err := m1.Tree().Create("sess_disk", workmodel.CreateWorkItemInput{
		Kind:      workmodel.WorkKindImplement,
		Title:     "Implement QueryLoop",
		Directive: "Wire loop into engine",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil || created.ID == "" {
		t.Fatal("expected created task")
	}

	m2 := workmodel.NewTaskManagerFromConfig(cfg, nil)
	list := m2.Tree().List("sess_disk")
	if len(list) == 0 {
		t.Fatal("expected at least one item from disk")
	}
	var found *workmodel.WorkItem
	for _, item := range list {
		if item == nil || item.Kind == workmodel.WorkKindGoal {
			continue
		}
		if item.ID == created.ID {
			found = item
			break
		}
	}
	if found == nil {
		t.Fatalf("expected created item in disk reload, got %d items", len(list))
	}
	if found.Title != "Implement QueryLoop" {
		t.Fatalf("unexpected title: %s", found.Title)
	}

	storePath := filepath.Join(dir, "sess_disk.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected store file: %v", err)
	}
}
