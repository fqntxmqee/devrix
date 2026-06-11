package tasks

import (
	"testing"
)

func TestTaskManager_Create(t *testing.T) {
	m := NewTaskManager()
	task := m.Create("session1", "Fix bug", "Fix authentication bug")

	if task.ID == "" {
		t.Error("expected task ID")
	}
	if task.Subject != "Fix bug" {
		t.Errorf("expected subject 'Fix bug', got %s", task.Subject)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("expected status pending, got %s", task.Status)
	}
}

func TestTaskManager_List(t *testing.T) {
	m := NewTaskManager()
	m.Create("session1", "Task 1", "Desc 1")
	m.Create("session1", "Task 2", "Desc 2")

	tasks := m.List("session1")
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTaskManager_UpdateStatus(t *testing.T) {
	m := NewTaskManager()
	task := m.Create("session1", "Task", "Desc")

	err := m.UpdateStatus("session1", task.ID, TaskStatusInProgress)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	updated, _ := m.Get("session1", task.ID)
	if updated.Status != TaskStatusInProgress {
		t.Errorf("expected in_progress, got %s", updated.Status)
	}
}

func TestTaskManager_Dependency(t *testing.T) {
	m := NewTaskManager()
	task1 := m.Create("session1", "Task 1", "First task")
	task2 := m.Create("session1", "Task 2", "Second task")

	err := m.AddDependency("session1", task2.ID, task1.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Get ready tasks - task1 should be ready, task2 should not
	ready := m.GetReadyTasks("session1")
	found := false
	for _, readyTask := range ready {
		if readyTask.ID == task1.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("task1 should be ready")
	}
}

func TestTaskManager_ClearSession(t *testing.T) {
	m := NewTaskManager()
	m.Create("session1", "Task", "Desc")
	m.Create("session1", "Task 2", "Desc 2")

	m.ClearSession("session1")
	tasks := m.List("session1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after clear, got %d", len(tasks))
	}
}
