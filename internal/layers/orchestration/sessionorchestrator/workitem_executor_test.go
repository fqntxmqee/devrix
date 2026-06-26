package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/types"
)

// scriptedLLM replays a sequence of streamed Chunk responses. Each entry
// becomes one llmgateway.Chunk (in order) on a closed channel. Tests build
// the script to exercise both "no tool calls (final answer)" and "with
// tool calls (loop)" paths through DefaultWorkItemExecutor.ExecuteWorkItem.
type scriptedLLM struct {
	script [][]llmgateway.Chunk
	// messages records every LLMInvokeRequest seen so tests can assert
	// the per-iteration message history (system prompt + tools + tool
	// result messages) matches the canonical ReAct pattern.
	messages [][]types.Message
	tools    [][]orchtypes.ToolSchema
	system   []string
	calls    int
}

func (s *scriptedLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.calls++
	s.messages = append(s.messages, req.Messages)
	s.tools = append(s.tools, req.Tools)
	s.system = append(s.system, req.SystemPrompt)
	if s.calls-1 >= len(s.script) {
		// No more scripted output — close channel so the executor's loop
		// sees a tool-call-free final answer with empty content.
		ch := make(chan llmgateway.Chunk)
		close(ch)
		return ch, nil
	}
	ch := make(chan llmgateway.Chunk, len(s.script[s.calls-1]))
	for _, c := range s.script[s.calls-1] {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// scriptedTools feeds pre-recorded ToolRoundResults. When the script is
// exhausted, returns a default "ok" result so the executor can finish a
// final iteration with empty tool_calls (scripted as no tools).
type scriptedTools struct {
	results []ToolRoundResult
	calls   int
}

func (s *scriptedTools) ExecuteRound(_ context.Context, _ ToolRoundRequest) (ToolRoundResult, error) {
	idx := s.calls
	s.calls++
	if idx < len(s.results) {
		return s.results[idx], nil
	}
	return ToolRoundResult{Results: []ToolResult{{Output: "ok"}}}, nil
}

// stubCtxPreparer returns canned SystemPrompt + Tools.
type stubCtxPreparer struct {
	system string
	tools  []ToolSchema
}

func (s stubCtxPreparer) Prepare(_ context.Context, _ PrepareRequest) (PreparedContext, error) {
	return PreparedContext{SystemPrompt: s.system, Tools: s.tools}, nil
}

func TestWorkItemExecutor_FinalAnswerNoTools(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "hello ", FinishReason: "stop"}, {Content: "world", FinishReason: "stop"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{system: "sys"}, nil)
	res, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "review d2领域代码")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if !res.Done {
		t.Fatal("Done = false, want true")
	}
	if res.Content != "hello world" {
		t.Fatalf("Content = %q, want %q", res.Content, "hello world")
	}
	if res.StopReason != "final_answer" {
		t.Fatalf("StopReason = %q, want final_answer", res.StopReason)
	}
	if res.Iterations != 1 || res.ToolCalls != 0 {
		t.Fatalf("iter=%d toolcalls=%d, want 1/0", res.Iterations, res.ToolCalls)
	}
	if len(llm.messages) != 1 || llm.messages[0][0].Content != "review d2领域代码" {
		t.Fatalf("messages = %+v", llm.messages)
	}
	if llm.system[0] != "sys" {
		t.Fatalf("system prompt = %q, want %q", llm.system[0], "sys")
	}
}

func TestWorkItemExecutor_ToolLoop(t *testing.T) {
	// First iteration: LLM asks to call "read_file". Second iteration:
	// LLM returns a tool-call-free final answer. The test asserts that
	// the tool result is appended as a tool-role message between the
	// two LLM calls and that the loop terminates successfully.
	toolCallsJSON, _ := json.Marshal([]map[string]any{{
		"id":   "call_1",
		"type": "function",
		"function": map[string]any{
			"name":      "read_file",
			"arguments": `{"path":"x.go"}`,
		},
	}})
	_ = toolCallsJSON
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "let me read the file", ToolCalls: []llmgateway.ToolCall{
			{ID: "call_1", Name: "read_file", Input: `{"path":"x.go"}`},
		}, FinishReason: "tool_calls"}},
		{{Content: "the answer is 42", FinishReason: "stop"}},
	}}
	tools := &scriptedTools{results: []ToolRoundResult{
		{Results: []ToolResult{{ToolCallID: "call_1", Output: "package x"}}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, tools)
	res, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "explain x")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if !res.Done || res.Content != "let me read the filethe answer is 42" {
		t.Fatalf("Done=%v Content=%q", res.Done, res.Content)
	}
	if res.Iterations != 2 || res.ToolCalls != 1 {
		t.Fatalf("iter=%d toolcalls=%d, want 2/1", res.Iterations, res.ToolCalls)
	}
	if tools.calls != 1 {
		t.Fatalf("tool round calls = %d, want 1", tools.calls)
	}
	// Second LLM call must see: [user, assistant+tool_calls, tool].
	second := llm.messages[1]
	if len(second) != 3 {
		t.Fatalf("2nd LLM messages = %d, want 3: %+v", len(second), second)
	}
	if second[1].Role != types.MessageRoleAssistant {
		t.Fatalf("messages[1].Role = %q, want assistant", second[1].Role)
	}
	if second[1].Metadata["tool_calls"] == "" {
		t.Fatal("assistant message missing Metadata[tool_calls]")
	}
	if !strings.Contains(second[1].Metadata["tool_calls"], "read_file") {
		t.Fatalf("tool_calls metadata missing read_file: %q", second[1].Metadata["tool_calls"])
	}
	var gotCalls []map[string]any
	if err := json.Unmarshal([]byte(second[1].Metadata["tool_calls"]), &gotCalls); err != nil {
		t.Fatalf("unmarshal tool_calls metadata: %v (raw=%q)", err, second[1].Metadata["tool_calls"])
	}
	if len(gotCalls) != 1 || gotCalls[0]["id"] != "call_1" {
		t.Fatalf("tool_calls metadata = %+v, want 1 entry with id=call_1", gotCalls)
	}
	if second[2].Role != types.MessageRoleTool || second[2].Metadata["tool_call_id"] != "call_1" {
		t.Fatalf("messages[2] = %+v, want tool role with call_id=call_1", second[2])
	}
	if second[2].Content != "package x" {
		t.Fatalf("tool result content = %q, want %q", second[2].Content, "package x")
	}
}

func TestWorkItemExecutor_MaxItersCap(t *testing.T) {
	// LLM always asks for a tool; loop must cap at MaxIters and return
	// accumulated content with StopReason="max_iters".
	chunks := []llmgateway.Chunk{{
		Content:      "iter",
		ToolCalls:    []llmgateway.ToolCall{{ID: "c", Name: "noop", Input: "{}"}},
		FinishReason: "tool_calls",
	}}
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{chunks, chunks, chunks, chunks, chunks, chunks, chunks}}
	tools := &scriptedTools{results: []ToolRoundResult{
		{Results: []ToolResult{{ToolCallID: "c", Output: "ok"}}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, tools)
	exec.MaxIters = 3
	res, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "loop forever")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if res.Done {
		t.Fatal("Done = true, want false (cap hit)")
	}
	if res.StopReason != "max_iters" {
		t.Fatalf("StopReason = %q, want max_iters", res.StopReason)
	}
	if res.Iterations != 3 {
		t.Fatalf("Iterations = %d, want 3", res.Iterations)
	}
}

func TestWorkItemExecutor_NoTools_Wired_Degrades(t *testing.T) {
	// LLM asks for a tool but no ToolRoundExecutor is wired. Executor
	// must degrade gracefully with accumulated content (not infinite
	// loop) and label StopReason="tool_no_executor".
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "first", ToolCalls: []llmgateway.ToolCall{
			{ID: "c", Name: "noop", Input: "{}"},
		}, FinishReason: "tool_calls"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	res, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "x")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if res.StopReason != "tool_no_executor" {
		t.Fatalf("StopReason = %q, want tool_no_executor", res.StopReason)
	}
	if res.Content != "first" {
		t.Fatalf("Content = %q, want %q", res.Content, "first")
	}
}

func TestWorkItemExecutor_RequiresLLM(t *testing.T) {
	exec := &DefaultWorkItemExecutor{Context: stubCtxPreparer{}, Tools: &scriptedTools{}}
	_, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "x")
	if err == nil {
		t.Fatal("expected error when LLMInvoker is nil")
	}
}

func TestWorkItemExecutor_RequiresDirective(t *testing.T) {
	llm := &scriptedLLM{}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	_, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "   ")
	if err == nil {
		t.Fatal("expected error when directive is blank")
	}
	if llm.calls != 0 {
		t.Fatalf("LLM should not be called when directive blank (calls=%d)", llm.calls)
	}
}

func TestWorkItemExecutor_LLMError(t *testing.T) {
	llm := &errLLM{err: errors.New("upstream broken")}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	res, err := exec.ExecuteWorkItem(context.Background(), "s1", "i1", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if res.StopReason != "llm_error" {
		t.Fatalf("StopReason = %q, want llm_error", res.StopReason)
	}
}

type errLLM struct{ err error }

func (e *errLLM) InvokeStream(_ context.Context, _ orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	return nil, e.err
}