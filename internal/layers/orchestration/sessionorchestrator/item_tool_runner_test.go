package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
)

type stubToolRoundExecutor struct{}

func (stubToolRoundExecutor) ExecuteRound(_ context.Context, req ToolRoundRequest) (ToolRoundResult, error) {
	out := ToolResult{Output: "tool ok"}
	if len(req.ToolCalls) > 0 {
		out.ToolCallID = req.ToolCalls[0].ID
	}
	return ToolRoundResult{Results: []ToolResult{out}}, nil
}

func TestItemToolRunner_WorkItemExecuteDecommissioned(t *testing.T) {
	// DM-20260626-009: the synthetic work_item_execute tool path is
	// replaced by WorkItemExecutor (workitem_executor.go). ItemToolRunner
	// must surface an explicit error so a leftover caller cannot silently
	// short-circuit the LLM call again.
	runner := NewItemToolRunner(nil)
	res, err := runner.Invoke(context.Background(), execute.ToolRequest{
		SessionID: "s1",
		ToolName:  workItemExecuteTool,
		Args:      map[string]any{"directive": "review d2领域代码"},
	})
	if err == nil {
		t.Fatalf("Invoke: expected decommission error, got nil (Output=%q)", res.Output)
	}
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d, want 1", res.ExitCode)
	}
}

func TestItemToolRunner_DelegatesRealTools(t *testing.T) {
	runner := NewItemToolRunner(stubToolRoundExecutor{})
	res, err := runner.Invoke(context.Background(), execute.ToolRequest{
		SessionID: "s1",
		ToolName:  "read_file",
		Args:      map[string]any{"path": "x.go"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.ExitCode != 0 || res.Output != "tool ok" {
		t.Fatalf("result = %+v", res)
	}
}