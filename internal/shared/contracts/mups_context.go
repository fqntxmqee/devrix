package contracts

import (
	"context"
	"errors"

	"github.com/devrix/devrix/internal/shared/types"
)

// MUPSPhase identifies an MUPS pipeline node for D2 context materialize.
type MUPSPhase string

const (
	MUPSPhaseObserve MUPSPhase = "observe"
	MUPSPhasePlan    MUPSPhase = "plan"
	MUPSPhaseExecute MUPSPhase = "execute"
	MUPSPhaseVerify  MUPSPhase = "verify"
	MUPSPhaseLearn   MUPSPhase = "learn"
	MUPSPhaseDecide  MUPSPhase = "decide"
)

// MUPSContextPolicy controls compression, locale, and agent profile for MUPS materialize.
type MUPSContextPolicy struct {
	TokenBudget     int
	ToolProfile     string
	Locale          string
	AgentProfile    string
	Depth           int
	WorkerLocal     bool
	CompressPerTurn bool
}

// MUPSScopeContract carries scope boundary fields for execute output hints.
type MUPSScopeContract struct {
	GoalStatement   string   `json:"goal_statement,omitempty"`
	InScope         []string `json:"in_scope,omitempty"`
	OutOfScope      []string `json:"out_of_scope,omitempty"`
	Assumptions     []string `json:"assumptions,omitempty"`
	OpenQuestions   []string `json:"open_questions,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
}

// MUPSWorkItemSnapshot carries WorkItem fields needed for MUPS materialize.
type MUPSWorkItemSnapshot struct {
	ID                string
	Directive         string
	ScopeIn           []string
	ScopeOut          []string
	ExpectedReturn    string
	DeliverableSchema string
	PriorVerifyReason string
	ScopeContract     *MUPSScopeContract
	Partition         MUPSPartition
}

// MUPSPartition addresses the private-chain partition for execute rounds.
type MUPSPartition struct {
	SessionID        string
	WorkItemID       string
	ParentWorkItemID string
}

// MUPSTurnContext carries session-level fields for MUPS materialize.
type MUPSTurnContext struct {
	SessionID      string
	WorkDir        string
	PermissionMode types.PermissionMode
	PlanFilePath   string
	AgentID        string
}

// MUPSContextRequest is the sole input D7 sends for MUPS LLM nodes.
type MUPSContextRequest struct {
	Phase        MUPSPhase
	PlanKind     string
	TaskKind     string
	ToolProfile  string
	AgentProfile string
	WorkItem     *MUPSWorkItemSnapshot
	Turn         *MUPSTurnContext
	Policy       MUPSContextPolicy
	// UserMessage is the directive-only payload for observe/plan user prompts.
	UserMessage string
	// ContractDimensionDoc is plan-phase deliverable_contract dimension doc (from D7).
	ContractDimensionDoc string
}

// MUPSTokenBudget summarizes token allocation for the prepared context.
type MUPSTokenBudget struct {
	MaxContextTokens int
	ReservedOutput   int
	Target           int
}

// MUPSPreparedContext is the sole output D7 consumes before LLM invoke.
type MUPSPreparedContext struct {
	SystemPrompt       string
	Messages           []types.Message
	Tools              []MUPSToolDescriptor
	TokenBudget        MUPSTokenBudget
	OutputHints        string
	PhaseAppendix      string
	UserContextPrepend map[string]string
	TokenEst           int
	MessageCount       int
}

// MUPSToolDescriptor is a minimal tool schema for D7 conversion.
type MUPSToolDescriptor struct {
	Name        string
	Description string
	Schema      map[string]any
}

// IMUPSContextMaterializer is the D2 port D7 depends on for MUPS nodes.
type IMUPSContextMaterializer interface {
	MaterializeForMUPS(ctx context.Context, req MUPSContextRequest) (MUPSPreparedContext, error)
}

var (
	// ErrPhaseNotMaterializable is returned for verify/learn/decide phases.
	ErrPhaseNotMaterializable = errors.New("mups: phase not materializable")
	// ErrWorkItemRequired is returned when execute lacks a WorkItem snapshot.
	ErrWorkItemRequired = errors.New("mups: work item required for execute")
	// ErrTokenBudgetExceeded is returned when compression cannot fit budget.
	ErrTokenBudgetExceeded = errors.New("mups: token budget exceeded")
)
