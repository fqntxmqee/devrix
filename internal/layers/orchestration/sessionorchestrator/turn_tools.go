package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

const (
	toolDelegateWave  = "delegate_wave"
	toolEnterPlanMode = "enter_plan_mode"
)

// TurnToolExecutor handles loop-first orchestration tools and delegates all
// other tool calls to the base executor (D2 tool runner).
type TurnToolExecutor struct {
	Base        ToolRoundExecutor
	Orchestrate *OrchestratePath
	PlanMode    *workmodel.PlanMode
	LoopFirst   bool
	Metrics     *TurnToolMetrics

	DelegateWaveCount atomic.Int64
}

// NewTurnToolExecutor wraps base with orchestration tools when loopFirst is true.
func NewTurnToolExecutor(
	base ToolRoundExecutor,
	op *OrchestratePath,
	pm *workmodel.PlanMode,
	loopFirst bool,
) *TurnToolExecutor {
	return &TurnToolExecutor{
		Base:        base,
		Orchestrate: op,
		PlanMode:    pm,
		LoopFirst:   loopFirst,
	}
}

// SetTurnToolMetrics attaches observability counters (optional).
func (e *TurnToolExecutor) SetTurnToolMetrics(m *TurnToolMetrics) {
	if e != nil {
		e.Metrics = m
	}
}

func (e *TurnToolExecutor) ExecuteRound(ctx context.Context, req ToolRoundRequest) (ToolRoundResult, error) {
	if e == nil || e.Base == nil {
		return ToolRoundResult{}, fmt.Errorf("turn tool executor: base is nil")
	}
	if !e.LoopFirst {
		return e.Base.ExecuteRound(ctx, req)
	}

	results := make([]ToolResult, len(req.ToolCalls))
	for i, tc := range req.ToolCalls {
		switch tc.Name {
		case toolDelegateWave:
			out, err := e.runDelegateWave(ctx, req.SessionID, tc.Input)
			results[i] = ToolResult{ToolCallID: tc.ID, Output: out, Error: errString(err)}
		case toolEnterPlanMode:
			out, err := e.runEnterPlanMode(ctx, req.SessionID, tc.Input)
			results[i] = ToolResult{ToolCallID: tc.ID, Output: out, Error: errString(err)}
		default:
			single, err := e.Base.ExecuteRound(ctx, ToolRoundRequest{
				SessionID: req.SessionID,
				ToolCalls: []llmgateway.ToolCall{tc},
			})
			if err != nil {
				return ToolRoundResult{}, err
			}
			if len(single.Results) > 0 {
				results[i] = single.Results[0]
			}
		}
	}
	return ToolRoundResult{Results: results}, nil
}

func (e *TurnToolExecutor) runDelegateWave(ctx context.Context, sessionID, input string) (string, error) {
	if e.Orchestrate == nil {
		return "", fmt.Errorf("delegate_wave: orchestrate path not wired")
	}
	goal, err := parseGoalInput(input)
	if err != nil {
		return "", err
	}
	e.DelegateWaveCount.Add(1)
	if e.Metrics != nil && e.Metrics.DelegateWave != nil {
		e.Metrics.DelegateWave.Inc()
	}
	emit := ToolEventStreamFrom(ctx)
	ch, err := e.Orchestrate.Run(ctx, orchtypes.ProcessRequest{SessionID: sessionID, Message: goal}, orchtypes.IntentClassification{})
	if err != nil {
		return "", err
	}
	var summary string
	for ev := range ch {
		if ev == nil {
			continue
		}
		if emit != nil {
			emit(ev)
		}
		if ev.Type == "text" && ev.Content != "" {
			summary = ev.Content
		}
	}
	if summary == "" {
		summary = "(wave completed with no summary text)"
	}
	return summary, nil
}

func (e *TurnToolExecutor) runEnterPlanMode(ctx context.Context, sessionID, input string) (string, error) {
	if e.PlanMode == nil {
		return "", fmt.Errorf("enter_plan_mode: plan mode not wired")
	}
	goal, err := parseGoalInput(input)
	if err != nil {
		return "", err
	}
	if err := e.PlanMode.Enter(ctx, sessionID, goal); err != nil {
		return "", err
	}
	display := e.PlanMode.GetDisplayPlan()
	if display == "" {
		display = "orchtypes.Plan mode entered."
	}
	return display, nil
}

func parseGoalInput(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("tool input: goal is required")
	}
	var payload struct {
		Goal string `json:"goal"`
	}
	if err := json.Unmarshal([]byte(input), &payload); err == nil && strings.TrimSpace(payload.Goal) != "" {
		return strings.TrimSpace(payload.Goal), nil
	}
	return input, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TurnPrepareWrapper adds orchestration tool schemas when loop-first is active.
type TurnPrepareWrapper struct {
	Inner     ContextPreparer
	LoopFirst bool
}

func (w *TurnPrepareWrapper) Prepare(ctx context.Context, req PrepareRequest) (PreparedContext, error) {
	pc, err := w.Inner.Prepare(ctx, req)
	if err != nil {
		return pc, err
	}
	if w.LoopFirst {
		pc.Tools = append(pc.Tools, orchestrationToolSchemas()...)
	}
	return pc, nil
}

func orchestrationToolSchemas() []ToolSchema {
	return []ToolSchema{
		{
			Name:        toolEnterPlanMode,
			Description: "Enter plan mode when the implementation approach is ambiguous and user approval is needed before coding.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"goal": map[string]any{"type": "string", "description": "The user goal to plan for"},
				},
				"required": []any{"goal"},
			},
		},
		{
			Name:        toolDelegateWave,
			Description: "Decompose a multi-step goal into a task graph and execute via parallel workers. Do NOT use for greetings or simple single-turn questions.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"goal": map[string]any{"type": "string", "description": "The multi-step goal to orchestrate"},
				},
				"required": []any{"goal"},
			},
		},
		// DM-20260617-004 (devrix-d7-tool-ctx-inject): expose free_fork to LLM
		// under loop_first so users saying "用 free_fork 启 N 个 worker" reach a
		// real registered tool. Execution is delegated to the base adapter's
		// ExecuteRound fallback path (freeforkRunner in D2 ToolRegistry).
		//
		// DM-20260620-001-B (AC10) — `mode` field selects sub-agent context
		// inheritance: brief (default, no parent history), fork (cache-friendly
		// prefix), full (legacy — full parent history).
		{
			Name:        "free_fork",
			Description: "Batch fork N child agents (1..5) under a parent session. Each child inherits the parent's session id and runs in an isolated worktree. Returns {spawned_count, agent_ids:[...]}.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"parent_session": map[string]any{"type": "string", "description": "Parent session id (caller's session)."},
					"requests": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":     map[string]any{"type": "string", "description": "Short name for the worker (used in logs)."},
								"prompt":   map[string]any{"type": "string", "description": "Self-contained instruction for the worker."},
								"worktree": map[string]any{"type": "boolean", "description": "Run in isolated worktree (default true)."},
								"mode":     map[string]any{"type": "string", "enum": []any{"brief", "fork", "full"}, "default": "brief", "description": "Sub-agent context inheritance mode (DM-20260620-001-B / AC10). brief = no parent history (default); fork = cache-friendly prefix for sibling workers; full = full parent history (legacy)."},
							},
							"required": []any{"name", "prompt"},
						},
						"minItems": 1,
						"maxItems": 5,
					},
				},
				"required": []any{"parent_session", "requests"},
			},
		},
	}
}

var _ ToolRoundExecutor = (*TurnToolExecutor)(nil)
var _ ContextPreparer = (*TurnPrepareWrapper)(nil)
