package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
	"github.com/google/uuid"
)

const workItemExecuteTool = "work_item_execute"

// ItemToolRunner adapts ToolRoundExecutor to execute.ToolRunner for the
// per-WorkItem MUPS pipeline (Phase D bootstrap).
//
// DM-20260626-009: ItemPipelineRunner.Run no longer routes through
// ItemToolRunner — WorkItemExecutor now drives the per-WorkItem
// LLM↔Tool ReAct loop directly (see workitem_executor.go). This type is
// kept as a thin adapter for any legacy ToolRunner consumers that still
// need execute.ToolRunner compatibility (e.g. ChannelRouter/ChannelExecute
// wiring in tests). The work_item_execute synthetic tool now fails fast
// so a misuse cannot silently short-circuit the LLM call again.
type ItemToolRunner struct {
	Exec ToolRoundExecutor
}

// NewItemToolRunner wraps a production tool executor for ItemPipelineRunner.
func NewItemToolRunner(exec ToolRoundExecutor) execute.ToolRunner {
	return ItemToolRunner{Exec: exec}
}

func (a ItemToolRunner) Invoke(ctx context.Context, req execute.ToolRequest) (execute.ToolResult, error) {
	now := time.Now()
	if req.ToolName == workItemExecuteTool {
		// DM-20260626-009: ItemPipelineRunner now drives the per-WorkItem
		// LLM↔Tool loop via WorkItemExecutor (workitem_executor.go). The
		// work_item_execute synthetic tool path is decommissioned; surface
		// an explicit error rather than returning a synthetic stub so any
		// leftover caller is forced to migrate.
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    1,
			Output:      "",
			StartedAt:   now,
			CompletedAt: now,
		}, fmt.Errorf("item tool runner: work_item_execute path decommissioned (DM-20260626-009); use WorkItemExecutor instead")
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