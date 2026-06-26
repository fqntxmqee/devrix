package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

const workItemExecuteTool = "work_item_execute"

// ItemToolRunner adapts ToolRoundExecutor to execute.ToolRunner for the
// per-WorkItem MUPS pipeline (Phase D bootstrap).
//
// work_item_execute (synthetic "execute directive" tool) delegates to the
// D7 LLMInvoker (D7-S2-A07) so a chat-style user instruction actually
// reaches the LLM. This is the regression hotfix for PR #243 + PR #246:
// the per-WorkItem pipeline became the default ingress (RunSessionTurnLoop
// in orchestrator.go:438-439), but ItemToolRunner previously returned a
// synthetic "work item executed: <directive>" string for work_item_execute,
// so the LLM was never called and the user saw an instant empty reply
// (sess_1782464239150_5000 — round completed in 11ms with no LLM call).
//
// Other tool names delegate to a.Exec.ExecuteRound (the real
// D2 ToolRoundExecutor path), preserving the v6.0.0+ surface/perms model.
type ItemToolRunner struct {
	Exec       ToolRoundExecutor
	LLMInvoker orchtypes.LLMInvoker
}

// NewItemToolRunner wraps a production tool executor for ItemPipelineRunner.
//
// LLMInvoker is optional: when nil, work_item_execute returns an explicit
// error instead of a synthetic result so the regression is surfaced instead
// of silently producing "work item executed: <directive>".
func NewItemToolRunner(exec ToolRoundExecutor) execute.ToolRunner {
	return ItemToolRunner{Exec: exec}
}

// NewItemToolRunnerWithLLM wires the LLMInvoker for the synthetic
// work_item_execute tool. Bootstrap uses this constructor so the
// per-WorkItem pipeline can actually reach the LLM (D7-S2-A07).
func NewItemToolRunnerWithLLM(exec ToolRoundExecutor, llm orchtypes.LLMInvoker) execute.ToolRunner {
	return ItemToolRunner{Exec: exec, LLMInvoker: llm}
}

func (a ItemToolRunner) Invoke(ctx context.Context, req execute.ToolRequest) (execute.ToolResult, error) {
	now := time.Now()
	if req.ToolName == workItemExecuteTool {
		return a.invokeWorkItemExecute(ctx, req, now)
	}
	if a.Exec == nil {
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			Output:      "",
			StartedAt:   now,
			CompletedAt: now,
		}, fmt.Errorf("item tool runner: executor not wired for %q", req.ToolName)
	}

	input := ""
	if len(req.Args) > 0 {
		b, err := json.Marshal(req.Args)
		if err != nil {
			return execute.ToolResult{ToolName: req.ToolName, StartedAt: now, CompletedAt: now},
				fmt.Errorf("item tool runner: marshal args: %w", err)
		}
		input = string(b)
	}

	round, err := a.Exec.ExecuteRound(ctx, ToolRoundRequest{
		SessionID: req.SessionID,
		ToolCalls: []llmgateway.ToolCall{{
			ID:    uuid.NewString(),
			Name:  req.ToolName,
			Input: input,
		}},
	})
	if err != nil {
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			StartedAt:   now,
			CompletedAt: time.Now(),
		}, err
	}
	if len(round.Results) == 0 {
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			StartedAt:   now,
			CompletedAt: time.Now(),
		}, fmt.Errorf("item tool runner: no result for %q", req.ToolName)
	}
	tr := round.Results[0]
	exit := 0
	if tr.Error != "" {
		exit = 1
	}
	return execute.ToolResult{
		ToolName:    req.ToolName,
		ExitCode:    exit,
		Output:      tr.Output,
		StartedAt:   now,
		CompletedAt: time.Now(),
	}, nil
}

// invokeWorkItemExecute runs the synthetic "execute this directive" tool
// by calling the LLM via D7-S2-A07 (orchtypes.LLMInvoker). The directive
// is the user message; the LLM response becomes the ToolResult.Output and
// flows into the round's ArtifactSummary, which RunSessionTurnLoop emits
// as a text event so the user sees the actual answer.
func (a ItemToolRunner) invokeWorkItemExecute(ctx context.Context, req execute.ToolRequest, now time.Time) (execute.ToolResult, error) {
	directive := ""
	if req.Args != nil {
		if v, ok := req.Args["directive"].(string); ok {
			directive = v
		}
	}
	directive = strings.TrimSpace(directive)
	if directive == "" {
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			StartedAt:   now,
			CompletedAt: now,
		}, fmt.Errorf("item tool runner: work_item_execute requires a non-empty directive arg")
	}
	if a.LLMInvoker == nil {
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			Output:      "",
			StartedAt:   now,
			CompletedAt: now,
		}, fmt.Errorf("item tool runner: work_item_execute requires LLMInvoker (bootstrap wiring missing — see wire_item_pipeline.go)")
	}

	ch, err := a.LLMInvoker.InvokeStream(ctx, orchtypes.LLMInvokeRequest{
		SessionID: req.SessionID,
		Messages: []types.Message{{
			Role:    types.MessageRoleUser,
			Content: directive,
		}},
	})
	if err != nil {
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			StartedAt:   now,
			CompletedAt: time.Now(),
		}, fmt.Errorf("item tool runner: llm invoke: %w", err)
	}

	var sb strings.Builder
	for chunk := range ch {
		if chunk.Content != "" {
			sb.WriteString(chunk.Content)
		}
		if chunk.Thinking != "" {
			sb.WriteString(chunk.Thinking)
		}
		if chunk.FinishReason != "" && chunk.FinishReason != "stop" {
			return execute.ToolResult{
				ToolName:    req.ToolName,
				ExitCode:    1,
				Output:      sb.String(),
				StartedAt:   now,
				CompletedAt: time.Now(),
			}, fmt.Errorf("item tool runner: llm finish_reason=%s", chunk.FinishReason)
		}
	}
	return execute.ToolResult{
		ToolName:    req.ToolName,
		ExitCode:    0,
		Output:      sb.String(),
		StartedAt:   now,
		CompletedAt: time.Now(),
	}, nil
}
