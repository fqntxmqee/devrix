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
	created := m1.Create("sess_disk", "Implement QueryLoop", "Wire loop into engine")
	if created == nil || created.ID == "" {
		t.Fatal("expected created task")
	}

	m2 := workmodel.NewTaskManagerFromConfig(cfg, nil)
	list := m2.List("sess_disk")
	if len(list) != 1 {
		t.Fatalf("expected 1 task from disk, got %d", len(list))
	}
	if list[0].Subject != "Implement QueryLoop" {
		t.Fatalf("unexpected subject: %s", list[0].Subject)
	}

	storePath := filepath.Join(dir, "sess_disk.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected store file: %v", err)
	}
}
