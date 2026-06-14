package coordinator

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// D2Executor is the D2 contract consumed by D7. It runs a single LLM↔Tool
// loop and streams EngineEvent. D7 must NOT call into D2 internals (no
// import of contextengine/queue/attachments/usercontext).
type D2Executor interface {
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

// D4Executor is the D4 contract for delegating an execute-type task to a
// worker (cursor / claude_code / subagent). D7 must NOT import
// multiagent/delegate; it goes through this interface.
type D4Executor interface {
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

// D1EventSink publishes EngineEvent to the communication layer.
type D1EventSink interface {
	Publish(ctx context.Context, ev *contracts.EngineEvent)
}

// D6Validator is the optional advisory D6 layer. Implementations must return
// within the configured timeout (default 50ms); a timeout is treated as
// "pass" by the orchestrator and recorded by the D5 timeout counter.
type D6Validator interface {
	ValidateOrchestration(ctx context.Context, decision OrchestrationDecision) ValidationResult
}

// OrchestrationDecision is the input to D6 advisory validation.
type OrchestrationDecision struct {
	Intent    IntentClassification
	SessionID string
	Plan      *Plan
}

// ValidationResult is the D6 advisory output.
type ValidationResult struct {
	Pass   bool
	Reason string
}
