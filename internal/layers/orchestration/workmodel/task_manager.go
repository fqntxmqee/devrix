package workmodel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/google/uuid"
)

// Task represents a single task item in the D7 work model.
// Legacy flat view; new code should use WorkItem via TaskManager.Tree().
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
// Internally delegates to WorkTree; Task methods remain for backward compatibility.
type TaskManager struct {
	tree      *WorkTree
	obsBridge *observability.Bridge

	// metrics is an optional counter sink for publishCompletion panics.
	// DM-20260621-010 PR-B: nil-safe; non-nil counters are incremented on
	// recover() in publishCompletion.
	metrics *TaskManagerMetrics

	// bus notifies background task completion to subscribers (e.g. task_output
	// tool, prepareTurn drainer). Optional; nil disables publishing.
	bus notify.Bus

	// registry tracks run observation handles (DM-011). Optional; nil
	// disables run tracking (SpawnForWorkItem becomes a no-op).
	registry *runregistry.Registry
}

// NewTaskManager creates a new in-memory task manager.
func NewTaskManager() *TaskManager {
	return &TaskManager{tree: NewWorkTree(), metrics: &TaskManagerMetrics{}}
}

// NewTaskManagerFromConfig creates a task manager with optional disk persistence.
func NewTaskManagerFromConfig(cfg config.TasksConfig, obsBridge *observability.Bridge) *TaskManager {
	m := NewTaskManager()
	m.obsBridge = obsBridge
	if cfg.Mode == "v2" && cfg.StoreDir != "" {
		store, err := NewDiskWorkItemStore(cfg.StoreDir)
		if err == nil {
			m.tree.SetStore(store)
		}
	}
	return m
}

// SetBus wires the notification bus. Optional; nil disables completion
// publishing. Returns the receiver for chaining. Bootstrap calls this once
// after constructing the TaskManager.
func (m *TaskManager) SetBus(bus notify.Bus) *TaskManager {
	m.bus = bus
	return m
}

// SetRegistry wires the run registry. Optional; nil disables run tracking.
// Returns the receiver for chaining. Bootstrap calls this once after
// constructing the TaskManager.
func (m *TaskManager) SetRegistry(reg *runregistry.Registry) *TaskManager {
	m.registry = reg
	return m
}

// Registry returns the run registry (nil if not wired). Nil-safe: returns
// nil if the receiver is nil.
func (m *TaskManager) Registry() *runregistry.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

// SetMetrics attaches a metrics sink. Safe to call before any publish.
// Passing nil disables metric recording. Returns the receiver for chaining.
func (m *TaskManager) SetMetrics(metrics *TaskManagerMetrics) *TaskManager {
	m.metrics = metrics
	return m
}

// Metrics returns the metrics sink (nil if not set).
func (m *TaskManager) Metrics() *TaskManagerMetrics {
	return m.metrics
}

// Tree returns the underlying work tree.
func (m *TaskManager) Tree() *WorkTree {
	return m.tree
}

// SetObservability wires the observability bridge.
func (m *TaskManager) SetObservability(obs *observability.Bridge) {
	m.obsBridge = obs
}

// SetStore wires optional disk persistence (legacy TaskStore adapter).
func (m *TaskManager) SetStore(store TaskStore) {
	if store == nil {
		m.tree.SetStore(nil)
		return
	}
	m.tree.SetStore(&taskStoreAdapter{store: store})
}

func (m *TaskManager) startSpan(operation string) (context.Context, tracer.Span) {
	if m.obsBridge == nil || !m.obsBridge.IsEnabled() {
		return nil, nil
	}
	ctx, span := m.obsBridge.Tracer().Start(context.Background(), operation)
	return ctx, span
}

// EnsureSession creates item map for session if not exists.
func (m *TaskManager) EnsureSession(sessionID string) {
	m.tree.EnsureSession(sessionID)
}

// EnsureGoal ensures session root goal exists.
func (m *TaskManager) EnsureGoal(sessionID, directive string) (*WorkItem, error) {
	return m.tree.EnsureGoal(sessionID, directive)
}

// Create creates a new implement work item under session goal.
//
// DM-20260620-003 (PR-C H3): returns (*Task, error) instead of silently
// swallowing tree.Create errors. Callers must handle the error; v1.x
// callers that ignore it via `task := m.Create(...)` should be updated
// to `task, err := m.Create(...)` and decide whether to log/return.
func (m *TaskManager) Create(sessionID, subject, description string) (*Task, error) {
	_, span := m.startSpan(telemetry.OpD7_S1_Task_Manager_Create)
	if span != nil {
		defer span.End()
	}

	goal, goalErr := m.tree.EnsureGoal(sessionID, subject)
	parentID := ""
	if goal != nil {
		parentID = goal.ID
	}

	item, err := m.tree.Create(sessionID, CreateWorkItemInput{
		ParentID:  parentID,
		Kind:      WorkKindImplement,
		Title:     subject,
		Directive: description,
	})
	if err != nil {
		if span != nil {
			span.SetAttributes(
				tracer.Attribute{Key: "task.create.error", Value: err.Error()},
				tracer.Attribute{Key: "task.ensure_goal.error", Value: goalErrString(goalErr)},
			)
		}
		if goalErr != nil {
			return nil, fmt.Errorf("taskmanager: ensure goal (session=%s): %w; create work item: %w", sessionID, goalErr, err)
		}
		return nil, fmt.Errorf("taskmanager: create work item (session=%s, subject=%q): %w", sessionID, subject, err)
	}

	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "task.id", Value: item.ID},
			tracer.Attribute{Key: "task.subject", Value: truncateSubject(subject, 200)},
		)
	}
	return item.ToTask(), nil
}

func goalErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// CreateWorkItem creates a work item with full control.
func (m *TaskManager) CreateWorkItem(sessionID string, in CreateWorkItemInput) (*WorkItem, error) {
	return m.tree.Create(sessionID, in)
}

// Get retrieves a task by ID.
func (m *TaskManager) Get(sessionID, taskID string) (*Task, bool) {
	item, ok := m.tree.Get(sessionID, taskID)
	if !ok {
		return nil, false
	}
	return item.ToTask(), true
}

// GetWorkItem retrieves a work item by ID.
func (m *TaskManager) GetWorkItem(sessionID, itemID string) (*WorkItem, bool) {
	return m.tree.Get(sessionID, itemID)
}

// List returns legacy task view items (excludes session goal and ephemeral checklist).
func (m *TaskManager) List(sessionID string) []*Task {
	items := m.tree.List(sessionID)
	out := make([]*Task, 0, len(items))
	for _, item := range items {
		if item == nil || item.Kind == WorkKindGoal {
			continue
		}
		if item.Kind == WorkKindChecklist && item.Ephemeral {
			continue
		}
		out = append(out, item.ToTask())
	}
	return out
}

// UpdateStatus updates task status.
func (m *TaskManager) UpdateStatus(sessionID, taskID string, status TaskStatus) error {
	_, span := m.startSpan(telemetry.OpD7_S1_Task_Manager_Update)
	if span != nil {
		defer span.End()
	}

	item, ok := m.tree.Get(sessionID, taskID)
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if err := m.tree.UpdateStatus(sessionID, taskID, status); err != nil {
		return err
	}

	if status == TaskStatusCompleted || status == TaskStatusFailed {
		task := item.ToTask()
		go m.publishCompletion(sessionID, taskID, task.Subject, status)
	}
	return nil
}

func (m *TaskManager) publishCompletion(sessionID, taskID, subject string, status TaskStatus) {
	// DM-20260621-010 PR-B: replace `_ = recover()` silent-swallow with
	// structured counter + slog.Error. m.bus.Publish can panic (e.g. nil-bus
	// misuse, panicking subscriber); we want to count and log.
	defer func() {
		if r := recover(); r != nil {
			if m.metrics != nil {
				m.metrics.PublishCompletionPanics.Add(1)
			}
			slog.Error("taskmanager: publishCompletion panic",
				"session", sessionID, "item_id", taskID, "panic", r,
				"metric", "publish_completion_panics")
		}
	}()
	if m.bus == nil {
		return
	}
	errStr := ""
	if status == TaskStatusFailed {
		errStr = "task failed"
	}
	m.bus.Publish(sessionID, notify.CompletionEvent{
		TaskID:  taskID,
		Kind:    "workmodel",
		Summary: fmt.Sprintf("%s → %s", subject, status),
		Error:   errStr,
		Time:    time.Now(),
	})
}

// SetOwner assigns owner to task.
func (m *TaskManager) SetOwner(sessionID, taskID, owner string) error {
	return m.tree.SetOwner(sessionID, taskID, owner)
}

// AddDependency adds a blocked-by dependency.
func (m *TaskManager) AddDependency(sessionID, taskID, blockedByID string) error {
	return m.tree.AddDependency(sessionID, taskID, blockedByID)
}

// RemoveTask removes a task.
func (m *TaskManager) RemoveTask(sessionID, taskID string) error {
	return m.tree.Remove(sessionID, taskID)
}

// GetReadyTasks returns tasks that are not blocked.
func (m *TaskManager) GetReadyTasks(sessionID string) []*Task {
	return TasksFromWorkItems(m.tree.GetReadyItems(sessionID))
}

// ClearSession removes all tasks for session.
func (m *TaskManager) ClearSession(sessionID string) {
	m.tree.ClearSession(sessionID)
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
