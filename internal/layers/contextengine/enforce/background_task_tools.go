package enforce

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/types"
)

// BackgroundTaskToolsDeps wires dependencies for task_stop / task_output.
// All fields are optional; the tools fall back to GlobalBackgroundRegistry
// when nil so the engine bootstrap stays simple.
type BackgroundTaskToolsDeps struct {
	Registry *BackgroundRegistry
	Waiter   *BackgroundWaiter
}

// globalBackgroundTaskTools is the singleton dependency holder for the
// background task LLM tools. It is configured by SetBackgroundTaskToolsDeps
// at engine bootstrap, paralleling delegatetools.SetDeps.
var globalBackgroundTaskTools BackgroundTaskToolsDeps

// SetBackgroundTaskToolsDeps configures task_stop / task_output handlers.
func SetBackgroundTaskToolsDeps(deps BackgroundTaskToolsDeps) {
	if deps.Registry == nil {
		deps.Registry = GlobalBackgroundRegistry
	}
	if deps.Waiter == nil && deps.Registry != nil {
		deps.Waiter = NewBackgroundWaiter(deps.Registry)
	}
	globalBackgroundTaskTools = deps
}

// RegisterBackgroundTaskTools registers task_stop / task_output as LLM tools.
// Safe to call when turn runtime is active. No-op if registry is nil.
func RegisterBackgroundTaskTools(reg *tools.ToolRegistry) error {
	if reg == nil {
		return nil
	}
	for _, runner := range []tools.PluginRunner{
		newTaskStopRunner(),
		newTaskOutputRunner(),
		newTaskListBackgroundRunner(),
	} {
		if err := reg.Register(runner); err != nil {
			return fmt.Errorf("register %s: %w", runner.Name(), err)
		}
	}
	return nil
}

// --- task_stop ----------------------------------------------------------------

type taskStopRunner struct{}

func newTaskStopRunner() *taskStopRunner { return &taskStopRunner{} }

func (r *taskStopRunner) Name() string { return "task_stop" }

func (r *taskStopRunner) RiskLevel() types.RiskLevel { return types.RiskLevelMedium }

func (r *taskStopRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "task_stop",
		Description: "Cancel a running background SubQuery by task_id (idempotent; returns the previous status).",
		Parameters:  `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string","description":"Background task id returned by delegate_explore/plan/implement with async=true or by RunBackground."}}}`,
	}
}

func (r *taskStopRunner) Execute(ctx context.Context, _, input string) (*tools.ToolResult, error) {
	sessionID := tools.ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &tools.ToolResult{Error: "task_stop: session_id unavailable"}, nil
	}
	fields := tools.ParseToolInput(input)
	taskID := strings.TrimSpace(fields["task_id"])
	if taskID == "" {
		return &tools.ToolResult{Error: "task_stop: task_id is required"}, nil
	}
	reg := backgroundRegistry()
	if reg == nil {
		return &tools.ToolResult{Error: "task_stop: background registry not initialized"}, nil
	}
	task, ok := reg.Get(taskID)
	if !ok {
		return &tools.ToolResult{Error: fmt.Sprintf("task_stop: unknown task_id %s", taskID)}, nil
	}
	if task.SessionID != sessionID {
		return &tools.ToolResult{Error: "task_stop: task belongs to a different session"}, nil
	}
	prev := task.Status
	cancelled := reg.Cancel(taskID)
	out := map[string]any{
		"task_id":     taskID,
		"cancelled":   cancelled,
		"prev_status": prev,
		"new_status":  "cancelled",
	}
	if !cancelled && prev != "running" {
		out["new_status"] = prev
	}
	data, _ := json.Marshal(out)
	return &tools.ToolResult{Output: string(data)}, nil
}

// --- task_output --------------------------------------------------------------

type taskOutputRunner struct{}

func newTaskOutputRunner() *taskOutputRunner { return &taskOutputRunner{} }

func (r *taskOutputRunner) Name() string { return "task_output" }

func (r *taskOutputRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskOutputRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "task_output",
		Description: "Read the status/output of a background SubQuery. block=true waits until the task reaches a terminal state (max 600s).",
		Parameters: `{"type":"object","required":["task_id"],"properties":{
			"task_id":{"type":"string"},
			"block":{"type":"boolean","default":false},
			"timeout_ms":{"type":"integer","default":30000,"minimum":1,"maximum":600000}
		}}`,
	}
}

func (r *taskOutputRunner) Execute(ctx context.Context, _, input string) (*tools.ToolResult, error) {
	sessionID := tools.ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &tools.ToolResult{Error: "task_output: session_id unavailable"}, nil
	}
	fields := tools.ParseToolInput(input)
	taskID := strings.TrimSpace(fields["task_id"])
	if taskID == "" {
		return &tools.ToolResult{Error: "task_output: task_id is required"}, nil
	}
	block := parseBoolField(fields["block"])
	timeout := parseTimeoutMs(fields["timeout_ms"], 30*time.Second)

	reg := backgroundRegistry()
	if reg == nil {
		return &tools.ToolResult{Error: "task_output: background registry not initialized"}, nil
	}
	waiter := backgroundWaiter()
	if waiter == nil {
		waiter = NewBackgroundWaiter(reg)
	}

	task, ok := reg.Get(taskID)
	if !ok {
		return &tools.ToolResult{Error: fmt.Sprintf("task_output: unknown task_id %s", taskID)}, nil
	}
	if task.SessionID != sessionID {
		return &tools.ToolResult{Error: "task_output: task belongs to a different session"}, nil
	}

	if block && task.Status == "running" {
		waiter.Register(taskID)
		_, _ = waiter.Wait(taskID, timeout)
		task, _ = reg.Get(taskID)
	}

	out := map[string]any{
		"task_id":    taskID,
		"agent_id":   task.AgentID,
		"agent_name": task.AgentName,
		"status":     task.Status,
		"output":     task.Result,
		"error":      task.Error,
		"started_at": task.StartedAt,
		"ended_at":   task.EndedAt,
	}
	data, _ := json.Marshal(out)
	return &tools.ToolResult{Output: string(data)}, nil
}

// --- task_list_background (P1) ----------------------------------------------

type taskListBackgroundRunner struct{}

func newTaskListBackgroundRunner() *taskListBackgroundRunner { return &taskListBackgroundRunner{} }

func (r *taskListBackgroundRunner) Name() string { return "task_list_background" }

func (r *taskListBackgroundRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskListBackgroundRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "task_list_background",
		Description: "List all background SubQuery tasks for the current session (running + terminal). task_id prefix 'bg_' identifies background SubQuery; 'task_' prefix is for Plan tasks.",
		Parameters:  `{"type":"object","properties":{}}`,
	}
}

func (r *taskListBackgroundRunner) Execute(ctx context.Context, _, _ string) (*tools.ToolResult, error) {
	sessionID := tools.ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &tools.ToolResult{Error: "task_list_background: session_id unavailable"}, nil
	}
	reg := backgroundRegistry()
	if reg == nil {
		return &tools.ToolResult{Error: "task_list_background: background registry not initialized"}, nil
	}
	tasks := reg.List(sessionID)
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, map[string]any{
			"task_id":    t.ID,
			"agent_id":   t.AgentID,
			"agent_name": t.AgentName,
			"status":     t.Status,
			"started_at": t.StartedAt,
			"ended_at":   t.EndedAt,
		})
	}
	data, _ := json.Marshal(map[string]any{"tasks": out, "count": len(out)})
	return &tools.ToolResult{Output: string(data)}, nil
}

// --- helpers ------------------------------------------------------------------

func backgroundRegistry() *BackgroundRegistry {
	if globalBackgroundTaskTools.Registry != nil {
		return globalBackgroundTaskTools.Registry
	}
	return GlobalBackgroundRegistry
}

func backgroundWaiter() *BackgroundWaiter {
	return globalBackgroundTaskTools.Waiter
}

func parseBoolField(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

func parseTimeoutMs(s string, fallback time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	ms, err := strconv.Atoi(s)
	if err != nil || ms <= 0 {
		return fallback
	}
	d := time.Duration(ms) * time.Millisecond
	if d > 600*time.Second {
		return 600 * time.Second
	}
	return d
}
