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
		directive := ""
		if req.Args != nil {
			if v, ok := req.Args["directive"].(string); ok {
				directive = v
			}
		}
		out := "work item executed"
		if directive != "" {
			out = fmt.Sprintf("work item executed: %s", directive)
		}
		return execute.ToolResult{
			ToolName:    req.ToolName,
			ExitCode:    0,
			Output:      out,
			StartedAt:   now,
			CompletedAt: now.Add(time.Millisecond),
		}, nil
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
