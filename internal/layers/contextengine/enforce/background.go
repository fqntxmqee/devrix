package enforce

import (
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// Complete marks a task finished and enqueues a task-notification.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from
// background.go (was 47 LOC). The notification body format follows the
// conventions documented in design.md §task_notifications:
//
//   - on success: "Background task <id> (<agentName>) completed: <result>"
//   - on failure: "Background task <id> failed: <errMsg>"
//
// The mode is contracts.ModeTaskNotification so the consuming queue
// processor can route the message to the appropriate session.
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