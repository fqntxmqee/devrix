package multiagent

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
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
	WorkerRole        string // explore|plan|implement (worker agents)
	TaskID            string
}

// AgentResult is produced when an agent terminates.
type AgentResult struct {
	Messages []types.Message
	ExitCode int
	Error    error
	Duration time.Duration
}

// PermissionGate approves tool execution (implemented by AgentPermissionGate).
type PermissionGate interface {
	Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool
}

// AgentDeps are injected dependencies for agent construction.
type AgentDeps struct {
	Engine        contracts.IEngine
	AgentObserver AgentObserver
	ObsBridge     *observability.Bridge
}

// AgentObserver receives agent lifecycle events (optional).
type AgentObserver interface {
	EmitAgentEvent(event AgentEvent)
}

// AgentObserverChain composes multiple observers into one.
type AgentObserverChain struct {
	observers []AgentObserver
}

// NewAgentObserverChain creates a chain from the given observers.
func NewAgentObserverChain(observers ...AgentObserver) *AgentObserverChain {
	return &AgentObserverChain{observers: observers}
}

// Add appends an observer to the chain.
func (c *AgentObserverChain) Add(obs AgentObserver) {
	c.observers = append(c.observers, obs)
}

// EmitAgentEvent dispatches the event to all observers in order.
func (c *AgentObserverChain) EmitAgentEvent(event AgentEvent) {
	for _, obs := range c.observers {
		if obs != nil {
			obs.EmitAgentEvent(event)
		}
	}
}

var _ AgentObserver = (*AgentObserverChain)(nil)

// AgentEvent is emitted to observers.
type AgentEvent struct {
	AgentID   string
	ParentID  string
	SessionID string
	EventType string
	Timestamp time.Time
	Metadata  map[string]any
}

// IAgentFactory creates agent instances.
type IAgentFactory interface {
	Create(ctx context.Context, cfg AgentConfig, session *types.Session) (Agent, error)
}

// MetaToolCallID is the metadata key under which the engine stores a
// tool_call identifier on both `*contracts.EngineEvent.Metadata` and
// `types.Message.Metadata`. It MUST stay in sync with
// `contextengine.MetaToolCallID` — the production context engine uses
// this key to tag tool_call events, and multi-agent dedup relies on it
// to collapse duplicate tool results from forked children.
//
// 历史：早期实现误用 "call_id" 作为 dedup key，与 contextengine 的
// "tool_call_id" 不一致，导致 dedup 在生产路径上为死代码。
// 2026-06-12 S4-Gate review 修正。
const MetaToolCallID = "tool_call_id"

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
	SetAgentObserver(AgentObserver)
	SetEngineEventSink(func(*contracts.EngineEvent))
}
