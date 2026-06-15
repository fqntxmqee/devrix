package workmodel

import "errors"

// TaskStatus is the canonical status enum for D7-S1 work model tasks.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// ErrIllegalTransition is returned by UpdateStatus when a status transition
// violates the task state machine (D7-S1-T08).
var ErrIllegalTransition = errors.New("illegal task status transition")

// validTransitions defines the legal status transitions for D7-S1 tasks.
// Terminal states (completed, failed, cancelled) have no outgoing edges.
var validTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusPending:    {TaskStatusInProgress, TaskStatusCancelled},
	TaskStatusInProgress: {TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled},
	TaskStatusCompleted:  {},
	TaskStatusFailed:     {},
	TaskStatusCancelled:  {},
}

// IsLegalTransition reports whether transitioning from current to target is
// permitted by the task state machine.
func IsLegalTransition(current, target TaskStatus) bool {
	if current == target {
		return true
	}
	for _, legal := range validTransitions[current] {
		if target == legal {
			return true
		}
	}
	return false
}
