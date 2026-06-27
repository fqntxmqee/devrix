//go:build integration && d7

package d7integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D7-S1-T01, D7-S1-T03
func TestIntegration_D7WorkModel_DiskPersistAndReload(t *testing.T) {
	stack := testutil.NewD7TestStack(t, testutil.D7StackOptions{})

	created, err := stack.TaskManager.Tree().Create("sess_wm", workmodel.CreateWorkItemInput{
		Kind:      workmodel.WorkKindImplement,
		Title:     "Wire D7 integration tests",
		Directive: "Add P0 vertical slices",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil || created.ID == "" {
		t.Fatal("expected created task")
	}

	storePath := filepath.Join(stack.WorkDir, "sess_wm.json")
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected disk store file: %v", err)
	}

	reloaded := workmodel.NewTaskManagerFromConfig(config.TasksConfig{
		Mode:     "v2",
		StoreDir: stack.WorkDir,
	}, stack.ObsBridge)
	list := reloaded.Tree().List("sess_wm")
	if len(list) == 0 {
		t.Fatalf("expected items after reload, got 0")
	}
	var found *workmodel.WorkItem
	for _, item := range list {
		if item != nil && item.ID == created.ID {
			found = item
			break
		}
	}
	if found == nil {
		t.Fatalf("expected created item after reload, got %d items", len(list))
	}
	if found.Title != "Wire D7 integration tests" {
		t.Fatalf("unexpected title: %q", found.Title)
	}
}

// T: D7-S5-T02
func TestIntegration_D7PlanMode_ReadOnlyWhitelistInvariant(t *testing.T) {
	agent := &workmodel.PlanAgent{}
	for _, banned := range []string{"write", "edit", "bash", "delete"} {
		if agent.IsReadOnlyTool(banned) {
			t.Fatalf("forbidden tool %q must not be read-only allowed", banned)
		}
	}
	for _, allowed := range workmodel.PlanAgentReadOnlyTools {
		if !agent.IsReadOnlyTool(allowed) {
			t.Fatalf("whitelist tool %q must be read-only allowed", allowed)
		}
	}
}
