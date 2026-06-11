package contextengine

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
)

// Covers: L5-4-10-07
func TestResolveDelegateTaskID_should_create_task_when_missing(t *testing.T) {
	tm := tasks.NewTaskManager()
	tasks.GlobalTaskManager = tm

	id := resolveDelegateTaskID("sess1", "", "explore auth module")
	if id == "" {
		t.Fatal("expected task id")
	}
	got, ok := tm.Get("sess1", id)
	if !ok {
		t.Fatal("task not found")
	}
	if got.Subject == "" {
		t.Fatalf("task = %+v", got)
	}
}

func TestResolveDelegateTaskID_should_keep_explicit_id(t *testing.T) {
	id := resolveDelegateTaskID("sess1", "task_explicit", "ignored")
	if id != "task_explicit" {
		t.Fatalf("id = %q", id)
	}
}
