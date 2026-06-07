package types

import (
	"fmt"
	"time"
)

// TaskFlowStatus represents the status of a task flow
type TaskFlowStatus string

const (
	TaskFlowStatusPending    TaskFlowStatus = "pending"
	TaskFlowStatusRunning    TaskFlowStatus = "running"
	TaskFlowStatusCompleted  TaskFlowStatus = "completed"
	TaskFlowStatusFailed     TaskFlowStatus = "failed"
)

// TaskFlow represents a task execution flow (Aggregate)
type TaskFlow struct {
	ID               string
	Name             string
	Description      string
	DAG              *MilestoneDAG
	CurrentMilestone string // 当前执行的里程碑 ID
	Status           TaskFlowStatus
	OverallProgress  float64 // 0.0-1.0
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewTaskFlow creates a new task flow
func NewTaskFlow(id, name string, dag *MilestoneDAG) *TaskFlow {
	now := time.Now()
	return &TaskFlow{
		ID:               id,
		Name:             name,
		DAG:              dag,
		Status:           TaskFlowStatusPending,
		OverallProgress:  0.0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// SetStatus sets the task flow status
func (t *TaskFlow) SetStatus(status TaskFlowStatus) {
	t.Status = status
	t.UpdatedAt = time.Now()
}

// UpdateProgress updates the overall progress based on milestone progress
func (t *TaskFlow) UpdateProgress() {
	if t.DAG == nil {
		return
	}
	t.OverallProgress = t.DAG.CalculateOverallProgress()
	t.UpdatedAt = time.Now()
}

// GetCurrentMilestone returns the current milestone being executed
func (t *TaskFlow) GetCurrentMilestone() *Milestone {
	if t.CurrentMilestone == "" || t.DAG == nil {
		return nil
	}
	return t.DAG.Milestones[t.CurrentMilestone]
}

// Start starts the task flow execution
func (t *TaskFlow) Start() error {
	if t.Status != TaskFlowStatusPending {
		return fmt.Errorf("task flow already started")
	}

	// Find the first executable milestone (no dependencies)
	order, err := t.DAG.GetExecutionOrder()
	if err != nil {
		return err
	}

	if len(order) == 0 {
		return fmt.Errorf("no milestones in task flow")
	}

	// Start with the first milestone
	t.CurrentMilestone = order[0].ID
	t.Status = TaskFlowStatusRunning
	t.UpdatedAt = time.Now()

	order[0].SetStatus(MilestoneStatusInProgress)
	return nil
}

// AdvanceToNext moves to the next milestone
func (t *TaskFlow) AdvanceToNext() error {
	if t.DAG == nil {
		return fmt.Errorf("DAG is nil")
	}

	// Complete current milestone
	current := t.DAG.Milestones[t.CurrentMilestone]
	if current != nil {
		current.SetStatus(MilestoneStatusCompleted)
		current.SetProgress(1.0)
	}

	// Find next executable milestone
	order, err := t.DAG.GetExecutionOrder()
	if err != nil {
		t.SetStatus(TaskFlowStatusFailed)
		return err
	}

	for _, m := range order {
		if m.Status == MilestoneStatusPending && !m.IsBlocked(t.DAG.Milestones) {
			t.CurrentMilestone = m.ID
			m.SetStatus(MilestoneStatusInProgress)
			t.UpdateProgress()
			return nil
		}
	}

	// All milestones completed
	t.Status = TaskFlowStatusCompleted
	t.CurrentMilestone = ""
	t.UpdateProgress()
	return nil
}

// Fail marks the current milestone as failed and stops the flow
func (t *TaskFlow) Fail(reason string) error {
	if t.DAG == nil || t.CurrentMilestone == "" {
		return fmt.Errorf("no current milestone")
	}

	current := t.DAG.Milestones[t.CurrentMilestone]
	if current != nil {
		current.SetStatus(MilestoneStatusFailed)
	}

	t.Status = TaskFlowStatusFailed
	t.UpdatedAt = time.Now()
	return nil
}

// GetStatusSummary returns a summary of the task flow status
func (t *TaskFlow) GetStatusSummary() string {
	if t.DAG == nil {
		return "No milestones"
	}

	var completed, inProgress, pending, failed int
	for _, m := range t.DAG.Milestones {
		switch m.Status {
		case MilestoneStatusCompleted:
			completed++
		case MilestoneStatusInProgress:
			inProgress++
		case MilestoneStatusPending:
			pending++
		case MilestoneStatusFailed:
			failed++
		}
	}

	return fmt.Sprintf("Completed: %d, In Progress: %d, Pending: %d, Failed: %d",
		completed, inProgress, pending, failed)
}
