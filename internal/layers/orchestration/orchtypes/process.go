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
//
// Phase 7 PR-7.2 (D7-S13-A48-T04): adds TrackMode string field to switch
// between Developer (Beta(5,3)) and Operator (Beta(8,1)) prior defaults.
// Empty string is the zero value and is treated as Developer by the
// orchestrator's fail-safe path. Invalid values are logged and fall back
// to Developer (matches the design in Phase 5 PR-E3 fail-safe).
type ProcessRequest struct {
	SessionID string
	Message   string
	Metadata  map[string]string

	// TrackMode — "developer" / "operator" / "" (zero value = developer).
	// Threaded into SessionOrchestrator.buildObserveRequest → Learner.Inject
	// to select the prior Beta for the next Observe call. When the
	// ReputationStore has a row for SessionID with a different TrackMode,
	// the Reputation's TrackMode takes precedence (cross-session state wins
	// over the per-request hint, see inject track-mode policy in
	// learner.go:DefaultLearner.Inject).
	TrackMode string
}

// ProcessResult is the outcome of ProcessMessage.
type ProcessResult struct {
	EventCh <-chan *contracts.EngineEvent
	Err     error
}

// TrackMode values (Phase 7 PR-7.2, D7-S13-A48-T04).
//
// Valid values: TrackModeDeveloper, TrackModeOperator, "" (zero value = developer).
// Any other value is treated as Developer by the orchestrator's fail-safe
// path (matches Phase 5 PR-E3 unknown-track-mode policy in
// learn.defaultPriorForTrackMode).
const (
	// TrackModeDeveloper — slightly positive prior Beta(5,3).
	TrackModeDeveloper = "developer"

	// TrackModeOperator — strongly positive prior Beta(8,1).
	TrackModeOperator = "operator"
)

// NewProcessRequest is the fail-fast constructor for ProcessRequest. It
// validates TrackMode against the learn package's ParseTrackMode and
// returns an error for empty SessionID or Message.
//
// TrackMode:
//   - "" (zero value)        → accepted, treated as developer
//   - "developer" / "operator" → accepted
//   - other non-empty value  → returns ErrInvalidTrackMode
//
// Note: NewProcessRequest enforces the strict path. The orchestrator's
// buildObserveRequest has a separate lenient path (log warn + fall back
// to developer) for callers that bypass NewProcessRequest. Both paths
// converge on the same DeveloperPrior fail-safe.
func NewProcessRequest(sessionID, message, trackMode string) (ProcessRequest, error) {
	if sessionID == "" {
		return ProcessRequest{}, ErrProcessRequestSessionIDEmpty
	}
	if message == "" {
		return ProcessRequest{}, ErrProcessRequestMessageEmpty
	}
	if trackMode != "" && trackMode != TrackModeDeveloper && trackMode != TrackModeOperator {
		return ProcessRequest{}, ErrProcessRequestInvalidTrackMode
	}
	return ProcessRequest{
		SessionID: sessionID,
		Message:   message,
		TrackMode: trackMode,
	}, nil
}
