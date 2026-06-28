package contracts

import "context"

// WorkerKind identifies a worker for IM presentation (string closed set).
// Mirrors D7 wavescheduler worker kinds without importing orchestration packages.
type WorkerKind string

const (
	WorkerKindCursor     WorkerKind = "cursor"
	WorkerKindClaudeCode WorkerKind = "claude_code"
	WorkerKindSubAgent   WorkerKind = "subagent"
	WorkerKindWorkItem   WorkerKind = "workitem"
)

// Valid reports whether the worker kind is recognized for presentation.
func (k WorkerKind) Valid() bool {
	switch k {
	case WorkerKindCursor, WorkerKindClaudeCode, WorkerKindSubAgent, WorkerKindWorkItem:
		return true
	default:
		return false
	}
}

// WorkerStreamEvent is a streaming chunk consumed by S17 worker card renderers.
type WorkerStreamEvent struct {
	Type      string // thinking|text|tool_use|error|complete|cancelled
	Content   string
	ToolName  string
	ToolInput string
}

// WorkerCardOpts configures per-worker Feishu double-block card creation (S17-F02).
type WorkerCardOpts struct {
	SessionID  string
	TaskID     string
	WorkerID   string
	WorkerKind WorkerKind
	Title      string
}

// WorkerProgressView is a presentation DTO for worker progress encode paths.
type WorkerProgressView struct {
	SessionID, TaskID, WorkerID string
	WorkerKind                  WorkerKind
	Title, Status               string
	ThinkingDelta, OutputDelta  string
}

// CLICommandRequest carries a raw CLI side-channel command line.
// Parsing and execution remain in D7 until Phase 3b full CommandHandler migration.
type CLICommandRequest struct {
	RawLine string
}

// TaskCLIHandler handles /task side-channel commands for CLI adapters.
type TaskCLIHandler interface {
	HandleTaskCommand(rawLine string, sessionID string) string
}

// PlanCLIHandler handles /plan side-channel commands for CLI adapters.
type PlanCLIHandler interface {
	HandlePlanCommand(args []string, sessionID, workDir string) string
}

// PlanLLMCompleter completes prompts for plan-mode CLI flows.
type PlanLLMCompleter interface {
	Complete(ctx context.Context, prompt string) (string, error)
}
