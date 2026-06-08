package multiagent

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// AgentState is the agent lifecycle state.
type AgentState int

const (
	AgentStateCreated AgentState = iota
	AgentStateRunning
	AgentStateIterating
	AgentStateWaitingPermission
	AgentStateTerminated
)

// String returns the stable state name.
func (s AgentState) String() string {
	switch s {
	case AgentStateCreated:
		return "CREATED"
	case AgentStateRunning:
		return "RUNNING"
	case AgentStateIterating:
		return "ITERATING"
	case AgentStateWaitingPermission:
		return "WAITING_PERMISSION"
	case AgentStateTerminated:
		return "TERMINATED"
	default:
		return "UNKNOWN"
	}
}

// IsActive reports whether the agent is not yet terminated.
func (s AgentState) IsActive() bool {
	return s >= AgentStateRunning && s <= AgentStateWaitingPermission
}

// IsTerminal reports whether the agent has finished.
func (s AgentState) IsTerminal() bool {
	return s == AgentStateTerminated
}

// CollaborationMode selects prompt enhancement strategy.
type CollaborationMode string

const (
	ModeChainOfThought      CollaborationMode = "chain-of-thought"
	ModeIterativeRefinement CollaborationMode = "iterative-refinement"
	ModeDefault             CollaborationMode = "default"
)

// AgentConfig holds creation-time settings.
type AgentConfig struct {
	SessionID         string
	WorkDir           string
	Mode              CollaborationMode
	MaxIter           int
	MaxChildren       int
	Timeout           time.Duration
	PermissionTimeout time.Duration
	SystemPrompt      string
	ParentID          string
	InitialInput      string
}

// AgentResult is produced when an agent terminates.
type AgentResult struct {
	Messages []types.Message
	ExitCode int
	Error    error
	Duration time.Duration
}

// AgentDeps are injected dependencies for agent construction.
type AgentDeps struct {
	Engine        contracts.IEngine
	Observer      contextengine.IObserver
	AgentObserver AgentObserver
}

// AgentObserver receives agent lifecycle events (optional).
type AgentObserver interface {
	EmitAgentEvent(event AgentEvent)
}

// AgentEvent is emitted to observers.
type AgentEvent struct {
	AgentID   string
	ParentID  string
	SessionID string
	EventType string
	State     AgentState
	Mode      CollaborationMode
	Timestamp time.Time
	Metadata  map[string]any
}

// IAgentFactory creates agent instances.
type IAgentFactory interface {
	Create(ctx context.Context, cfg AgentConfig, session *types.Session) (Agent, error)
}

// Agent is the multi-agent aggregate root interface.
type Agent interface {
	ID() string
	State() AgentState
	Config() AgentConfig
	Run(ctx context.Context) (*AgentResult, error)
	Fork(ctx context.Context, cfg AgentConfig) (Agent, error)
	Join(ctx context.Context, child Agent) error
	Terminate(ctx context.Context) error
	Wait(ctx context.Context) (*AgentResult, error)
	ResolvePermission(toolName string, granted bool)
	GetMessages() []types.Message
}
