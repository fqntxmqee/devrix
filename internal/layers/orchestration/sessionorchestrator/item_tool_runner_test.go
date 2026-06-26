package sessionorchestrator

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubToolRoundExecutor struct{}

func (stubToolRoundExecutor) ExecuteRound(_ context.Context, req ToolRoundRequest) (ToolRoundResult, error) {
	out := ToolResult{Output: "tool ok"}
	if len(req.ToolCalls) > 0 {
		out.ToolCallID = req.ToolCalls[0].ID
	}
	return ToolRoundResult{Results: []ToolResult{out}}, nil
}

type stubLLMInvoker struct {
	chunks   []string
	calls    int
	req      orchtypes.LLMInvokeRequest
	tierWant string
}

func (s *stubLLMInvoker) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.calls++
	s.req = req
	ch := make(chan llmgateway.Chunk, len(s.chunks)+1)
	for _, c := range s.chunks {
		ch <- llmgateway.Chunk{Content: c}
	}
	close(ch)
	return ch, nil
}

func TestItemToolRunner_WorkItemExecuteRequiresLLMInvoker(t *testing.T) {
	// Regression: with nil LLMInvoker the synthetic "work item executed: …"
	// path must NOT silently return — the regression hotfix surfaces a
	// wiring error so bootstrap can't silently bypass the LLM again.
	runner := NewItemToolRunner(nil)
	res, err := runner.Invoke(context.Background(), execute.ToolRequest{
		SessionID: "s1",
		ToolName:  workItemExecuteTool,
		Args:      map[string]any{"directive": "review d2领域代码"},
	})
	if err == nil {
		t.Fatalf("Invoke: expected wiring error, got nil (Output=%q)", res.Output)
	}
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d, want 1", res.ExitCode)
	}
}

func TestItemToolRunner_WorkItemExecuteCallsLLM(t *testing.T) {
	// Hotfix path: directive flows through to LLMInvoker and the streamed
	// chunks concatenate into ToolResult.Output. This replaces the
	// pre-hotfix synthetic "work item executed: <directive>" stub.
	llm := &stubLLMInvoker{chunks: []string{"hello ", "world"}}
	runner := NewItemToolRunnerWithLLM(nil, llm)
	res, err := runner.Invoke(context.Background(), execute.ToolRequest{
		SessionID: "s1",
		ToolName:  workItemExecuteTool,
		Args:      map[string]any{"directive": "review d2领域代码"},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0", res.ExitCode)
	}
	if res.Output != "hello world" {
		t.Fatalf("Output = %q, want %q", res.Output, "hello world")
	}
	if llm.calls != 1 {
		t.Fatalf("llm calls = %d, want 1", llm.calls)
	}
	if len(llm.req.Messages) != 1 || llm.req.Messages[0].Content != "review d2领域代码" {
		t.Fatalf("llm req messages = %+v", llm.req.Messages)
	}
	if llm.req.Messages[0].Role != types.MessageRoleUser {
		t.Fatalf("llm req role = %q, want user", llm.req.Messages[0].Role)
	}
	if llm.req.SessionID != "s1" {
		t.Fatalf("llm req sessionID = %q, want s1", llm.req.SessionID)
	}
}

func TestItemToolRunner_WorkItemExecuteEmptyDirectiveRejected(t *testing.T) {
	llm := &stubLLMInvoker{}
	runner := NewItemToolRunnerWithLLM(nil, llm)
	res, err := runner.Invoke(context.Background(), execute.ToolRequest{
		SessionID: "s1",
		ToolName:  workItemExecuteTool,
		Args:      map[string]any{"directive": "  "},
	})
	if err == nil {
		t.Fatalf("Invoke: expected error for empty directive, got Output=%q", res.Output)
	}
	if llm.calls != 0 {
		t.Fatalf("llm should not be called when directive is empty (calls=%d)", llm.calls)
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
