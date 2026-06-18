package workmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	ToolNameTaskWrite = "task_write"
	ToolNameTaskSpawn = "task_spawn"
	ToolNameTaskAwait = "task_await"
)

var globalUnifiedReg *toolrunner.ToolRegistry

// SetUnifiedToolRegistry wires forward targets for alias tools (call after full registration).
func SetUnifiedToolRegistry(reg *toolrunner.ToolRegistry) {
	globalUnifiedReg = reg
}

// RegisterUnifiedTaskTools registers v2.0 unified task tools (alias period).
func RegisterUnifiedTaskTools(reg *toolrunner.ToolRegistry, cfg *config.ContextEngineConfig, manager *TaskManager) error {
	if reg == nil || cfg == nil || manager == nil || cfg.Tasks.Mode != "v2" {
		return nil
	}
	for _, runner := range []toolrunner.PluginRunner{
		&taskWriteRunner{manager: manager},
		&taskSpawnRunner{manager: manager},
		&taskAwaitRunner{manager: manager},
	} {
		if err := reg.Register(runner); err != nil {
			return err
		}
	}
	return nil
}

func forwardTool(ctx context.Context, name, input string) (*toolrunner.ToolResult, error) {
	if globalUnifiedReg == nil {
		return &toolrunner.ToolResult{Error: name + ": tool registry not wired"}, nil
	}
	return globalUnifiedReg.Execute(ctx, toolrunner.ToolCall{Name: name, Input: input})
}

func withDeprecationNotice(res *toolrunner.ToolResult, notice string) *toolrunner.ToolResult {
	if res == nil {
		return res
	}
	slog.Debug("workmodel: unified tool alias", "notice", notice)
	if res.Output != "" {
		res.Output = res.Output + "\n\n" + notice
	} else if res.Error == "" {
		res.Output = notice
	}
	return res
}

type taskWriteRunner struct {
	manager *TaskManager
}

func (r *taskWriteRunner) Name() string { return ToolNameTaskWrite }
func (r *taskWriteRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskWriteRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name: ToolNameTaskWrite,
		Description: "Unified task write: mode=checklist (replaces todo_write), mode=create, mode=update. " +
			"Prefer this over legacy todo_write / task_create / task_update.",
		Parameters: `{"type":"object","required":["mode"],"properties":{"mode":{"type":"string","enum":["checklist","create","update"]},"todos":{"type":"array"},"subject":{"type":"string"},"description":{"type":"string"},"task_id":{"type":"string"},"status":{"type":"string"},"owner":{"type":"string"},"blocked_by":{"type":"string"}}}`,
	}
}

func (r *taskWriteRunner) Execute(ctx context.Context, _, input string) (*toolrunner.ToolResult, error) {
	fields := parseUnifiedInput(input)
	mode := strings.ToLower(strings.TrimSpace(fields["mode"]))
	if mode == "" {
		mode = "checklist"
	}
	switch mode {
	case "checklist":
		payload, err := json.Marshal(map[string]any{"todos": fieldsRawTodos(input)})
		if err != nil {
			return &toolrunner.ToolResult{Error: err.Error()}, nil
		}
		res, err := forwardTool(ctx, "todo_write", string(payload))
		if err != nil {
			return nil, err
		}
		return withDeprecationNotice(res, "[task_write] checklist mode is equivalent to todo_write (alias period)."), nil
	case "create":
		res, err := forwardTool(ctx, ToolNameTaskCreate, input)
		if err != nil {
			return nil, err
		}
		return withDeprecationNotice(res, "[task_write] create mode forwards to task_create."), nil
	case "update":
		res, err := forwardTool(ctx, ToolNameTaskUpdate, input)
		if err != nil {
			return nil, err
		}
		return withDeprecationNotice(res, "[task_write] update mode forwards to task_update."), nil
	default:
		return &toolrunner.ToolResult{Error: "task_write: mode must be checklist, create, or update"}, nil
	}
}

type taskSpawnRunner struct {
	manager *TaskManager
}

func (r *taskSpawnRunner) Name() string { return ToolNameTaskSpawn }
func (r *taskSpawnRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskSpawnRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name: ToolNameTaskSpawn,
		Description: "Unified spawn: kind=explore|plan|implement (replaces delegate_*). " +
			"Requires directive; optional task_id, async, worktree_slug.",
		Parameters: `{"type":"object","required":["kind","directive"],"properties":{"kind":{"type":"string","enum":["explore","plan","implement"]},"directive":{"type":"string"},"task_id":{"type":"string"},"async":{"type":"boolean"},"worktree_slug":{"type":"string"}}}`,
	}
}

func (r *taskSpawnRunner) Execute(ctx context.Context, _, input string) (*toolrunner.ToolResult, error) {
	fields := parseUnifiedInput(input)
	kind := strings.ToLower(strings.TrimSpace(fields["kind"]))
	target := "delegate_" + kind
	if kind != "explore" && kind != "plan" && kind != "implement" {
		return &toolrunner.ToolResult{Error: "task_spawn: kind must be explore, plan, or implement"}, nil
	}
	if strings.TrimSpace(fields["directive"]) == "" {
		return &toolrunner.ToolResult{Error: "task_spawn: directive is required"}, nil
	}
	res, err := forwardTool(ctx, target, input)
	if err != nil {
		return nil, err
	}
	return withDeprecationNotice(res, fmt.Sprintf("[task_spawn] kind=%s forwards to %s (alias period).", kind, target)), nil
}

type taskAwaitRunner struct {
	manager *TaskManager
}

func (r *taskAwaitRunner) Name() string { return ToolNameTaskAwait }
func (r *taskAwaitRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskAwaitRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name: ToolNameTaskAwait,
		Description: "Wait for a work item run to reach terminal state. Uses RunRegistry when run_ref is set; " +
			"falls back to task_output for legacy background tasks.",
		Parameters: `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string"},"block":{"type":"boolean"},"timeout_ms":{"type":"integer"}}}`,
	}
}

func (r *taskAwaitRunner) Execute(ctx context.Context, _, input string) (*toolrunner.ToolResult, error) {
	sessionID := toolrunner.ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &toolrunner.ToolResult{Error: "task_await: session_id unavailable"}, nil
	}
	fields := parseUnifiedInput(input)
	taskID := strings.TrimSpace(fields["task_id"])
	if taskID == "" {
		return &toolrunner.ToolResult{Error: "task_await: task_id is required"}, nil
	}
	block := fields["block"] == "true" || fields["block"] == "True"
	timeout := 600 * time.Second
	if ms, err := parseIntField(fields["timeout_ms"]); err == nil && ms > 0 {
		timeout = time.Duration(ms) * time.Millisecond
	}

	item, ok := r.manager.GetWorkItem(sessionID, taskID)
	if ok && item.RunRef != "" && runregistry.Global != nil {
		out, err := runregistry.Await(ctx, item.RunRef, block, timeout)
		if err != nil {
			return &toolrunner.ToolResult{Error: err.Error()}, nil
		}
		return &toolrunner.ToolResult{Output: out}, nil
	}
	if runID, ok := runregistry.Global.GetByWorkItem(taskID); ok && runregistry.Global != nil {
		out, err := runregistry.Await(ctx, runID, block, timeout)
		if err != nil {
			return &toolrunner.ToolResult{Error: err.Error()}, nil
		}
		return &toolrunner.ToolResult{Output: out}, nil
	}
	payload, _ := json.Marshal(map[string]any{
		"task_id":    taskID,
		"block":      block,
		"timeout_ms": int(timeout / time.Millisecond),
	})
	res, err := forwardTool(ctx, "task_output", string(payload))
	if err != nil {
		return nil, err
	}
	return withDeprecationNotice(res, "[task_await] no run_ref; fell back to task_output."), nil
}

func parseUnifiedInput(input string) map[string]string {
	fields := make(map[string]string)
	if input == "" {
		return fields
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return fields
	}
	for k, v := range raw {
		if s, ok := v.(string); ok {
			fields[k] = s
		} else if v != nil {
			fields[k] = fmt.Sprint(v)
		}
	}
	return fields
}

func fieldsRawTodos(input string) []types.TodoItem {
	var raw struct {
		Todos []types.TodoItem `json:"todos"`
	}
	_ = json.Unmarshal([]byte(input), &raw)
	return raw.Todos
}

func parseIntField(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
