package materialize

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// PartitionKind identifies a context partition (DM-20260627-003).
type PartitionKind string

const (
	PartitionSession PartitionKind = "session"
	PartitionCohort  PartitionKind = "cohort"
	PartitionWorkItem PartitionKind = "workitem"
	PartitionAgent   PartitionKind = "agent"
	PartitionWave    PartitionKind = "wave"
)

// Mode selects materialize composition policy.
type Mode string

const (
	ModeFresh        Mode = "fresh"
	ModeInheritCohort Mode = "inherit_cohort"
	ModeUpstream     Mode = "upstream"
	ModeResume       Mode = "resume"
	ModeRollupSynth  Mode = "rollup_synth"
)

// Partition addresses one context bucket.
type Partition struct {
	SessionID        string
	Kind             PartitionKind
	ParentWorkItemID string
	WorkItemID       string
	AgentID          string
	ParentSessionID  string // wave resume sidechain session
}

// Policy controls token budget and tool profile.
type Policy struct {
	Mode         Mode
	TokenBudget  int
	ToolProfile  string
	Locale       string
	AgentProfile string
	Depth        int
}

// MaterializePolicy is the MUPS-facing alias for Policy (DM-20260704-001).
type MaterializePolicy = Policy

// InboundSignals are D7 signals injected at materialize time (not full transcripts).
type InboundSignals struct {
	Directive       string
	ScopeIn         []string
	ScopeOut        []string
	ExpectedReturn  string
	ChildDownlink   bool
	SignalLines     []string
	// Wave worker fields (D7-S16 T34).
	WaveExtraPrompt   string
	WaveFileScope     []string
	WaveUpstreamFiles []string
	WaveUpstreamError string
}

// Request is the D2 Materialize input.
type Request struct {
	Partition     Partition
	Policy        Policy
	Signals       InboundSignals
	SystemPrompt  string          // optional override (SubTurn / Wave worker)
	SubTurnParent []types.Message // parent history for SubTurn fork/full modes
}

// Result is LLM-ready context from D2.
type Result struct {
	SystemPrompt string
	Messages     []types.Message
	Tools        []ToolDescriptor
	MessageCount int
	TokenEst     int
}

// ToolDescriptor is a minimal tool schema for D7 conversion.
type ToolDescriptor struct {
	Name        string
	Description string
	Schema      map[string]any
}

// Materializer assembles partitioned context for WorkItem execute.
type Materializer interface {
	Materialize(ctx context.Context, req Request) (Result, error)
	Append(ctx context.Context, partition Partition, msgs []types.Message) error
}
