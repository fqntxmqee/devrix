package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

const (
	toolDelegateWave  = "delegate_wave"
	toolEnterPlanMode = "enter_plan_mode"
)

// TurnToolExecutor handles loop-first orchestration tools and delegates all
// other tool calls to the base executor (D2 tool runner).
type TurnToolExecutor struct {
	Base        turn.ToolRoundExecutor
	Orchestrate *OrchestratePath
	PlanMode    *workmodel.PlanMode
	LoopFirst   bool
	Metrics     *TurnToolMetrics

	DelegateWaveCount atomic.Int64
}

// NewTurnToolExecutor wraps base with orchestration tools when loopFirst is true.
func NewTurnToolExecutor(
	base turn.ToolRoundExecutor,
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

func (e *TurnToolExecutor) ExecuteRound(ctx context.Context, req turn.ToolRoundRequest) (turn.ToolRoundResult, error) {
	if e == nil || e.Base == nil {
		return turn.ToolRoundResult{}, fmt.Errorf("turn tool executor: base is nil")
	}
	if !e.LoopFirst {
		return e.Base.ExecuteRound(ctx, req)
	}

	results := make([]turn.ToolResult, len(req.ToolCalls))
	for i, tc := range req.ToolCalls {
		switch tc.Name {
		case toolDelegateWave:
			out, err := e.runDelegateWave(ctx, req.SessionID, tc.Input)
			results[i] = turn.ToolResult{ToolCallID: tc.ID, Output: out, Error: errString(err)}
		case toolEnterPlanMode:
			out, err := e.runEnterPlanMode(ctx, req.SessionID, tc.Input)
			results[i] = turn.ToolResult{ToolCallID: tc.ID, Output: out, Error: errString(err)}
		default:
			single, err := e.Base.ExecuteRound(ctx, turn.ToolRoundRequest{
				SessionID: req.SessionID,
				ToolCalls: []llmgateway.ToolCall{tc},
			})
			if err != nil {
				return turn.ToolRoundResult{}, err
			}
			if len(single.Results) > 0 {
				results[i] = single.Results[0]
			}
		}
	}
	return turn.ToolRoundResult{Results: results}, nil
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
	emit := turn.ToolEventStreamFrom(ctx)
	ch, err := e.Orchestrate.Run(ctx, ProcessRequest{SessionID: sessionID, Message: goal}, IntentClassification{})
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
		display = "Plan mode entered."
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
	Inner     turn.ContextPreparer
	LoopFirst bool
}

func (w *TurnPrepareWrapper) Prepare(ctx context.Context, req turn.PrepareRequest) (turn.PreparedContext, error) {
	pc, err := w.Inner.Prepare(ctx, req)
	if err != nil {
		return pc, err
	}
	if w.LoopFirst {
		pc.Tools = append(pc.Tools, orchestrationToolSchemas()...)
	}
	return pc, nil
}

func orchestrationToolSchemas() []turn.ToolSchema {
	return []turn.ToolSchema{
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
	}
}

var _ turn.ToolRoundExecutor = (*TurnToolExecutor)(nil)
var _ turn.ContextPreparer = (*TurnPrepareWrapper)(nil)
