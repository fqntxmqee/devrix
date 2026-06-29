package enforce

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// RunBackground starts SubQuery asynchronously and registers notification on completion.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from background.go
// (was a 26-LOC god function). The actual run loop is here; Complete
// (queue integration) lives in notifications.go.
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

// BackgroundWaiter blocks on background-task terminal transitions.
//
// DM-20260629-002 PR-3: co-located with RunBackground (the waiter is the
// read-side counterpart to the async writer). Kept in run.go rather than
// registry.go because it never mutates the registry.
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