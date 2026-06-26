package workmodel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify"
	"github.com/devrix/devrix/internal/shared/config"
)

// TaskManager is the canonical D7 work-item manager. It owns a WorkTree
// (the unified hierarchical model) and provides narrow side effects
// (notification bus, run registry, metrics) wired at construction.
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
	registry *Registry

	// adaptiveThreshold supplies per-user decompose gates (TD-WT-01).
	adaptiveThreshold *AdaptiveThreshold

	// parentReevalMu deduplicates concurrent parent re-evaluation (TD-WT-06).
	parentReevalMu sync.Map
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
func (m *TaskManager) SetRegistry(reg *Registry) *TaskManager {
	m.registry = reg
	return m
}

// SetAdaptiveThreshold wires self-evolving decompose thresholds (TD-WT-01).
func (m *TaskManager) SetAdaptiveThreshold(a *AdaptiveThreshold) *TaskManager {
	if m != nil {
		m.adaptiveThreshold = a
	}
	return m
}

// AdaptiveThreshold returns the wired threshold state (may be nil).
func (m *TaskManager) AdaptiveThreshold() *AdaptiveThreshold {
	if m == nil {
		return nil
	}
	return m.adaptiveThreshold
}

func (m *TaskManager) parentReevalLock(sessionID, parentID string) *sync.Mutex {
	key := sessionID + "|" + parentID
	v, _ := m.parentReevalMu.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// Registry returns the run registry (nil if not wired). Nil-safe: returns
// nil if the receiver is nil.
func (m *TaskManager) Registry() *Registry {
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

// CreateWorkItem creates a work item with full control.
func (m *TaskManager) CreateWorkItem(sessionID string, in CreateWorkItemInput) (*WorkItem, error) {
	item, err := m.tree.Create(sessionID, in)
	if err != nil {
		return nil, err
	}
	if FeatureWorkItemContextGraphEnabled() {
		m.EnsureContextScope(sessionID, item)
	}
	_, span := m.startSpan(telemetry.OpD7_S1_Task_Manager_Create)
	if span != nil {
		span.SetAttributes(
			tracer.Attribute{Key: "task.id", Value: item.ID},
			tracer.Attribute{Key: "task.subject", Value: truncateSubject(in.Title, 200)},
		)
		span.End()
	}
	return item, nil
}

// GetWorkItem retrieves a work item by ID.
func (m *TaskManager) GetWorkItem(sessionID, itemID string) (*WorkItem, bool) {
	return m.tree.Get(sessionID, itemID)
}

// UpdateStatus updates a work-item status and, on terminal transitions,
// publishes a completion event to the notification bus.
func (m *TaskManager) UpdateStatus(sessionID, itemID string, status TaskStatus) error {
	_, span := m.startSpan(telemetry.OpD7_S1_Task_Manager_Update)
	if span != nil {
		defer span.End()
	}

	item, ok := m.tree.Get(sessionID, itemID)
	if !ok {
		return fmt.Errorf("task not found: %s", itemID)
	}

	if err := m.tree.UpdateStatus(sessionID, itemID, status); err != nil {
		return err
	}

	if status == TaskStatusCompleted || status == TaskStatusFailed {
		go m.publishCompletion(sessionID, itemID, item.Title, status)
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
