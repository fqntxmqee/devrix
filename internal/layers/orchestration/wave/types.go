// Package wave implements the WaveScheduler: a DAG-driven, 5-slot worker pool
// that dispatches Plan Engine tasks to SubAgents and CLI Agent Tools in parallel.
//
// Design references: openspec/changes/devrix-wave-scheduler/design.md §1-§12
package wave

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// WorkerType identifies a worker kind in the pool. Each kind has a fixed slot
// count configured via WorkerPoolCapacity (cursor=1, claude_code=1, subagent=3).
type WorkerType string

const (
	WorkerCursor     WorkerType = "cursor"
	WorkerClaudeCode WorkerType = "claude_code"
	WorkerSubAgent   WorkerType = "subagent"
)

// Valid reports whether the worker type is recognized.
func (w WorkerType) Valid() bool {
	switch w {
	case WorkerCursor, WorkerClaudeCode, WorkerSubAgent:
		return true
	default:
		return false
	}
}

// ContextPolicy decides what messages / system prompt to feed a worker.
type ContextPolicy string

const (
	// ContextFresh: no Leader history; new Sidechain; directive only.
	ContextFresh ContextPolicy = "fresh"
	// ContextResume: sidechain.Load(agentID); QueryDepth+1.
	ContextResume ContextPolicy = "resume"
	// ContextUpstream: dependency artifact; directive + upstream summary.
	ContextUpstream ContextPolicy = "upstream"
)

// Valid reports whether the policy is recognized.
func (p ContextPolicy) Valid() bool {
	switch p {
	case ContextFresh, ContextResume, ContextUpstream:
		return true
	default:
		return false
	}
}

// TaskNode is the unit of work produced by Plan Engine. The Scheduler treats
// this as immutable input; the in-memory runtime state is tracked separately.
type TaskNode struct {
	ID                string         `json:"id"`
	Title             string         `json:"title"`
	Directive         string         `json:"directive"`
	WorkerType        WorkerType     `json:"worker_type"`
	DependsOn         []string       `json:"depends_on,omitempty"`
	ContextPolicy     ContextPolicy  `json:"context_policy"`
	UpstreamTaskID    string         `json:"upstream_task_id,omitempty"`
	SidechainAgentID  string         `json:"sidechain_agent_id,omitempty"`
	ParentSessionID   string         `json:"parent_session_id,omitempty"`
	FileScope         []string       `json:"file_scope,omitempty"`
	ConflictGroup     string         `json:"conflict_group,omitempty"`
	SystemPromptExtra string         `json:"system_prompt_extra,omitempty"`
	ReadOnly          bool           `json:"read_only,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

// Validate ensures the node has the minimum required fields.
func (n TaskNode) Validate() error {
	if n.ID == "" {
		return errWave("task node: id is required")
	}
	if !n.WorkerType.Valid() {
		return errWave("task node %q: invalid worker_type %q", n.ID, n.WorkerType)
	}
	if !n.ContextPolicy.Valid() {
		return errWave("task node %q: invalid context_policy %q", n.ID, n.ContextPolicy)
	}
	if n.ContextPolicy == ContextUpstream && n.UpstreamTaskID == "" {
		return errWave("task node %q: context_policy=upstream requires upstream_task_id", n.ID)
	}
	if n.ContextPolicy == ContextResume && n.SidechainAgentID == "" {
		return errWave("task node %q: context_policy=resume requires sidechain_agent_id", n.ID)
	}
	if n.Directive == "" {
		return errWave("task node %q: directive is required", n.ID)
	}
	return nil
}

// WorkspaceDir returns the per-task work directory. v1.0 leaves this empty
// so workers inherit the Leader session WorkDir via the runner (mirrors
// design §3.3 — "WorkerRunSpec.WorkDir"); tests inject workdir via the
// directive or runner deps. A future v1.2 change may add per-task workdir.
func (n TaskNode) WorkspaceDir() string {
	return ""
}

// TaskState is the runtime lifecycle state of a TaskNode as tracked by the scheduler.
type TaskState string

const (
	StatePending   TaskState = "pending"
	StateReady     TaskState = "ready"
	StateRunning   TaskState = "running"
	StateCompleted TaskState = "completed"
	StateFailed    TaskState = "failed"
	StateCancelled TaskState = "cancelled"
)

// Terminal reports whether the state is final.
func (s TaskState) Terminal() bool {
	switch s {
	case StateCompleted, StateFailed, StateCancelled:
		return true
	default:
		return false
	}
}

// Artifact is the outcome of a worker run; written to ArtifactStore on terminal
// transitions and consumed by downstream tasks with context_policy=upstream.
type Artifact struct {
	TaskID       string         `json:"task_id"`
	SessionID    string         `json:"session_id,omitempty"`
	WorkerType   WorkerType     `json:"worker_type"`
	Summary      string         `json:"summary"`
	FilesChanged []string       `json:"files_changed,omitempty"`
	ExitCode     int            `json:"exit_code"`
	Error        string         `json:"error,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	EndedAt      time.Time      `json:"ended_at"`
	Duration     time.Duration  `json:"duration"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ResolvedContext is the materialized context handed to a worker runner.
type ResolvedContext struct {
	Policy       ContextPolicy
	SystemPrompt string
	Messages     []types.Message
	// UpstreamSummary is non-empty when Policy=upstream.
	UpstreamSummary string
	// ResumeAgentID is the agent whose sidechain is loaded when Policy=resume.
	ResumeAgentID string
}

// WorkerEvent is a streaming event emitted by a worker runner. The scheduler
// fans these out to FlowHub and the worker card renderer.
type WorkerEvent struct {
	Type    string // "thinking" | "text" | "tool_use" | "error" | "complete" | "cancelled"
	Content string
	At      time.Time
}

// WorkerRunner is implemented by SubAgent / AgentTool / Stub runners. It owns
// the per-worker goroutine, must listen to ctx.Done(), and emits terminal
// events so the scheduler can release the slot.
type WorkerRunner interface {
	Kind() WorkerType
	Run(ctx context.Context, spec WorkerRunSpec) error
}

// WorkerRunSpec is the immutable input handed to a runner.
type WorkerRunSpec struct {
	SessionID    string
	TaskID       string
	WorkerID     string
	WorkDir      string
	Directive    string
	Context      ResolvedContext
	Emit         func(WorkerEvent)
	BackgroundID string // SubQuery background id; "" for non-subagent workers
}

// SlotID identifies a slot in the pool; opaque, comparable.
type SlotID string

// PoolCapacity is the default Worker pool configuration (D4 — cursor=1, claude_code=1, subagent=3).
var DefaultPoolCapacity = map[WorkerType]int{
	WorkerCursor:     1,
	WorkerClaudeCode: 1,
	WorkerSubAgent:   3,
}
