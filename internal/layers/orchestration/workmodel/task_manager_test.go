package workmodel

import (
	"errors"
	"testing"
)

func TestTaskManager_Create(t *testing.T) {
	m := NewTaskManager()
	task, err := m.Create("session1", "Fix bug", "Fix authentication bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task == nil {
		t.Fatal("expected non-nil task")
	}
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
	if _, err := m.Create("session1", "Task 1", "Desc 1"); err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	if _, err := m.Create("session1", "Task 2", "Desc 2"); err != nil {
		t.Fatalf("Create #2: %v", err)
	}

	tasks := m.List("session1")
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTaskManager_UpdateStatus(t *testing.T) {
	m := NewTaskManager()
	task, err := m.Create("session1", "Task", "Desc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.UpdateStatus("session1", task.ID, TaskStatusInProgress); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	updated, _ := m.Get("session1", task.ID)
	if updated.Status != TaskStatusInProgress {
		t.Errorf("expected in_progress, got %s", updated.Status)
	}
}

func TestTaskManager_Dependency(t *testing.T) {
	m := NewTaskManager()
	task1, err := m.Create("session1", "Task 1", "First task")
	if err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	task2, err := m.Create("session1", "Task 2", "Second task")
	if err != nil {
		t.Fatalf("Create #2: %v", err)
	}

	if err := m.AddDependency("session1", task2.ID, task1.ID); err != nil {
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
	if _, err := m.Create("session1", "Task", "Desc"); err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	if _, err := m.Create("session1", "Task 2", "Desc 2"); err != nil {
		t.Fatalf("Create #2: %v", err)
	}

	m.ClearSession("session1")
	tasks := m.List("session1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after clear, got %d", len(tasks))
	}
}

// D7-S1-T08: Task 非法状态转换拒绝 (v1.2).

func TestIsLegalTransition(t *testing.T) {
	tests := []struct {
		from, to TaskStatus
		want     bool
	}{
		// Legal transitions
		{TaskStatusPending, TaskStatusInProgress, true},
		{TaskStatusPending, TaskStatusCancelled, true},
		{TaskStatusPending, TaskStatusPending, true}, // same-state idempotent
		{TaskStatusInProgress, TaskStatusCompleted, true},
		{TaskStatusInProgress, TaskStatusFailed, true},
		{TaskStatusInProgress, TaskStatusCancelled, true},
		{TaskStatusInProgress, TaskStatusInProgress, true},

		// Illegal: terminal → anything
		{TaskStatusCompleted, TaskStatusPending, false},
		{TaskStatusCompleted, TaskStatusInProgress, false},
		{TaskStatusCompleted, TaskStatusFailed, false},
		{TaskStatusCompleted, TaskStatusCancelled, false},
		{TaskStatusFailed, TaskStatusPending, false},
		{TaskStatusFailed, TaskStatusInProgress, false},
		{TaskStatusFailed, TaskStatusCompleted, false},
		{TaskStatusFailed, TaskStatusCancelled, false},
		{TaskStatusCancelled, TaskStatusPending, false},
		{TaskStatusCancelled, TaskStatusInProgress, false},
		{TaskStatusCancelled, TaskStatusCompleted, false},
		{TaskStatusCancelled, TaskStatusFailed, false},

		// Illegal: backward transitions
		{TaskStatusInProgress, TaskStatusPending, false},

		// Illegal: skip in_progress
		{TaskStatusPending, TaskStatusCompleted, false},
		{TaskStatusPending, TaskStatusFailed, false},
	}
	for _, tt := range tests {
		got := IsLegalTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("IsLegalTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestTaskManager_UpdateStatus_IllegalTransition(t *testing.T) {
	m := NewTaskManager()
	task, err := m.Create("s1", "Task", "Desc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// pending → in_progress (legal)
	if err := m.UpdateStatus("s1", task.ID, TaskStatusInProgress); err != nil {
		t.Fatalf("legal transition failed: %v", err)
	}
	// in_progress → completed (legal)
	if err := m.UpdateStatus("s1", task.ID, TaskStatusCompleted); err != nil {
		t.Fatalf("legal transition failed: %v", err)
	}

	// completed → pending (illegal)
	err = m.UpdateStatus("s1", task.ID, TaskStatusPending)
	if err == nil {
		t.Fatal("expected error for completed→pending, got nil")
	}
	if !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("expected ErrIllegalTransition, got %v", err)
	}

	// Verify status unchanged
	updated, _ := m.Get("s1", task.ID)
	if updated.Status != TaskStatusCompleted {
		t.Errorf("status should be unchanged (completed), got %s", updated.Status)
	}
}

func TestTaskManager_UpdateStatus_LegalTransitions(t *testing.T) {
	paths := []struct {
		name string
		path []TaskStatus
	}{
		{"pending→in_progress→completed", []TaskStatus{TaskStatusInProgress, TaskStatusCompleted}},
		{"pending→in_progress→failed", []TaskStatus{TaskStatusInProgress, TaskStatusFailed}},
		{"pending→in_progress→cancelled", []TaskStatus{TaskStatusInProgress, TaskStatusCancelled}},
		{"pending→cancelled", []TaskStatus{TaskStatusCancelled}},
	}

	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			m := NewTaskManager()
			task, err := m.Create("s1", "Task", p.name)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			for _, target := range p.path {
				if err := m.UpdateStatus("s1", task.ID, target); err != nil {
					t.Fatalf("legal transition to %s failed: %v", target, err)
				}
			}
		})
	}
}
