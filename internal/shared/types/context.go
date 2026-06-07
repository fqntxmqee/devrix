package types

import "time"

// ContextSnapshotVersion is the JSON snapshot format version.
const ContextSnapshotVersion = "ctx-v1"

// PEVPhase represents the current PEV loop phase.
type PEVPhase string

const (
	PEVPhasePlan    PEVPhase = "plan"
	PEVPhaseExecute PEVPhase = "execute"
	PEVPhaseVerify  PEVPhase = "verify"
	PEVPhaseDone    PEVPhase = "done"
)

// VerifyResult holds PEV verification outcome (value type; zero = not verified).
type VerifyResult struct {
	Passed    bool
	Deviation float64
	Commands  []string
}

// ToolCallRecord records a single tool invocation.
type ToolCallRecord struct {
	ToolName  string    `json:"toolName"`
	Input     string    `json:"input"`
	Output    string    `json:"output,omitempty"`
	RiskLevel RiskLevel `json:"riskLevel,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// TokenBudget holds context token limits.
type TokenBudget struct {
	MaxContextTokens  int
	ReservedOutput    int
	ToolResultBudget  int
	CompressionTarget int
}

// DefaultTokenBudget returns V1 default token budget.
func DefaultTokenBudget() TokenBudget {
	max := 128000
	reserved := 8192
	return TokenBudget{
		MaxContextTokens:  max,
		ReservedOutput:    reserved,
		ToolResultBudget:  800,
		CompressionTarget: int(float64(max-reserved) * 0.9),
	}
}

// PEVState holds PEV loop state.
type PEVState struct {
	Phase              PEVPhase
	Iteration          int
	MaxIterations      int
	ActiveTaskID       string
	ActiveMilestoneID  string
	LastToolCalls      []ToolCallRecord
	VerifyResult       VerifyResult
}

// DefaultPEVState returns initial PEV state.
func DefaultPEVState(maxIterations int) PEVState {
	if maxIterations <= 0 {
		maxIterations = 3
	}
	return PEVState{
		Phase:         PEVPhaseDone,
		MaxIterations: maxIterations,
	}
}

// CompressionReport describes a single compression run.
type CompressionReport struct {
	OriginalTokens   int
	CompressedTokens int
	StepsApplied     []string
	Truncated        bool
}

// SessionContext is the context engine aggregate root (in-memory).
type SessionContext struct {
	SessionID      string
	WorkDir        string
	Model          string
	Messages       []Message
	CompressedView []Message
	PEVState       PEVState
	TokenBudget    TokenBudget
	SystemPrompt   string
	MilestoneRefs  []string
	LastRequestID  string
	UpdatedAt      time.Time
}

// ContextSnapshotV1 is the persisted snapshot format.
type ContextSnapshotV1 struct {
	Version      string              `json:"version"`
	SessionID    string              `json:"sessionId"`
	Model        string              `json:"model"`
	WorkDir      string              `json:"workDir"`
	Messages     []MessageSnapshot   `json:"messages"`
	TokenBudget  TokenBudgetSnapshot `json:"tokenBudget"`
	PEVState     PEVStateSnapshot    `json:"pevState"`
	SystemPrompt string              `json:"systemPrompt"`
	UpdatedAt    string              `json:"updatedAt"`
}

// MessageSnapshot is a serializable message.
type MessageSnapshot struct {
	ID        string            `json:"id"`
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// TokenBudgetSnapshot is a serializable token budget.
type TokenBudgetSnapshot struct {
	MaxContextTokens  int `json:"maxContextTokens"`
	ReservedOutput    int `json:"reservedOutput"`
	ToolResultBudget  int `json:"toolResultBudget"`
	CompressionTarget int `json:"compressionTarget"`
}

// PEVStateSnapshot is a serializable PEV state.
type PEVStateSnapshot struct {
	Phase         string             `json:"phase"`
	Iteration     int                `json:"iteration"`
	MaxIterations int                `json:"maxIterations"`
	LastToolCalls []ToolCallRecord   `json:"lastToolCalls"`
	VerifyResult  VerifyResultSnapshot `json:"verifyResult"`
}

// VerifyResultSnapshot is a serializable verify result.
type VerifyResultSnapshot struct {
	Passed    bool     `json:"passed"`
	Deviation float64  `json:"deviation"`
	Commands  []string `json:"commands,omitempty"`
}
