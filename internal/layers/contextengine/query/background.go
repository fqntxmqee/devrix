package query

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/queue"
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
}

// BackgroundRegistry tracks in-flight and completed background tasks.
type BackgroundRegistry struct {
	mu    sync.RWMutex
	tasks map[string]*BackgroundTask
}

// NewBackgroundRegistry creates a registry.
func NewBackgroundRegistry() *BackgroundRegistry {
	return &BackgroundRegistry{tasks: make(map[string]*BackgroundTask)}
}

// Register adds a running task and returns its id.
func (r *BackgroundRegistry) Register(sessionID, agentID, agentName string) string {
	if r == nil {
		return ""
	}
	id := "bg_" + uuid.New().String()[:8]
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[id] = &BackgroundTask{
		ID:        id,
		SessionID: sessionID,
		AgentID:   agentID,
		AgentName: agentName,
		Status:    "running",
		StartedAt: time.Now(),
	}
	return id
}

// Complete marks a task finished and enqueues a task-notification.
func (r *BackgroundRegistry) Complete(taskID, result, errMsg string, q *queue.SessionQueue) {
	if r == nil {
		return
	}
	r.mu.Lock()
	task, ok := r.tasks[taskID]
	if !ok {
		r.mu.Unlock()
		return
	}
	task.Status = "completed"
	if errMsg != "" {
		task.Status = "failed"
		task.Error = errMsg
	}
	task.Result = result
	task.EndedAt = time.Now()
	r.mu.Unlock()

	if q != nil {
		body := result
		if errMsg != "" {
			body = fmt.Sprintf("Background task %s failed: %s", taskID, errMsg)
		} else {
			body = fmt.Sprintf("Background task %s (%s) completed: %s", taskID, task.AgentName, result)
		}
		q.Enqueue(task.SessionID, queue.QueuedCommand{
			Value:   body,
			Mode:    queue.ModeTaskNotification,
			AgentID: task.AgentID,
		})
	}
}

// Get returns a task by id.
func (r *BackgroundRegistry) Get(taskID string) (*BackgroundTask, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[taskID]
	return t, ok
}

// RunBackground starts SubQuery asynchronously and registers notification on completion.
func RunBackground(ctx context.Context, deps LoopDeps, params SubQueryParams, reg *BackgroundRegistry, q *queue.SessionQueue) (string, error) {
	if reg == nil {
		return "", fmt.Errorf("background registry is nil")
	}
	taskID := reg.Register(params.ParentSC.SessionID, params.AgentID, params.AgentName)
	go func() {
		res, err := Run(ctx, deps, params)
		result := ""
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else if res != nil && res.Result != nil {
			result = res.Result.AssistantText
		}
		reg.Complete(taskID, result, errMsg, q)
	}()
	return taskID, nil
}
