package communication

import "log/slog"

// TaskFlowHandler handles task flow for milestone DAG (V3 feature)
type TaskFlowHandler struct{}

// NewTaskFlowHandler creates a new TaskFlowHandler
func NewTaskFlowHandler() *TaskFlowHandler {
	return &TaskFlowHandler{}
}

// HandleMilestoneDAG logs that task flow is not implemented in V1
func (h *TaskFlowHandler) HandleMilestoneDAG(sessionID string) {
	slog.Info("Task flow (Milestone DAG) not implemented in V1",
		"sessionID", sessionID,
		"hint", "V3 will implement milestone DAG for task progress tracking",
	)
}

// IsSupported returns false since V1 doesn't support milestone DAG
func (h *TaskFlowHandler) IsSupported() bool {
	return false
}
