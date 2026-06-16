package types

import "time"

// ContextSnapshotVersion is the JSON snapshot format version.
const ContextSnapshotVersion = "ctx-v1"

// ToolCallRecord records a single tool invocation.
type ToolCallRecord struct {
	CallID    string    `json:"callId,omitempty"`
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
	CompressionTarget int // autocompact 触发阈值（LLM 摘要）
	SnipTarget        int // snip 触发阈值（逐出旧消息，高于 autocompact）
}

// DefaultTokenBudget returns V1 default token budget.
func DefaultTokenBudget() TokenBudget {
	max := 128000
	reserved := 8192
	return TokenBudget{
		MaxContextTokens:  max,
		ReservedOutput:    reserved,
		ToolResultBudget:  800,
		CompressionTarget: int(float64(max-reserved) * 0.6),
		SnipTarget:        int(float64(max-reserved) * 0.8),
	}
}

// CompressionReport describes a single compression run.
type CompressionReport struct {
	OriginalTokens   int
	CompressedTokens int
	StepsApplied     []string
	Truncated        bool
}

// Ratio returns compressed/original token ratio in [0,1]; returns 1 when original is zero.
func (r CompressionReport) Ratio() float64 {
	if r.OriginalTokens <= 0 {
		return 1
	}
	return float64(r.CompressedTokens) / float64(r.OriginalTokens)
}

// TodoStatus represents a task status for todo_write tool.
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusCompleted  TodoStatus = "completed"
)

// TodoItem is a single task tracked by todo_write.
type TodoItem struct {
	Content    string     `json:"content"`
	Status     TodoStatus `json:"status"`
	ActiveForm string     `json:"activeForm"`
}

// VerifState tracks verification nudges.
type VerifState struct {
	VerifTriggered          bool `json:"verifTriggered"`
	CompletedSinceLastVerif int  `json:"completedSinceLastVerif"`
}

// SessionContext is the context engine aggregate root (in-memory).
type SessionContext struct {
	SessionID      string
	WorkDir        string
	Model          string
	ModelTier      string
	Messages       []Message
	CompressedView []Message
	TokenBudget    TokenBudget
	SystemPrompt   string
	LastRequestID  string
	UpdatedAt      time.Time
	// QueryLoop / permission state (Claude Code aligned).
	PermissionMode PermissionMode
	PrePlanMode    PermissionMode
	PlanFilePath   string
	AgentID        string
	QueryChainID   string
	QueryDepth     int
	IsWorker       bool
	WorkerRole     string
	// TodoWrite tracking.
	Todos      []TodoItem `json:"todos,omitempty"`
	VerifState VerifState `json:"verifState,omitempty"`
}

// ContextSnapshotV1 is the persisted snapshot format.
type ContextSnapshotV1 struct {
	Version      string              `json:"version"`
	SessionID    string              `json:"sessionId"`
	Model        string              `json:"model"`
	WorkDir      string              `json:"workDir"`
	Messages     []MessageSnapshot   `json:"messages"`
	TokenBudget  TokenBudgetSnapshot `json:"tokenBudget"`
	SystemPrompt string              `json:"systemPrompt"`
	UpdatedAt    string              `json:"updatedAt"`
	Todos        []TodoItem          `json:"todos,omitempty"`
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

