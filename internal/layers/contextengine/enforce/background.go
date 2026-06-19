package enforce

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/google/uuid"
)

// BackgroundTask tracks an async SubQuery run.
type BackgroundTask struct {
	ID        string
	SessionID string
	AgentID   string
	AgentName string
	Status    string
	Result    string
	Error     string
	StartedAt time.Time
	EndedAt   time.Time
	cancel    context.CancelFunc
	done      chan struct{}
}

// BackgroundRegistry tracks in-flight and completed background tasks.
type BackgroundRegistry struct {
	mu    sync.Mutex
	tasks map[string]*BackgroundTask
}

// GlobalBackgroundRegistry is the process-wide singleton used by the
// task_stop / task_output LLM tools. It is set by the engine bootstrap
// and read by tool runners that do not have a ContextEngine reference.
var GlobalBackgroundRegistry *BackgroundRegistry

// NewBackgroundRegistry creates a registry.
func NewBackgroundRegistry() *BackgroundRegistry {
	return &BackgroundRegistry{tasks: make(map[string]*BackgroundTask)}
}

// SetGlobalBackgroundRegistry creates a registry and installs it as the
// process-wide singleton consumed by the task_stop / task_output LLM
// tools. Returns the new registry for callers to wire into the query loop.
func SetGlobalBackgroundRegistry() *BackgroundRegistry {
	GlobalBackgroundRegistry = NewBackgroundRegistry()
	return GlobalBackgroundRegistry
}

// Register adds a running task and returns its id.
func (r *BackgroundRegistry) Register(sessionID, agentID, agentName string) string {
	handle, _ := r.RegisterWithCancel(sessionID, agentID, agentName, agentID)
	return handle.ID
}

// CancelHandle exposes the generated task id and a per-task cancel func.
type CancelHandle struct {
	ID     string
	Cancel context.CancelFunc
}

// RegisterWithCancel stores a new running task along with its cancel func.
func (r *BackgroundRegistry) RegisterWithCancel(sessionID, agentID, agentName, _ string) (CancelHandle, context.CancelFunc) {
	if r == nil {
		return CancelHandle{}, func() {}
	}
	id := "bg_" + uuid.New().String()[:8]
	task := &BackgroundTask{
		ID:        id,
		SessionID: sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		Status:    "running",
		StartedAt: time.Now(),
		done:      make(chan struct{}),
	}
	r.mu.Lock()
	r.tasks[id] = task
	r.mu.Unlock()
	return CancelHandle{ID: id, Cancel: func() {}}, task.cancel
}

// SetTaskCancel attaches the cancel func to the task after registration.
func (r *BackgroundRegistry) SetTaskCancel(taskID string, cancel context.CancelFunc) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[taskID]; ok {
		t.cancel = cancel
	}
}

// IsTerminal reports whether the task has reached a terminal state.
func (r *BackgroundRegistry) IsTerminal(taskID string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[taskID]
	if !ok {
		return false
	}
	return t.Status != "running"
}

// Cancel cancels a running task. Idempotent.
func (r *BackgroundRegistry) Cancel(taskID string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	task, ok := r.tasks[taskID]
	if !ok {
		r.mu.Unlock()
		return false
	}
	if task.Status != "running" {
		r.mu.Unlock()
		return false
	}
	task.Status = "cancelled"
	task.EndedAt = time.Now()
	cancel := task.cancel
	done := task.done
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		close(done)
	}
	return true
}

// Complete marks a task finished and enqueues a task-notification.
func (r *BackgroundRegistry) Complete(taskID, result, errMsg string, q contracts.SessionCommandQueue) {
	if r == nil {
		return
	}
	r.mu.Lock()
	task, ok := r.tasks[taskID]
	if !ok {
		r.mu.Unlock()
		return
	}
	if task.Status == "cancelled" {
		done := task.done
		r.mu.Unlock()
		if done != nil {
			close(done)
		}
		return
	}
	task.Status = "completed"
	if errMsg != "" {
		task.Status = "failed"
		task.Error = errMsg
	}
	task.Result = result
	task.EndedAt = time.Now()
	done := task.done
	r.mu.Unlock()

	if done != nil {
		close(done)
	}

	if q != nil {
		body := result
		if errMsg != "" {
			body = fmt.Sprintf("Background task %s failed: %s", taskID, errMsg)
		} else {
			body = fmt.Sprintf("Background task %s (%s) completed: %s", taskID, task.AgentName, result)
		}
		q.Enqueue(task.SessionID, contracts.QueuedCommand{
			Value:   body,
			Mode:    contracts.ModeTaskNotification,
			AgentID: task.AgentID,
		})
	}
}

// Get returns a task by id.
func (r *BackgroundRegistry) Get(taskID string) (*BackgroundTask, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[taskID]
	return t, ok
}

// List returns a snapshot of tasks for a given session.
func (r *BackgroundRegistry) List(sessionID string) []*BackgroundTask {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*BackgroundTask, 0, len(r.tasks))
	for _, t := range r.tasks {
		if t.SessionID == sessionID {
			cp := *t
			out = append(out, &cp)
		}
	}
	return out
}

// BackgroundWaiter blocks on background-task terminal transitions.
type BackgroundWaiter struct {
	reg  *BackgroundRegistry
	mu   sync.Mutex
	done map[string]chan struct{}
}

// NewBackgroundWaiter creates a waiter bound to a registry.
func NewBackgroundWaiter(reg *BackgroundRegistry) *BackgroundWaiter {
	return &BackgroundWaiter{reg: reg, done: make(map[string]chan struct{})}
}

// Register binds a waiter to a specific task id.
func (w *BackgroundWaiter) Register(taskID string) {
	if w == nil || taskID == "" {
		return
	}
	task, ok := w.reg.Get(taskID)
	if !ok || task.done == nil {
		return
	}
	w.mu.Lock()
	w.done[taskID] = task.done
	w.mu.Unlock()
}

// Wait blocks until taskID reaches a terminal state or timeout.
func (w *BackgroundWaiter) Wait(taskID string, timeout time.Duration) (*BackgroundTask, bool) {
	if w == nil {
		return nil, false
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 600*time.Second {
		timeout = 600 * time.Second
	}
	w.mu.Lock()
	ch := w.done[taskID]
	w.mu.Unlock()

	if ch == nil {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if task, ok := w.reg.Get(taskID); ok && task.Status != "running" {
				return task, true
			}
			time.Sleep(20 * time.Millisecond)
		}
		task, _ := w.reg.Get(taskID)
		return task, task != nil && task.Status != "running"
	}

	select {
	case <-ch:
		task, _ := w.reg.Get(taskID)
		return task, true
	case <-time.After(timeout):
		task, _ := w.reg.Get(taskID)
		return task, task != nil && task.Status != "running"
	}
}

// RunBackground starts SubQuery asynchronously and registers notification on completion.
func RunBackground(ctx context.Context, deps SubQueryDeps, params SubQueryParams, reg *BackgroundRegistry, q contracts.SessionCommandQueue) (string, error) {
	if reg == nil {
		return "", fmt.Errorf("background registry is nil")
	}
	taskCtx, cancel := context.WithCancel(ctx)
	handle, _ := reg.RegisterWithCancel(params.ParentSC.SessionID, params.AgentID, params.AgentName, params.AgentID)
	reg.SetTaskCancel(handle.ID, cancel)

	go func() {
		defer cancel()
		res, err := Run(taskCtx, deps, params)
		if taskCtx.Err() != nil || errors.Is(err, context.Canceled) {
			_ = reg
			return
		}
		result := ""
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else if res != nil && res.Result != nil {
			result = res.Result.AssistantText
		}
		reg.Complete(handle.ID, result, errMsg, q)
	}()
	return handle.ID, nil
}
