package orchtypes

import (
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// TaskStatus is the canonical status enum used by the WorkModel facade.
type TaskStatus = workmodel.TaskStatus

const (
	TaskStatusPending    = workmodel.TaskStatusPending
	TaskStatusInProgress = workmodel.TaskStatusInProgress
	TaskStatusCompleted  = workmodel.TaskStatusCompleted
	TaskStatusFailed     = workmodel.TaskStatusFailed
	TaskStatusCancelled  = workmodel.TaskStatusCancelled
)

// TaskType is the work-class used by SynthesizeTaskGraph and executor selection.
type TaskType string

const (
	TaskTypeExplore    TaskType = "explore"
	TaskTypePlan       TaskType = "plan"
	TaskTypeExecute    TaskType = "execute"
	TaskTypeBackground TaskType = "background"
)

// TaskSpec is a serializable, in-flight task description.
type TaskSpec struct {
	ID           string    `json:"id"`
	Subject      string    `json:"subject"`
	Type         TaskType  `json:"type"`
	Goal         string    `json:"goal,omitempty"`
	Dependencies []string  `json:"dependencies,omitempty"`
	WorkerType   string    `json:"worker_type,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Plan is a DAG of tasks for a single orchestration request.
type Plan struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Tasks     []TaskSpec `json:"tasks"`
	CreatedAt time.Time  `json:"created_at"`
}

// ProcessRequest is the input to SessionOrchestrator.ProcessMessage.
type ProcessRequest struct {
	SessionID string
	Message   string
	Metadata  map[string]string
}

// ProcessResult is the outcome of ProcessMessage.
type ProcessResult struct {
	EventCh <-chan *contracts.EngineEvent
	Err     error
}
