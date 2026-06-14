// Package d7 implements the D7 Orchestration Domain.
//
// DSAFT: D7 (core) → S1-S5.
//
// D7 orchestrates D2 (LLM↔Tool execution) and D4 (multi-agent delegation),
// and publishes progress events to D1 (communication). It owns:
//   - D7-S1 Work Model — Task/Plan unified facade
//   - D7-S2 Session Orchestrator — D1 entry point replacement
//   - D7-S3 Wave Scheduler — DAG multi-agent scheduling (re-export from orchestration/wave)
//   - D7-S4 Execution Flow — FlowEvent aggregation (re-export from orchestration/flow)
//   - D7-S5 Decision & Planning — rule-based intent classification and task routing
//
// v1.0 is feature-flagged by orchestration.d7_enabled (default: false). When
// disabled, D1 Gateway routes directly to D2.ContextEngine.Process (legacy).
// When enabled, D1 routes to D7.SessionOrchestrator.ProcessMessage, which
// dispatches to FastPath (D2) or OrchestratePath (Plan / Wave).
package d7

import (
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// IntentKind is the routing decision produced by ClassifyIntent.
//
// "fast":       short/simple message → direct D2 RunQueryLoop, no Plan/Wave.
// "command":    recognized command (e.g. /plan, /stop) → handled by command-first path.
// "orchestrate": multi-step goal → Plan / Wave.
// "skip":       empty / no-op message.
type IntentKind string

const (
	IntentFast        IntentKind = "fast"
	IntentCommand     IntentKind = "command"
	IntentOrchestrate IntentKind = "orchestrate"
	IntentSkip        IntentKind = "skip"
)

// IntentClassification is the result of ClassifyIntent.
//
// Confidence is 0..100. The threshold for the FastPath is configurable via
// orchestration.fast_path.confidence_threshold (default 90). Routes:
//   - skip        : always skip (no LLM call)
//   - command     : always go command-first (no LLM call, no FastPath proxy)
//   - fast        : go to D2.RunQueryLoop directly (FastPath proxy)
//   - orchestrate : SynthesizeTaskGraph (v1.1+) or PlanMode/manual
type IntentClassification struct {
	Kind       IntentKind
	Confidence int
	Reason     string
	Command    string // populated when Kind == IntentCommand
}

// TaskStatus is the canonical status enum used by the WorkModel facade.
//
// In v1.0, status transitions are not strictly validated (see R1 Q3). v1.1
// introduces a state-machine guard. Aliases for the v1.0 target names from
// the spec are documented in d7-domain.md.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskType is the work-class used by SynthesizeTaskGraph (v1.1) and executor
// selection in v1.0 (PlanAgent maps explore|plan to D2 read-only; execute
// routes to D4 delegate workers).
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
	// Metadata is forwarded to the underlying executor. Optional.
	Metadata map[string]string
}

// ProcessResult is the outcome of ProcessMessage. EventCh streams EngineEvent
// for D1 to forward to the IM worker_progress sink. Err is set on the first
// non-recoverable failure and the channel is closed.
type ProcessResult struct {
	EventCh <-chan *contracts.EngineEvent
	Err     error
}
