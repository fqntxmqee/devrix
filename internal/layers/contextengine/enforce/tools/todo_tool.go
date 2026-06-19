package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/devrix/devrix/internal/shared/types"
)

var verifPattern = regexp.MustCompile(`(?i)verif`)

type todoWriteInput struct {
	Todos []types.TodoItem `json:"todos"`
}

type todoWriteOutput struct {
	OldTodos               []types.TodoItem `json:"oldTodos"`
	NewTodos               []types.TodoItem `json:"newTodos"`
	VerificationNudgeNeeded bool            `json:"verificationNudgeNeeded,omitempty"`
	NudgeMessage           string           `json:"nudgeMessage,omitempty"`
}

type todoWriteRunner struct{}

// TodoSyncFunc syncs todos to WorkTree; returns projected todos for session context.
type TodoSyncFunc func(sessionID string, todos []types.TodoItem) []types.TodoItem

var globalTodoSync TodoSyncFunc

// SetTodoSync wires D7 WorkTree sync from bootstrap.
func SetTodoSync(fn TodoSyncFunc) {
	globalTodoSync = fn
}

func NewTodoWriteRunner() *todoWriteRunner {
	return &todoWriteRunner{}
}

func (r *todoWriteRunner) Name() string { return "todo_write" }

func (r *todoWriteRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "todo_write",
		Description: "Manage the session task checklist with full-snapshot replacement. Track progress across pending, in_progress, and completed items.",
		Parameters:  `{"type":"object","required":["todos"],"properties":{"todos":{"type":"array","items":{"type":"object","required":["content","status","activeForm"],"properties":{"content":{"type":"string"},"status":{"type":"string","enum":["pending","in_progress","completed"]},"activeForm":{"type":"string"}}}}}}`,
	}
}

func (r *todoWriteRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *todoWriteRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &ToolResult{Error: "todo_write: session context unavailable"}, nil
	}

	var req todoWriteInput
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return &ToolResult{Error: fmt.Sprintf("todo_write: invalid input: %s", err)}, nil
	}

	oldTodos := make([]types.TodoItem, len(sc.Todos))
	copy(oldTodos, sc.Todos)

	// Count newly completed items for verification nudge tracking
	newCompleted := 0
	for _, t := range req.Todos {
		if t.Status == types.TodoStatusCompleted {
			newCompleted++
		}
	}

	// Full-snapshot replacement (+ optional WorkTree sync)
	if len(req.Todos) == 0 {
		sc.Todos = nil
	} else {
		sc.Todos = req.Todos
	}
	if globalTodoSync != nil && sc.SessionID != "" {
		sc.Todos = globalTodoSync(sc.SessionID, sc.Todos)
	}

	// Update verification state
	sc.VerifState.CompletedSinceLastVerif += newCompleted

	verificationNudgeNeeded := false
	nudgeMessage := ""
	if sc.VerifState.CompletedSinceLastVerif >= 3 {
		hasVerif := false
		for _, t := range req.Todos {
			if t.Status == types.TodoStatusCompleted && verifPattern.MatchString(t.Content) {
				hasVerif = true
				break
			}
		}
		if !hasVerif {
			verificationNudgeNeeded = true
			nudgeMessage = "You have completed 3+ tasks without a verification step. Consider running a verification agent or using /verify to validate completeness and quality before concluding."
		}
		// Reset counter regardless of whether nudge fires (prevents repeated nudges)
		sc.VerifState.CompletedSinceLastVerif = 0
	}
	if verificationNudgeNeeded {
		sc.VerifState.VerifTriggered = true
	}

	out := todoWriteOutput{
		OldTodos:                oldTodos,
		NewTodos:                sc.Todos,
		VerificationNudgeNeeded: verificationNudgeNeeded,
		NudgeMessage:           nudgeMessage,
	}
	outData, _ := json.Marshal(out)

	resultText := fmt.Sprintf("Todo list updated: %d item(s) (old: %d item(s))", len(sc.Todos), len(oldTodos))
	if verificationNudgeNeeded {
		resultText += "\n\n" + nudgeMessage
	}

	output := string(outData)
	if output == "" {
		output = resultText
	}
	if strings.TrimSpace(output) == "" {
		output = resultText
	}

	return &ToolResult{Output: output}, nil
}
