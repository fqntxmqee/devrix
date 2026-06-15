package workmodel

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/config"
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
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
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

// TaskManager manages task lists for sessions in D7-S1.
type TaskManager struct {
	mu        sync.RWMutex
	tasks     map[string]map[string]*Task // sessionID -> taskID -> Task
	store     TaskStore
	obsBridge *observability.Bridge
}

// GlobalTaskManager is the process-wide task manager singleton.
var GlobalTaskManager *TaskManager

func init() {
	GlobalTaskManager = NewTaskManager()
}

// InitGlobalTaskManager reconfigures the singleton from config (disk when mode=v2).
func InitGlobalTaskManager(cfg config.TasksConfig, obsBridge *observability.Bridge) {
	GlobalTaskManager = NewTaskManagerFromConfig(cfg, obsBridge)
}

// NewTaskManager creates a new in-memory task manager.
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]map[string]*Task),
	}
}

// NewTaskManagerFromConfig creates a task manager with optional disk persistence.
func NewTaskManagerFromConfig(cfg config.TasksConfig, obsBridge *observability.Bridge) *TaskManager {
	m := NewTaskManager()
	m.obsBridge = obsBridge
	if cfg.Mode == "v2" && cfg.StoreDir != "" {
		store, err := NewDiskTaskStore(cfg.StoreDir)
		if err == nil {
			m.store = store
		}
	}
	return m
}

// SetObservability wires the observability bridge.
func (m *TaskManager) SetObservability(obs *observability.Bridge) {
	m.obsBridge = obs
}

// SetStore wires optional disk persistence.
func (m *TaskManager) SetStore(store TaskStore) {
	m.store = store
}

func (m *TaskManager) startSpan(operation string) (context.Context, tracer.Span) {
	if m.obsBridge == nil || !m.obsBridge.IsEnabled() {
		return nil, nil
	}
	ctx, span := m.obsBridge.Tracer().Start(context.Background(), operation)
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
	if m.store == nil {
		return
	}
	loaded, err := m.store.Load(sessionID)
	if err != nil || len(loaded) == 0 {
		return
	}
	for _, t := range loaded {
		if t != nil {
			m.tasks[sessionID][t.ID] = t
		}
	}
}

func (m *TaskManager) persistLocked(sessionID string) {
	if m.store == nil {
		return
	}
	tasks := make([]*Task, 0, len(m.tasks[sessionID]))
	for _, t := range m.tasks[sessionID] {
		tasks = append(tasks, t)
	}
	_ = m.store.Save(sessionID, tasks)
}

// Create creates a new task in session.
func (m *TaskManager) Create(sessionID, subject, description string) *Task {
	_, span := m.startSpan(telemetry.OpD7_S1_Task_Manager_Create)
	if span != nil {
		defer span.End()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	task := NewTask(subject, description)
	m.tasks[sessionID][task.ID] = task
	m.persistLocked(sessionID)

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "task.id", Value: task.ID},
			tracer.Attribute{Key: "task.subject", Value: truncateSubject(subject, 200)},
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
	_, span := m.startSpan(telemetry.OpD7_S1_Task_Manager_Update)
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
	m.persistLocked(sessionID)
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
	m.persistLocked(sessionID)
	return nil
}

// AddDependency adds a blocked-by dependency.
func (m *TaskManager) AddDependency(sessionID, taskID, blockedByID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	task, ok := m.tasks[sessionID][taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	task.BlockedBy = append(task.BlockedBy, blockedByID)

	if blocker, ok := m.tasks[sessionID][blockedByID]; ok {
		blocker.Blocks = append(blocker.Blocks, taskID)
	}

	task.UpdatedAt = time.Now()
	m.persistLocked(sessionID)
	return nil
}

// RemoveTask removes a task.
func (m *TaskManager) RemoveTask(sessionID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureSessionLocked(sessionID)

	_, ok := m.tasks[sessionID][taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	for _, t := range m.tasks[sessionID] {
		for i, blocked := range t.BlockedBy {
			if blocked == taskID {
				t.BlockedBy = append(t.BlockedBy[:i], t.BlockedBy[i+1:]...)
				break
			}
		}
	}

	delete(m.tasks[sessionID], taskID)
	m.persistLocked(sessionID)
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
	if store, ok := m.store.(*DiskTaskStore); ok && store != nil {
		_ = os.Remove(store.path(sessionID))
	}
}

// FormatTaskSummary returns a formatted summary string.
func FormatTaskSummary(completed, inProgress, pending, failed int) string {
	return fmt.Sprintf("✓%d ●%d ○%d ✗%d", completed, inProgress, pending, failed)
}

func truncateSubject(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
