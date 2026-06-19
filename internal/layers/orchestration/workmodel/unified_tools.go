package workmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	ToolNameTaskWrite = "task_write"
	ToolNameTaskSpawn = "task_spawn"
	ToolNameTaskAwait = "task_await"
)

var globalUnifiedReg *tools.ToolRegistry

// SetUnifiedToolRegistry wires forward targets for alias tools (call after full registration).
func SetUnifiedToolRegistry(reg *tools.ToolRegistry) {
	globalUnifiedReg = reg
}

// RegisterUnifiedTaskTools registers v2.0 unified task tools (alias period).
func RegisterUnifiedTaskTools(reg *tools.ToolRegistry, cfg *config.ContextEngineConfig, manager *TaskManager) error {
	if reg == nil || cfg == nil || manager == nil || cfg.Tasks.Mode != "v2" {
		return nil
	}
	for _, runner := range []tools.PluginRunner{
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

func forwardTool(ctx context.Context, name, input string) (*tools.ToolResult, error) {
	if globalUnifiedReg == nil {
		return &tools.ToolResult{Error: name + ": tool registry not wired"}, nil
	}
	return globalUnifiedReg.Execute(ctx, tools.ToolCall{Name: name, Input: input})
}

func withDeprecationNotice(res *tools.ToolResult, notice string) *tools.ToolResult {
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

func (r *taskWriteRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name: ToolNameTaskWrite,
		Description: "Unified task write: mode=checklist (replaces todo_write), mode=create, mode=update, mode=decompose. " +
			"Prefer this over legacy todo_write / task_create / task_update.",
		Parameters: `{"type":"object","required":["mode"],"properties":{"mode":{"type":"string","enum":["checklist","create","update","decompose"]},"todos":{"type":"array"},"children":{"type":"array","items":{"type":"object","properties":{"title":{"type":"string"},"directive":{"type":"string"},"kind":{"type":"string"}}}},"parent_id":{"type":"string"},"subject":{"type":"string"},"description":{"type":"string"},"task_id":{"type":"string"},"status":{"type":"string"},"owner":{"type":"string"},"blocked_by":{"type":"string"}}}`,
	}
}

func (r *taskWriteRunner) Execute(ctx context.Context, _, input string) (*tools.ToolResult, error) {
	sessionID := tools.ToolSessionIDFromContext(ctx)
	fields := parseUnifiedInput(input)
	mode := strings.ToLower(strings.TrimSpace(fields["mode"]))
	if mode == "" {
		mode = "checklist"
	}
	switch mode {
	case "decompose":
		return r.executeDecompose(ctx, sessionID, input)
	case "checklist":
		payload, err := json.Marshal(map[string]any{"todos": fieldsRawTodos(input)})
		if err != nil {
			return &tools.ToolResult{Error: err.Error()}, nil
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
		return &tools.ToolResult{Error: "task_write: mode must be checklist, create, update, or decompose"}, nil
	}
}

func (r *taskWriteRunner) executeDecompose(ctx context.Context, sessionID, input string) (*tools.ToolResult, error) {
	if sessionID == "" {
		return &tools.ToolResult{Error: "task_write: session_id unavailable"}, nil
	}
	specs, parentID, err := parseDecomposeInput(input)
	if err != nil {
		return &tools.ToolResult{Error: err.Error()}, nil
	}
	if parentID == "" {
		focus, err := ResolveFocus(sessionID, r.manager)
		if err != nil || focus == nil {
			return &tools.ToolResult{Error: "task_write: parent_id required when no focus"}, nil
		}
		parentID = focus.ID
	}
	items, err := r.manager.DecomposeChildren(sessionID, parentID, specs)
	if err != nil {
		return &tools.ToolResult{Error: err.Error()}, nil
	}
	data, _ := json.Marshal(map[string]any{"parent_id": parentID, "children": items})
	return &tools.ToolResult{Output: string(data)}, nil
}

func parseDecomposeInput(input string) ([]ChildSpec, string, error) {
	var raw struct {
		ParentID string `json:"parent_id"`
		Children []struct {
			Title     string `json:"title"`
			Directive string `json:"directive"`
			Kind      string `json:"kind"`
		} `json:"children"`
	}
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil, "", fmt.Errorf("task_write decompose: invalid json")
	}
	if len(raw.Children) == 0 {
		return nil, "", fmt.Errorf("task_write decompose: children required")
	}
	specs := make([]ChildSpec, 0, len(raw.Children))
	for _, c := range raw.Children {
		specs = append(specs, ChildSpec{
			Kind:      ResolveFocusKind(c.Kind),
			Title:     c.Title,
			Directive: c.Directive,
		})
	}
	return specs, raw.ParentID, nil
}

type taskSpawnRunner struct {
	manager *TaskManager
}

func (r *taskSpawnRunner) Name() string { return ToolNameTaskSpawn }
func (r *taskSpawnRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskSpawnRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name: ToolNameTaskSpawn,
		Description: "Unified spawn: kind=explore|plan|implement (replaces delegate_*). " +
			"Requires directive; optional task_id, async, worktree_slug.",
		Parameters: `{"type":"object","required":["kind","directive"],"properties":{"kind":{"type":"string","enum":["explore","plan","implement"]},"directive":{"type":"string"},"task_id":{"type":"string"},"async":{"type":"boolean"},"worktree_slug":{"type":"string"}}}`,
	}
}

func (r *taskSpawnRunner) Execute(ctx context.Context, _, input string) (*tools.ToolResult, error) {
	fields := parseUnifiedInput(input)
	kind := strings.ToLower(strings.TrimSpace(fields["kind"]))
	target := "delegate_" + kind
	if kind != "explore" && kind != "plan" && kind != "implement" {
		return &tools.ToolResult{Error: "task_spawn: kind must be explore, plan, or implement"}, nil
	}
	if strings.TrimSpace(fields["directive"]) == "" {
		return &tools.ToolResult{Error: "task_spawn: directive is required"}, nil
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

func (r *taskAwaitRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name: ToolNameTaskAwait,
		Description: "Wait for a work item run to reach terminal state. Uses RunRegistry when run_ref is set; " +
			"falls back to task_output for legacy background tasks.",
		Parameters: `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string"},"block":{"type":"boolean"},"timeout_ms":{"type":"integer"}}}`,
	}
}

func (r *taskAwaitRunner) Execute(ctx context.Context, _, input string) (*tools.ToolResult, error) {
	sessionID := tools.ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &tools.ToolResult{Error: "task_await: session_id unavailable"}, nil
	}
	fields := parseUnifiedInput(input)
	taskID := strings.TrimSpace(fields["task_id"])
	if taskID == "" {
		return &tools.ToolResult{Error: "task_await: task_id is required"}, nil
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
			return &tools.ToolResult{Error: err.Error()}, nil
		}
		return &tools.ToolResult{Output: out}, nil
	}
	if runID, ok := runregistry.Global.GetByWorkItem(taskID); ok && runregistry.Global != nil {
		out, err := runregistry.Await(ctx, runID, block, timeout)
		if err != nil {
			return &tools.ToolResult{Error: err.Error()}, nil
		}
		return &tools.ToolResult{Output: out}, nil
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
