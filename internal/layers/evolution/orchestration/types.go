package orchestration

import (
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
)

// DecisionCategory classifies the type of routing decision.
type DecisionCategory string

const (
	DecisionToolCall DecisionCategory = "tool_call"
	DecisionPermit   DecisionCategory = "permit"
	DecisionFork     DecisionCategory = "fork"
)

// RiskClass represents the risk level of a decision.
type RiskClass int

const (
	RiskLow      RiskClass = 0
	RiskEvaluate RiskClass = 1
	RiskCritical RiskClass = 2
)

// DecisionRecord captures a routing decision made by an agent.
type DecisionRecord struct {
	ID            string
	SessionID     string
	AgentID       string
	ParentAgentID string
	Category      DecisionCategory
	RiskClass     RiskClass
	Timestamp     time.Time
	ToolName      string
	ToolInput     string
	TargetAgentID string
	ForkConfig    *multiagent.AgentConfig
	RecentMessages []types.Message
	SessionState  types.SessionState
}

// ValidationResult is the judge's verdict on a decision.
type ValidationResult struct {
	DecisionID       string
	Reasoning        string
	Valid            bool
	Confidence       float64
	SuggestedAction  string // "none" | "terminate" | "reroute" | "deny"
	SuggestedAgentID string
	JudgeModel       string
	Duration         time.Duration
}

// Intervention describes a corrective action to apply.
type Intervention struct {
	DecisionID    string
	Action        string // "terminate" | "reroute" | "update_state"
	Reason        string
	TargetAgentID string
	AgentConfig   *multiagent.AgentConfig
	TaskFail      bool
	MilestoneFail bool
	FailReason    string
}
