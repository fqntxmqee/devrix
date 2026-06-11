package contextengine

import (
	"github.com/devrix/devrix/internal/layers/contextengine/pev"
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
)

// TaskIntegration provides integration between PEV and TaskManager.
type TaskIntegration struct {
	manager *tasks.TaskManager
}

// NewTaskIntegration creates a new task integration.
func NewTaskIntegration(manager *tasks.TaskManager) *TaskIntegration {
	return &TaskIntegration{manager: manager}
}

// OnPlanCompleted creates tasks from plan milestones.
func (t *TaskIntegration) OnPlanCompleted(sessionID string, planResult *pev.PlanResult) {
	if planResult == nil || t.manager == nil {
		return
	}

	for _, m := range planResult.Milestones {
		t.manager.Create(sessionID, m.Name, m.Description)
	}
}

// OnMilestoneStart marks a milestone as in_progress.
func (t *TaskIntegration) OnMilestoneStart(sessionID, milestoneID string) {
	if t.manager == nil {
		return
	}

	// Find task matching milestone
	taskList := t.manager.List(sessionID)
	for _, task := range taskList {
		// Match by name pattern or ID
		if task.Subject == milestoneID || task.ID == milestoneID {
			t.manager.UpdateStatus(sessionID, task.ID, tasks.TaskStatusInProgress)
			return
		}
	}
}

// OnMilestoneComplete marks a milestone as completed.
func (t *TaskIntegration) OnMilestoneComplete(sessionID, milestoneID string) {
	if t.manager == nil {
		return
	}

	taskList := t.manager.List(sessionID)
	for _, task := range taskList {
		if task.Subject == milestoneID || task.ID == milestoneID {
			t.manager.UpdateStatus(sessionID, task.ID, tasks.TaskStatusCompleted)
			return
		}
	}
}

// OnMilestoneFail marks a milestone as failed.
func (t *TaskIntegration) OnMilestoneFail(sessionID, milestoneID, reason string) {
	if t.manager == nil {
		return
	}

	taskList := t.manager.List(sessionID)
	for _, task := range taskList {
		if task.Subject == milestoneID || task.ID == milestoneID {
			t.manager.UpdateStatus(sessionID, task.ID, tasks.TaskStatusFailed)
			return
		}
	}
}

// GetTaskSummary returns a summary of tasks for display.
func (t *TaskIntegration) GetTaskSummary(sessionID string) string {
	if t.manager == nil {
		return ""
	}

	taskList := t.manager.List(sessionID)
	if len(taskList) == 0 {
		return ""
	}

	var completed, inProgress, pending, failed int
	for _, task := range taskList {
		switch task.Status {
		case tasks.TaskStatusCompleted:
			completed++
		case tasks.TaskStatusInProgress:
			inProgress++
		case tasks.TaskStatusPending:
			pending++
		case tasks.TaskStatusFailed:
			failed++
		}
	}

	return tasks.FormatTaskSummary(completed, inProgress, pending, failed)
}
