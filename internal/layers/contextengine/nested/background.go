package nested

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
	// cancel is the per-task cancel func; not serialized.
	cancel context.CancelFunc
	// done is closed when the task transitions to a terminal state. It
	// backs BackgroundWaiter.Wait so observers wake up without polling.
	done chan struct{}
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
// The returned handle's Cancel field is a no-op (cancel is owned by the
// caller) — callers should run their work with a context derived via
// context.WithCancel and pass the cancel func here so the registry can
// invoke it on Cancel(taskID).
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
	return CancelHandle{ID: id, Cancel: func() {}}, task.cancel // populated below via initCancel
}

// initCancel attaches the cancel func to the task after registration.
// Exposed via SetTaskCancel to avoid leaking the mu into callers.
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

// Cancel cancels a running task. Idempotent: returns true the first time
// the task was transitioned to "cancelled", false on subsequent calls or
// when the task id is unknown. Cancel is a no-op for already-terminal
// tasks (completed / failed / cancelled) so background goroutines can
// call it without coordination.
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

// Complete marks a task finished and enqueues a task-notification. If the
// task was already cancelled mid-flight (status="cancelled"), no
// notification is emitted — the tombstone is the cancellation record.
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
		// Tombstone: cancel wins; do not enqueue a completed/failed
		// notification. Just signal waiters.
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

// List returns a snapshot of tasks for a given session. Returned tasks
// are shallow copies so callers can read fields without holding the lock.
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

// BackgroundWaiter blocks on background-task terminal transitions. It is
// intentionally separate from BackgroundRegistry so production code that
// only needs Cancel/Get/List does not pay for the done-channel map.
type BackgroundWaiter struct {
	reg  *BackgroundRegistry
	mu   sync.Mutex
	done map[string]chan struct{}
}

// NewBackgroundWaiter creates a waiter bound to a registry.
func NewBackgroundWaiter(reg *BackgroundRegistry) *BackgroundWaiter {
	return &BackgroundWaiter{reg: reg, done: make(map[string]chan struct{})}
}

// Register binds a waiter to a specific task id, capturing its done
// channel. Must be called after RegisterWithCancel for the same id.
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

// Wait blocks until taskID reaches a terminal state (completed / failed /
// cancelled) or the timeout elapses. Returns (snapshot, true) on terminal,
// (running-snapshot, false) on timeout. Max supported timeout is 600s.
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
		// Fallback: poll until terminal.
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
// If the parent ctx or a registry-level Cancel(taskID) fires, the task is
// transitioned to "cancelled" and no task-notification is enqueued.
func RunBackground(ctx context.Context, deps LoopDeps, params SubQueryParams, reg *BackgroundRegistry, q contracts.SessionCommandQueue) (string, error) {
	if reg == nil {
		return "", fmt.Errorf("background registry is nil")
	}
	taskCtx, cancel := context.WithCancel(ctx)
	handle, _ := reg.RegisterWithCancel(params.ParentSC.SessionID, params.AgentID, params.AgentName, params.AgentID)
	reg.SetTaskCancel(handle.ID, cancel)

	go func() {
		defer cancel()
		res, err := Run(taskCtx, deps, params)
		// If the task was cancelled mid-flight (status was flipped to
		// "cancelled" by Cancel), suppress the completion notification.
		// Check is racy by design — Complete() also checks under the lock
		// and no-ops if the task was already terminal.
		if taskCtx.Err() != nil || errors.Is(err, context.Canceled) {
			_ = reg // already cancelled via cancel() callback
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
