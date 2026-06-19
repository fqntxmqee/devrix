package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// TurnExecutor is the D7 turn-runtime contract consumed by the orchestrator.
type TurnExecutor interface {
	RunTurn(ctx context.Context, req QueryRequest) (<-chan *contracts.EngineEvent, error)
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

// DelegateExecutor is the multi-agent contract for delegating an execute-type task.
type DelegateExecutor interface {
	CreateAgent(ctx context.Context, spec AgentSpec) (AgentHandle, error)
	RunAgent(ctx context.Context, handle AgentHandle) (<-chan *contracts.EngineEvent, error)
	CancelAgent(ctx context.Context, handle AgentHandle) error
}

// AgentSpec describes a delegate worker.
type AgentSpec struct {
	WorkerType    string
	Directive     string
	Tools         []string
	ContextPolicy string
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

// AdvisoryValidator is the optional evolution-layer advisory.
type AdvisoryValidator interface {
	ValidateOrchestration(ctx context.Context, decision OrchestrationDecision) ValidationResult
}

// OrchestrationDecision is the input to advisory validation.
type OrchestrationDecision struct {
	Intent    orchtypes.IntentClassification
	SessionID string
	Plan      *orchtypes.Plan
}

// ValidationResult is the advisory output.
type ValidationResult struct {
	Pass   bool
	Reason string
}
