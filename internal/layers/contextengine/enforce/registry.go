package enforce

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BackgroundTask tracks an async SubQuery run.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from background.go
// to registry.go alongside BackgroundRegistry CRUD.
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
//
// DM-20260629-002 PR-3: extracted from background.go.
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
//
// DM-20260629-002 PR-3: extracted from background.go.
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
//
// DM-20260629-002 PR-3: extracted from background.go.
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
//
// DM-20260629-002 PR-3: extracted from background.go.
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
//
// DM-20260629-002 PR-3: extracted from background.go.
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

// Get returns a task by id.
//
// DM-20260629-002 PR-3: extracted from background.go.
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
//
// DM-20260629-002 PR-3: extracted from background.go.
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