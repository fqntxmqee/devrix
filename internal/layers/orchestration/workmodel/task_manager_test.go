package workmodel

import (
	"errors"
	"testing"
)

func TestTaskManager_Create(t *testing.T) {
	m := NewTaskManager()
	item, err := m.Tree().Create("session1", CreateWorkItemInput{
		Kind:      WorkKindImplement,
		Title:     "Fix bug",
		Directive: "Fix authentication bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.ID == "" {
		t.Error("expected item ID")
	}
	if item.Title != "Fix bug" {
		t.Errorf("expected title 'Fix bug', got %s", item.Title)
	}
	if item.Status != TaskStatusPending {
		t.Errorf("expected status pending, got %s", item.Status)
	}
}

func TestTaskManager_List(t *testing.T) {
	m := NewTaskManager()
	if _, err := m.Tree().Create("session1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task 1"}); err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	if _, err := m.Tree().Create("session1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task 2"}); err != nil {
		t.Fatalf("Create #2: %v", err)
	}

	items := m.Tree().List("session1")
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestTaskManager_UpdateStatus(t *testing.T) {
	m := NewTaskManager()
	item, err := m.Tree().Create("session1", CreateWorkItemInput{
		Kind:  WorkKindImplement,
		Title: "Task",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.UpdateStatus("session1", item.ID, TaskStatusInProgress); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	updated, _ := m.Tree().Get("session1", item.ID)
	if updated.Status != TaskStatusInProgress {
		t.Errorf("expected in_progress, got %s", updated.Status)
	}
}

func TestTaskManager_Dependency(t *testing.T) {
	m := NewTaskManager()
	item1, err := m.Tree().Create("session1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task 1"})
	if err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	item2, err := m.Tree().Create("session1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task 2"})
	if err != nil {
		t.Fatalf("Create #2: %v", err)
	}

	if err := m.Tree().AddDependency("session1", item2.ID, item1.ID); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	ready := m.Tree().GetReadyItems("session1")
	found := false
	for _, readyItem := range ready {
		if readyItem.ID == item1.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("item1 should be ready")
	}
}

func TestTaskManager_ClearSession(t *testing.T) {
	m := NewTaskManager()
	if _, err := m.Tree().Create("session1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task"}); err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	if _, err := m.Tree().Create("session1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task 2"}); err != nil {
		t.Fatalf("Create #2: %v", err)
	}

	m.Tree().ClearSession("session1")
	items := m.Tree().List("session1")
	if len(items) != 0 {
		t.Errorf("expected 0 items after clear, got %d", len(items))
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
	item, err := m.Tree().Create("s1", CreateWorkItemInput{Kind: WorkKindImplement, Title: "Task"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// pending → in_progress (legal)
	if err := m.UpdateStatus("s1", item.ID, TaskStatusInProgress); err != nil {
		t.Fatalf("legal transition failed: %v", err)
	}
	// in_progress → completed (legal)
	if err := m.UpdateStatus("s1", item.ID, TaskStatusCompleted); err != nil {
		t.Fatalf("legal transition failed: %v", err)
	}

	// completed → pending (illegal)
	err = m.UpdateStatus("s1", item.ID, TaskStatusPending)
	if err == nil {
		t.Fatal("expected error for completed→pending, got nil")
	}
	if !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("expected ErrIllegalTransition, got %v", err)
	}

	// Verify status unchanged
	updated, _ := m.Tree().Get("s1", item.ID)
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
			item, err := m.Tree().Create("s1", CreateWorkItemInput{
				Kind:  WorkKindImplement,
				Title: p.name,
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			for _, target := range p.path {
				if err := m.UpdateStatus("s1", item.ID, target); err != nil {
					t.Fatalf("legal transition to %s failed: %v", target, err)
				}
			}
		})
	}
}
