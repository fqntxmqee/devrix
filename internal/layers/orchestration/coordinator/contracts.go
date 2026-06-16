package coordinator

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// QueryLoopExecutor is the context-engine contract consumed by the
// orchestrator. It runs a single LLM↔Tool loop and streams EngineEvent.
// The orchestrator must NOT call into contextengine internals (no
// import of contextengine/prepare/attachments or prepare/usercontext).
type QueryLoopExecutor interface {
	// RunQueryLoop runs a single LLM↔Tool interaction and returns a
	// streaming channel of EngineEvent. The channel is closed on terminal
	// (completed | failed | cancelled).
	RunQueryLoop(ctx context.Context, req QueryRequest) (<-chan *contracts.EngineEvent, error)
}

// QueryRequest is the minimal D2 input the orchestrator needs.
type QueryRequest struct {
	SessionID    string
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolSpec
	MaxTurns     int
}

// ToolSpec is the tool descriptor for the D2 loop.
type ToolSpec struct {
	Name        string
	Description string
	Schema      map[string]any
}

// DelegateExecutor is the multi-agent contract for delegating an
// execute-type task to a worker (cursor / claude_code / subagent). The
// orchestrator must NOT import multiagent/delegate; it goes through
// this interface.
type DelegateExecutor interface {
	CreateAgent(ctx context.Context, spec AgentSpec) (AgentHandle, error)
	RunAgent(ctx context.Context, handle AgentHandle) (<-chan *contracts.EngineEvent, error)
	CancelAgent(ctx context.Context, handle AgentHandle) error
}

// AgentSpec describes a delegate worker.
type AgentSpec struct {
	WorkerType    string   // cursor | claude_code | subagent
	Directive     string   // task description
	Tools         []string // tool whitelist (e.g. "read", "grep", "edit")
	ContextPolicy string   // fresh | resume | upstream
	Metadata      map[string]string
}

// AgentHandle is opaque from D7's view; D4 owns the lifecycle.
type AgentHandle interface {
	ID() string
}

// EventPublisher publishes EngineEvent to the communication layer.
type EventPublisher interface {
	Publish(ctx context.Context, ev *contracts.EngineEvent)
}

// AdvisoryValidator is the optional evolution-layer advisory. Implementations
// must return within the configured timeout (default 50ms); a timeout is
// treated as "pass" by the orchestrator and recorded by the
// validation timeout counter.
type AdvisoryValidator interface {
	ValidateOrchestration(ctx context.Context, decision OrchestrationDecision) ValidationResult
}

// OrchestrationDecision is the input to advisory validation.
type OrchestrationDecision struct {
	Intent    IntentClassification
	SessionID string
	Plan      *Plan
}

// ValidationResult is the advisory output.
type ValidationResult struct {
	Pass   bool
	Reason string
}
