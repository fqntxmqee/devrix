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

func TestItemToolRunner_WorkItemExecuteSynthetic(t *testing.T) {
	runner := NewItemToolRunner(nil)
	res, err := runner.Invoke(context.Background(), execute.ToolRequest{
		SessionID: "s1",
		ToolName:  workItemExecuteTool,
		Args:      map[string]any{"directive": "build cache"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0", res.ExitCode)
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
