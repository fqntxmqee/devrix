package workmodel

import (
	"time"

	"github.com/google/uuid"
)

// WorkKind classifies the intent of a work item in the unified tree.
type WorkKind string

const (
	WorkKindGoal      WorkKind = "goal"
	WorkKindExplore   WorkKind = "explore"
	WorkKindPlan      WorkKind = "plan"
	WorkKindImplement WorkKind = "implement"
	WorkKindVerify    WorkKind = "verify"
	WorkKindChecklist WorkKind = "checklist"
	WorkKindShell     WorkKind = "shell"
	WorkKindAgent     WorkKind = "agent"
)

// ExecPolicy describes how a work item should be executed.
type ExecPolicy string

const (
	ExecPolicySync       ExecPolicy = "sync"
	ExecPolicyAsync      ExecPolicy = "async"
	ExecPolicyReadonly   ExecPolicy = "readonly"
	ExecPolicyParallelOK ExecPolicy = "parallel_ok"
)

// WorkItem is the canonical work-unit model for D7 orchestration.
type WorkItem struct {
	ID            string     `json:"id"`
	ParentID      string     `json:"parent_id,omitempty"`
	Kind          WorkKind   `json:"kind"`
	Status        TaskStatus `json:"status"`
	Title         string     `json:"title"`
	Directive     string     `json:"directive,omitempty"`
	Uncertainty   float64    `json:"uncertainty,omitempty"`
	RoundPhase    RoundPhase `json:"round_phase,omitempty"`
	LastRound     *WorkItemPipelineRound `json:"last_round,omitempty"`
	Policy        ExecPolicy `json:"policy,omitempty"`
	Owner         string     `json:"owner,omitempty"`
	BlockedBy     []string   `json:"blocked_by,omitempty"`
	Blocks        []string   `json:"blocks,omitempty"`
	RunRef        string     `json:"run_ref,omitempty"`
	Ephemeral     bool       `json:"ephemeral,omitempty"`
	Locked        bool       `json:"locked,omitempty"`
	SourceSession  string          `json:"source_session,omitempty"`
	ContextScopeID string          `json:"context_scope_id,omitempty"`
	ContextPolicy  ContextLinkKind `json:"context_policy,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateWorkItemInput holds fields for creating a work item.
type CreateWorkItemInput struct {
	ParentID      string
	Kind          WorkKind
	Title         string
	Directive     string
	Uncertainty   float64
	Policy        ExecPolicy
	Ephemeral     bool
	SourceSession string
}

// ChecklistEntry is one row in a todo_write snapshot.
type ChecklistEntry struct {
	Content    string
	Status     TaskStatus
	ActiveForm string
}

// ErrWorkItemLocked is returned when mutating a locked historical item.
var ErrWorkItemLocked = errWorkItem("work item is locked")

// ErrDependencyCycle is returned when a dependency would create a cycle.
var ErrDependencyCycle = errWorkItem("dependency cycle detected")

type errWorkItem string

func (e errWorkItem) Error() string { return string(e) }

// NewWorkItem creates a work item with generated ID and defaults.
func NewWorkItem(kind WorkKind, title, directive string) *WorkItem {
	now := time.Now()
	if kind == "" {
		kind = WorkKindImplement
	}
	policy := ExecPolicySync
	if kind == WorkKindExplore || kind == WorkKindPlan || kind == WorkKindVerify {
		policy = ExecPolicyReadonly
	}
	return &WorkItem{
		ID:        "wi_" + uuid.New().String()[:8],
		Kind:      kind,
		Title:     title,
		Directive: directive,
		Status:    TaskStatusPending,
		Policy:    policy,
		BlockedBy: []string{},
		Blocks:    []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
