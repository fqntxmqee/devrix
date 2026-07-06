package sessionorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
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

// stubExecMUPS implements MaterializeForMUPS for executor tests.
type stubExecMUPS struct {
	system string
	tools  []contracts.MUPSToolDescriptor
}

func (s stubExecMUPS) MaterializeForMUPS(_ context.Context, req contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	var msgs []types.Message
	if req.UserMessage != "" && req.Turn != nil {
		msgs = []types.Message{{
			SessionID: req.Turn.SessionID,
			Role:      types.MessageRoleUser,
			Content:   req.UserMessage,
		}}
	}
	return contracts.MUPSPreparedContext{
		SystemPrompt: s.system,
		Messages:     msgs,
		Tools:        s.tools,
	}, nil
}

func workItemExecTestCtx(sessionID string) (context.Context, string) {
	tm := workmodel.NewTaskManager()
	goal, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{Kind: workmodel.WorkKindGoal, Title: "g", Directive: "d"})
	child, _ := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{ParentID: goal.ID, Kind: workmodel.WorkKindExplore, Title: "c", Directive: "d"})
	return WithWorkItemExecContext(context.Background(), WorkItemExecContext{Item: child, Tasks: tm}), child.ID
}

// stubCtxPreparer returns canned SystemPrompt + Tools (legacy Prepare tests).
type stubCtxPreparer struct {
	system string
	tools  []ToolSchema
}

func (s stubCtxPreparer) Prepare(_ context.Context, _ PrepareRequest) (PreparedContext, error) {
	return PreparedContext{SystemPrompt: s.system, Tools: s.tools}, nil
}

func TestWorkItemExecutor_FindingsJSONFinalIterDisablesTools(t *testing.T) {
	contract := workmodel.DeliverableContract{
		Structure: workmodel.DeliverableStructureFindingsJSON,
		Citation:  workmodel.DeliverableCitationFileLine,
		Severity:  workmodel.DeliverableSeverityP0P1,
	}
	toolChunks := []llmgateway.Chunk{{
		Content:      "tool",
		ToolCalls:    []llmgateway.ToolCall{{ID: "c", Name: "read_file", Input: "{}"}},
		FinishReason: "tool_calls",
	}}
	finalChunks := []llmgateway.Chunk{{Content: `{"findings":[]}`, FinishReason: "stop"}}
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{toolChunks, toolChunks, toolChunks, toolChunks, finalChunks}}
	toolsExec := &scriptedTools{results: []ToolRoundResult{
		{Results: []ToolResult{{ToolCallID: "c", Output: "ok"}}},
	}}
	prep := stubExecMUPS{tools: []contracts.MUPSToolDescriptor{{Name: "read_file"}}}
	exec := NewWorkItemExecutor(llm, prep, toolsExec)
	exec.MaxIters = 5
	ctx, itemID := workItemExecTestCtx("s1")
	ec, _ := WorkItemExecContextFrom(ctx)
	ec.DeliverableContract = contract
	ctx = WithWorkItemExecContext(ctx, ec)
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "review plan code")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if !res.Done {
		t.Fatalf("Done=false StopReason=%q", res.StopReason)
	}
	if len(llm.tools) != 5 {
		t.Fatalf("expected 5 LLM calls, got %d", len(llm.tools))
	}
	if len(llm.tools[4]) != 0 {
		t.Fatalf("final iter must disable tools, got %d tools", len(llm.tools[4]))
	}
}

func TestWorkItemExecutor_FinalAnswerNoTools(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "hello ", FinishReason: "stop"}, {Content: "world", FinishReason: "stop"}},
	}}
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{system: "sys"}, nil)
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "review d2领域代码")
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
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, tools)
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "explain x")
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
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, tools)
	exec.MaxIters = 3
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "loop forever")
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
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, nil)
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "x")
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
	ctx, itemID := workItemExecTestCtx("s1")
	exec := &DefaultWorkItemExecutor{MUPS: stubExecMUPS{}, Tools: &scriptedTools{}}
	_, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "x")
	if err == nil {
		t.Fatal("expected error when LLMInvoker is nil")
	}
}

func TestWorkItemExecutor_RequiresDirective(t *testing.T) {
	llm := &scriptedLLM{}
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, nil)
	_, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "   ")
	if err == nil {
		t.Fatal("expected error when directive is blank")
	}
	if llm.calls != 0 {
		t.Fatalf("LLM should not be called when directive blank (calls=%d)", llm.calls)
	}
}

func TestWorkItemExecutor_LLMError(t *testing.T) {
	llm := &errLLM{err: errors.New("upstream broken")}
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, nil)
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "x")
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

// TestWorkItemExecutor_EmitHook_Hotfix_2026_06_27 pins the per-WorkItem
// ReAct loop's intermediate event emission. Without this hook the
// ItemPipelineRunner default path ran tool.bash calls but feishu cards
// only saw the final ArtifactSummary (silent regression introduced
// when ItemPipelineRunner became the default execution surface in
// DM-20260626-009). The fix: WorkItemExecutor forwards each chunk's
// text/thinking/tool_call + the post-round tool_result events to a
// caller-provided Emit hook so RunSessionTurnLoop can land them on
// the gateway out channel — matching what OrchestratePath's
// streamEmit/workerEventToEngine already does for the Wave path.
//
// Assertions:
//   - text events fire per non-empty Content chunk (sequence: "let me check" then "answer is 42")
//   - thinking event fires for non-empty Thinking chunk
//   - tool_call event fires with ToolName/ToolInput populated
//   - tool_result event fires after the round with name resolved via
//     ToolCallID lookup (ToolResult itself carries no Name field)
//   - nil Emit hook is a no-op (legacy / test safety, separate test)
//   - emitted events carry SessionID so gateway can route
func TestWorkItemExecutor_EmitHook_Hotfix_2026_06_27(t *testing.T) {
	var (
		mu  sync.Mutex
		all []*contracts.EngineEvent // ordered
	)
	emit := func(ev *contracts.EngineEvent) {
		mu.Lock()
		defer mu.Unlock()
		cp := *ev
		all = append(all, &cp)
	}

	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{
			Content:      "let me check",
			Thinking:     "the model is reasoning",
			ToolCalls:    []llmgateway.ToolCall{{ID: "call_1", Name: "read_file", Input: `{"path":"x.go"}`}},
			FinishReason: "tool_calls",
		}},
		{{Content: "answer is 42", FinishReason: "stop"}},
	}}
	tools := &scriptedTools{results: []ToolRoundResult{
		{Results: []ToolResult{{ToolCallID: "call_1", Output: "package x"}}},
	}}
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, tools)

	ctx, itemID := workItemExecTestCtx("sess_emit")
	ec, _ := WorkItemExecContextFrom(ctx)
	ec.Emit = emit
	ctx = WithWorkItemExecContext(ctx, ec)
	if _, err := exec.ExecuteWorkItem(ctx, "sess_emit", itemID, "explain x"); err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Expected sequence:
	//   text("let me check") → thinking("reasoning") → tool_call(read_file) → tool_result(package x) → text("answer is 42")
	wantSeq := []string{"text", "thinking", "tool_call", "tool_result", "text"}
	if len(all) != len(wantSeq) {
		types := []string{}
		for _, e := range all {
			types = append(types, e.Type)
		}
		t.Fatalf("emit count = %d (types=%v), want %d (want=%v)", len(all), types, len(wantSeq), wantSeq)
	}
	for i, w := range wantSeq {
		if all[i].Type != w {
			t.Fatalf("emit[%d].Type = %q, want %q", i, all[i].Type, w)
		}
	}

	// text event content (first text = "let me check", last text = "answer is 42")
	textEvents := []*contracts.EngineEvent{}
	for _, e := range all {
		if e.Type == "text" {
			textEvents = append(textEvents, e)
		}
	}
	if len(textEvents) != 2 {
		t.Fatalf("text events = %d, want 2", len(textEvents))
	}
	if textEvents[0].Content != "let me check" || textEvents[1].Content != "answer is 42" {
		t.Fatalf("text content = %q / %q, want let me check / answer is 42",
			textEvents[0].Content, textEvents[1].Content)
	}
	if textEvents[0].SessionID != "sess_emit" {
		t.Fatalf("text[0].SessionID = %q, want sess_emit", textEvents[0].SessionID)
	}

	// thinking event
	var thinkEv *contracts.EngineEvent
	for _, e := range all {
		if e.Type == "thinking" {
			thinkEv = e
			break
		}
	}
	if thinkEv == nil || thinkEv.Content != "the model is reasoning" {
		t.Fatalf("thinking event missing or wrong content: %+v", thinkEv)
	}

	// tool_call event
	var tcEv *contracts.EngineEvent
	for _, e := range all {
		if e.Type == "tool_call" {
			tcEv = e
			break
		}
	}
	if tcEv == nil {
		t.Fatal("tool_call event missing")
	}
	if tcEv.ToolName != "read_file" || tcEv.ToolInput != `{"path":"x.go"}` {
		t.Fatalf("tool_call ToolName=%q ToolInput=%q", tcEv.ToolName, tcEv.ToolInput)
	}
	if tcEv.Metadata["tool_name"] != "read_file" || tcEv.Metadata["call_id"] != "call_1" {
		t.Fatalf("tool_call metadata = %+v", tcEv.Metadata)
	}

	// tool_result event: ToolResult has no Name field, so the executor
	// must look up the originating tool name via ToolCallID.
	var trEv *contracts.EngineEvent
	for _, e := range all {
		if e.Type == "tool_result" {
			trEv = e
			break
		}
	}
	if trEv == nil {
		t.Fatal("tool_result event missing")
	}
	if trEv.ToolName != "read_file" {
		t.Fatalf("tool_result.ToolName = %q, want read_file (resolved via ToolCallID lookup)", trEv.ToolName)
	}
	if trEv.Content != "package x" {
		t.Fatalf("tool_result.Content = %q, want %q", trEv.Content, "package x")
	}
	if trEv.Metadata["tool_call_id"] != "call_1" {
		t.Fatalf("tool_result metadata tool_call_id = %q", trEv.Metadata["tool_call_id"])
	}
}

// TestWorkItemExecutor_NilEmit_NoOp ensures the Emit hook is optional
// (legacy fixtures, fast-path callers, and tests that don't need
// observability). When Emit is nil, ExecuteWorkItem must still return
// a valid WorkItemResult.
func TestWorkItemExecutor_NilEmit_NoOp(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "ok", FinishReason: "stop"}},
	}}
	ctx, itemID := workItemExecTestCtx("s1")
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, nil)
	// Emit intentionally nil
	res, err := exec.ExecuteWorkItem(ctx, "s1", itemID, "noop")
	if err != nil {
		t.Fatalf("ExecuteWorkItem with nil Emit: %v", err)
	}
	if !res.Done || res.Content != "ok" {
		t.Fatalf("res = %+v, want Done=true Content=ok", res)
	}
}

// TestWorkItemExecutor_PlanFrameDelta_Binder_DM20260706_002 covers the
// hotfix binder for D7 Phase 1 FrameDelta injection: when ec.PlanFrameDelta
// is non-nil the LLM must receive a system_prompt whose suffix carries the
// `<plan_frame_delta schema=...>` block produced by summarizePlanFrameDelta.
// When ec.PlanFrameDelta is nil the system_prompt is the baseline verbatim
// (InjectPlanFrameDelta no-op branch). Pre-hotfix Jaeger verification showed
// 0 D7_Execute_PlanFrameDelta_Inject spans in production (trace
// cf29aeaaa602f736 on 2026-07-06) even when ItemPipelineRunner.bind
// produced a non-nil FrameDelta — the binder was missing in
// ExecuteWorkItem. This test locks the wiring.
func TestWorkItemExecutor_PlanFrameDelta_Binder_DM20260706_002(t *testing.T) {
	const baselineSystem = "baseline-exec-system-prompt-DM-20260706-002"

	type tc struct {
		name      string
		fd        *interfaces.FrameDelta // nil → expect verbatim baseline
		wantDelta bool                    // expect system_prompt to differ from baseline
		wantSub   string                  // substring to assert if wantDelta
	}
	cases := []tc{
		{
			name:      "nil_plan_frame_delta_returns_baseline_verbatim",
			fd:        nil,
			wantDelta: false,
		},
		{
			name: "non_zero_plan_frame_delta_appends_injection",
			fd: &interfaces.FrameDelta{
				ExecutionMode: "protocol",
			},
			wantDelta: true,
			wantSub:   `<plan_frame_delta schema="`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			llm := &scriptedLLM{script: [][]llmgateway.Chunk{
				{{Content: "ok", FinishReason: "stop"}},
			}}
			exec := NewWorkItemExecutor(llm, stubExecMUPS{system: baselineSystem}, nil)
			ctx, itemID := workItemExecTestCtx("sess_" + c.name)
			ec, _ := WorkItemExecContextFrom(ctx)
			ec.PlanFrameDelta = c.fd
			ctx = WithWorkItemExecContext(ctx, ec)
			if _, err := exec.ExecuteWorkItem(ctx, "sess_"+c.name, itemID, "do-it"); err != nil {
				t.Fatalf("ExecuteWorkItem: %v", err)
			}
			if len(llm.system) == 0 {
				t.Fatalf("LLM received no system_prompt (calls=%d)", llm.calls)
			}
			got := llm.system[0]
			if c.wantDelta {
				if got == baselineSystem {
					t.Fatalf("system_prompt unchanged; want baseline + InjectPlanFrameDelta injection")
				}
				if !strings.Contains(got, c.wantSub) {
					t.Fatalf("system_prompt missing injection marker %q; got prefix=%q", c.wantSub, got[:min(160, len(got))])
				}
				// baseline is preserved as prefix
				if !strings.HasPrefix(got, baselineSystem) {
					t.Fatalf("system_prompt dropped baseline; got prefix=%q", got[:min(80, len(got))])
				}
			} else {
				if got != baselineSystem {
					t.Fatalf("system_prompt changed despite nil plan_delta; got=%q want=%q", got, baselineSystem)
				}
			}
		})
	}
}

func TestFilterEphemeralExecuteMessages_dropsSynthesisHint(t *testing.T) {
	msgs := []types.Message{
		{Role: types.MessageRoleAssistant, Content: "ok"},
		{Role: types.MessageRoleUser, Content: "<deliverable_format>findings_json_only</deliverable_format>\nFinal iteration: tools are disabled."},
	}
	got := filterEphemeralExecuteMessages(msgs)
	if len(got) != 1 || got[0].Role != types.MessageRoleAssistant {
		t.Fatalf("got %v", got)
	}
}

// DM-20260706-010: thinking chunks MUST NOT pollute WorkItemResult.Content.
// Previously streamLLM concatenated chunk.Thinking into the same strings.Builder
// as chunk.Content, so for a trivial Q&A like "2×3=几?" the final user-visible
// answer was a verbose template wrapping the LLM's reasoning. Transcript
// evidence (sess_1783349211523_1000 entry 5): result.Content = "2 × 3 = ...6...
// 用户问了一个简单的数学题...让我给出简洁的中文回答。<conclusion>2×3=6（六）。
// </conclusion>" — exactly the bug. Fix: only chunk.Content accumulates into the
// return string; chunk.Thinking still emits a "thinking" EngineEvent for trace.
func TestWorkItemExecutor_ThinkingDoesNotPolluteResult_DM20260706_010(t *testing.T) {
	var (
		mu          sync.Mutex
		eventTypes  []string
		thinkingEv  *contracts.EngineEvent
	)
	emit := func(ev *contracts.EngineEvent) {
		mu.Lock()
		defer mu.Unlock()
		cp := *ev
		eventTypes = append(eventTypes, cp.Type)
		if cp.Type == "thinking" {
			thinkingEv = &cp
		}
	}

	// Simulate minimax M2.7-style streaming: thinking interleaved with text,
	// bracketed by template tokens the LLM emitted in its prior round.
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{
			{Content: "用户问了一个简单的数学题。", Thinking: "用户问了一个简单的数学题，让我心算一下："},
			{Content: "2 × 3 = ", Thinking: "2 乘以 3 = 6"},
			{Content: "6", Thinking: "得到答案 6"},
			{Content: "（六）", FinishReason: "stop"},
		},
	}}
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, nil)

	ctx, itemID := workItemExecTestCtx("sess_think_pollute")
	ec, _ := WorkItemExecContextFrom(ctx)
	ec.Emit = emit
	ctx = WithWorkItemExecContext(ctx, ec)

	res, err := exec.ExecuteWorkItem(ctx, "sess_think_pollute", itemID, "2×3=几?")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if !res.Done {
		t.Fatalf("Done=false StopReason=%q", res.StopReason)
	}

	// (1) result.Content MUST be content-only, no thinking bleed.
	want := "用户问了一个简单的数学题。2 × 3 = 6（六）"
	if res.Content != want {
		t.Fatalf("result.Content polluted by thinking:\n got: %q\nwant: %q", res.Content, want)
	}
	for _, leak := range []string{
		"让我心算",
		"2 乘以 3",
		"得到答案",
		"<think>",
		"<conclusion>",
	} {
		if strings.Contains(res.Content, leak) {
			t.Fatalf("result.Content contains thinking/template leak %q:\n%s", leak, res.Content)
		}
	}

	// (2) thinking events still fire (trace + transcript contract).
	mu.Lock()
	defer mu.Unlock()
	wantThinkCount := 3
	gotThinkCount := 0
	for _, et := range eventTypes {
		if et == "thinking" {
			gotThinkCount++
		}
	}
	if gotThinkCount != wantThinkCount {
		t.Fatalf("thinking event count = %d, want %d (eventTypes=%v)", gotThinkCount, wantThinkCount, eventTypes)
	}
	if thinkingEv == nil {
		t.Fatalf("no thinking event captured (eventTypes=%v)", eventTypes)
	}
	// thinking loop iterates and overwrites thinkingEv, so we can only
	// assert at least one thinking event fired with any of the scripted
	// chunks. The count assertion above is the strong invariant.
	wantThinkSubstrings := []string{"让我心算", "2 乘以 3", "得到答案"}
	matchedAny := false
	for _, want := range wantThinkSubstrings {
		if strings.Contains(thinkingEv.Content, want) {
			matchedAny = true
			break
		}
	}
	if !matchedAny {
		t.Fatalf("last thinking event has unexpected content %q", thinkingEv.Content)
	}
}

// DM-20260706-010: assistant message carried into the next LLM iteration must
// not include thinking either (otherwise we'd echo the LLM's prior reasoning
// back into its own context, inflating tokens and potentially confusing the
// model on multi-iter tool-call loops). This locks the wire-format invariant
// for `buildWorkItemAssistantToolCallMsg`.
func TestWorkItemExecutor_AssistantMsgExcludesThinking_DM20260706_010(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		// Iter 1: LLM emits thinking + tool_call. Expect assistant message
		// pushed into iter 2's history to contain Content only.
		{{
			Content:      "let me read the file",
			Thinking:     "reasoning should NOT appear in assistant message",
			ToolCalls:    []llmgateway.ToolCall{{ID: "call_1", Name: "read_file", Input: `{"path":"x.go"}`}},
			FinishReason: "tool_calls",
		}},
		// Iter 2: final answer — only content, no thinking.
		{{Content: "answer is 42", FinishReason: "stop"}},
	}}
	tools := &scriptedTools{results: []ToolRoundResult{
		{Results: []ToolResult{{ToolCallID: "call_1", Output: "package x"}}},
	}}
	exec := NewWorkItemExecutor(llm, stubExecMUPS{}, tools)
	ctx, itemID := workItemExecTestCtx("sess_think_msg")
	if _, err := exec.ExecuteWorkItem(ctx, "sess_think_msg", itemID, "explain x"); err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if len(llm.messages) != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(llm.messages))
	}

	// Iter 2's messages = [system(setup)..., user(question), assistant(content+tool_call), tool(result)]
	// Find the assistant message — must not contain "reasoning should NOT".
	iter2 := llm.messages[1]
	var assistantMsg *types.Message
	for i := range iter2 {
		if iter2[i].Role == types.MessageRoleAssistant {
			assistantMsg = &iter2[i]
			break
		}
	}
	if assistantMsg == nil {
		t.Fatalf("iter 2 has no assistant message: %+v", iter2)
	}
	if strings.Contains(assistantMsg.Content, "reasoning should NOT") {
		t.Fatalf("assistant message polluted by thinking:\n  Content=%q", assistantMsg.Content)
	}
	if assistantMsg.Content != "let me read the file" {
		t.Fatalf("assistant Content = %q, want %q", assistantMsg.Content, "let me read the file")
	}
}
