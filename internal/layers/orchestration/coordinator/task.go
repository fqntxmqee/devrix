package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/google/uuid"
)

// Task represents a single task item in the D7 work model.
type Task struct {
	ID          string     `json:"id"`
	Subject     string     `json:"subject"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Owner       string     `json:"owner,omitempty"`
	BlockedBy   []string   `json:"blocked_by,omitempty"`
	Blocks      []string   `json:"blocks,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewTask creates a new task with generated ID.
func NewTask(subject, description string) *Task {
	now := time.Now()
	return &Task{
		ID:          "task_" + uuid.New().String()[:8],
		Subject:     subject,
		Description: description,
		Status:      TaskStatusPending,
		BlockedBy:   []string{},
		Blocks:      []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TaskManager manages task lists for sessions in D7.
type TaskManager struct {
	mu        sync.RWMutex
	tasks     map[string]map[string]*Task // sessionID -> taskID -> Task
	obsBridge *observability.Bridge
}

// NewTaskManager creates a new in-memory task manager for D7.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]map[string]*Task),
	}
}

// SetObservability wires the observability bridge.
func (m *TaskManager) SetObservability(obs *observability.Bridge) {
	m.obsBridge = obs
}

// startSpan creates a child span for TaskManager operations.
func (m *TaskManager) startSpan(operation string) (context.Context, tracer.Span) {
	if m.obsBridge == nil || !m.obsBridge.IsEnabled() {
		return nil, nil
	}
	ctx, span := m.obsBridge.Tracer().Start(nil, operation)
	return ctx, span
}

// EnsureSession creates task map for session if not exists.
func (m *TaskManager) EnsureSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)
}

func (m *TaskManager) ensureSessionLocked(sessionID string) {
	if m.tasks[sessionID] != nil {
		return
	}
	m.tasks[sessionID] = make(map[string]*Task)
}

// Create creates a new task in session.
func (m *TaskManager) Create(sessionID, subject, description string) *Task {
	_, span := m.startSpan(telemetry.OpTaskManagerCreate)
	if span != nil {
		defer span.End()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	task := NewTask(subject, description)
	m.tasks[sessionID][task.ID] = task

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "task.id", Value: task.ID},
			tracer.Attribute{Key: "task.subject", Value: truncate(subject, 200)},
		)
	}
	return task
}

// Get retrieves a task by ID.
func (m *TaskManager) Get(sessionID, taskID string) (*Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)
	task, ok := m.tasks[sessionID][taskID]
	return task, ok
}

// List returns all tasks for session.
func (m *TaskManager) List(sessionID string) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	tasks := make([]*Task, 0, len(m.tasks[sessionID]))
	for _, t := range m.tasks[sessionID] {
		tasks = append(tasks, t)
	}
	return tasks
}

// UpdateStatus updates task status.
func (m *TaskManager) UpdateStatus(sessionID, taskID string, status TaskStatus) error {
	_, span := m.startSpan(telemetry.OpTaskManagerUpdate)
	if span != nil {
		defer span.End()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	task, ok := m.tasks[sessionID][taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	return nil
}

// SetOwner assigns owner to task.
func (m *TaskManager) SetOwner(sessionID, taskID, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	task, ok := m.tasks[sessionID][taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Owner = owner
	task.UpdatedAt = time.Now()
	return nil
}

// AddDependency adds a blocked-by dependency.
func (m *TaskManager) AddDependency(sessionID, taskID, blockedByID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[sessionID][taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.BlockedBy = append(task.BlockedBy, blockedByID)

	if blocker, ok := m.tasks[sessionID][blockedByID]; ok {
		blocker.Blocks = append(blocker.Blocks, taskID)
	}

	task.UpdatedAt = time.Now()
	return nil
}

// GetReadyTasks returns tasks that are not blocked.
func (m *TaskManager) GetReadyTasks(sessionID string) []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	var ready []*Task
	for _, t := range m.tasks[sessionID] {
		if t.Status != TaskStatusPending {
			continue
		}
		allBlocked := true
		for _, blockerID := range t.BlockedBy {
			if blocker, ok := m.tasks[sessionID][blockerID]; !ok || blocker.Status != TaskStatusCompleted {
				allBlocked = false
				break
			}
		}
		if allBlocked || len(t.BlockedBy) == 0 {
			ready = append(ready, t)
		}
	}
	return ready
}

// ClearSession removes all tasks for session.
func (m *TaskManager) ClearSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, sessionID)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
