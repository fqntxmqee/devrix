package turn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// --- stubs ---

type stubLLM struct {
	chunks []llmgateway.Chunk
	err    error
	calls  atomic.Int64
	// fn overrides the default behaviour when non-nil.
	fn func(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error)
}

func (s *stubLLM) InvokeStream(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.calls.Add(1)
	if s.fn != nil {
		return s.fn(ctx, req)
	}
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan llmgateway.Chunk, len(s.chunks))
	for _, c := range s.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

type stubContext struct {
	prepared PreparedContext
	err      error
}

func (s *stubContext) Prepare(_ context.Context, _ PrepareRequest) (PreparedContext, error) {
	return s.prepared, s.err
}

type stubTools struct {
	results   []ToolResult
	err       error
	lastCalls []llmgateway.ToolCall
	fn        func(ctx context.Context, req ToolRoundRequest) (ToolRoundResult, error)
}

func (s *stubTools) ExecuteRound(ctx context.Context, req ToolRoundRequest) (ToolRoundResult, error) {
	s.lastCalls = append([]llmgateway.ToolCall(nil), req.ToolCalls...)
	if s.fn != nil {
		return s.fn(ctx, req)
	}
	if s.err != nil {
		return ToolRoundResult{}, s.err
	}
	return ToolRoundResult{Results: s.results}, nil
}

type stubPersist struct {
	persisted []PersistRequest
	err       error
}

func (s *stubPersist) PersistTurn(_ context.Context, req PersistRequest) error {
	s.persisted = append(s.persisted, req)
	return s.err
}

// --- helpers ---

func textChunk(content string) llmgateway.Chunk {
	return llmgateway.Chunk{Content: content}
}

func thinkingChunk(content string) llmgateway.Chunk {
	return llmgateway.Chunk{Thinking: content}
}

func toolCallChunk(name, input string) llmgateway.Chunk {
	return llmgateway.Chunk{
		ToolCalls: []llmgateway.ToolCall{{Name: name, ID: "call_1", Input: input}},
		Done:      true,
	}
}

func doneChunk() llmgateway.Chunk {
	return llmgateway.Chunk{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: 10, CompletionTokens: 5}}
}

func usageDoneChunk(prompt, completion int) llmgateway.Chunk {
	return llmgateway.Chunk{Done: true, Usage: llmgateway.TokenUsage{PromptTokens: prompt, CompletionTokens: completion}}
}

func collectEvents(ch <-chan *contracts.EngineEvent) []*contracts.EngineEvent {
	var evs []*contracts.EngineEvent
	for e := range ch {
		evs = append(evs, e)
	}
	return evs
}

func eventTypes(evs []*contracts.EngineEvent) []string {
	types := make([]string, len(evs))
	for i, e := range evs {
		types[i] = e.Type
	}
	return types
}

func hasType(evs []*contracts.EngineEvent, typ string) bool {
	for _, e := range evs {
		if e.Type == typ {
			return true
		}
	}
	return false
}

// --- tests ---

// D7-S2-A06-T01b: LLM request must contain exactly one copy of the current user turn.
func TestOrchestrator_RunTurn_SingleUserMessageInLLMRequest(t *testing.T) {
	var captured LLMInvokeRequest
	llm := &stubLLM{fn: func(_ context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		captured = req
		ch := make(chan llmgateway.Chunk, 1)
		ch <- textChunk("ok")
		close(ch)
		return ch, nil
	}}
	history := []types.Message{
		{Role: types.MessageRoleUser, Content: "上一轮", SessionID: "sess-dup"},
		{Role: types.MessageRoleAssistant, Content: "回答", SessionID: "sess-dup"},
	}
	ctxPrep := &stubContext{prepared: PreparedContext{Messages: history}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 1,
	})

	current := types.Message{
		Role:      types.MessageRoleUser,
		Content:   "d5和d6重构需求应该交付了，请结合代码判断一下",
		SessionID: "sess-dup",
	}
	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-dup",
		UserMessage: current,
		MaxTurns:    1,
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	_ = collectEvents(ch)

	userCount := 0
	for _, m := range captured.Messages {
		if m.Role == types.MessageRoleUser && m.Content == current.Content {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("expected 1 current user message in LLM request, got %d: %+v", userCount, captured.Messages)
	}
	if len(captured.Messages) != len(history)+1 {
		t.Fatalf("expected %d messages (history + current), got %d: %+v", len(history)+1, len(captured.Messages), captured.Messages)
	}
}

// D7-S2-A06-T01: Basic turn — PREPARE → LLM(text) → PERSIST → complete
func TestOrchestrator_RunTurn_SingleTurn_NoTools(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("hello "), textChunk("world"), doneChunk(),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{
		SystemPrompt: "be helpful",
		Tools:        []ToolSchema{{Name: "read", Description: "read file"}},
	}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-1",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "text") {
		t.Error("expected text event")
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event")
	}
	if hasType(evs, "tool_call") {
		t.Error("unexpected tool_call event")
	}
	if llm.calls.Load() != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.calls.Load())
	}
	if len(persist.persisted) != 1 {
		t.Fatalf("expected 1 persist, got %d", len(persist.persisted))
	}
	p := persist.persisted[0]
	if p.SessionID != "sess-1" {
		t.Errorf("persist SessionID = %q, want sess-1", p.SessionID)
	}
	if p.TurnCount != 1 {
		t.Errorf("persist TurnCount = %d, want 1", p.TurnCount)
	}
}

func TestOrchestrator_RunTurn_CompleteCarriesUsageMetadata(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("hello"), usageDoneChunk(12800, 5),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{
		Model:            "MiniMax-M2.5",
		MaxContextTokens: 128000,
	}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
		DefaultModel: "fallback-model", MaxContextTokens: 64000,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-meta",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	var complete *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "complete" {
			complete = ev
			break
		}
	}
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if complete.Metadata["usage"] != "12805" {
		t.Fatalf("usage metadata = %q, want 12805", complete.Metadata["usage"])
	}
	if complete.Metadata["model"] != "MiniMax-M2.5" {
		t.Fatalf("model metadata = %q, want MiniMax-M2.5", complete.Metadata["model"])
	}
	if complete.Metadata["duration"] == "" {
		t.Fatal("expected duration metadata")
	}
	if complete.Metadata["ctx_pct"] == "" {
		t.Fatal("expected ctx_pct metadata")
	}
}

func TestOrchestrator_RunTurn_UsageOnNonDoneChunk(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("hello"),
		{Usage: llmgateway.TokenUsage{PromptTokens: 120, CompletionTokens: 30}},
		{Done: true},
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{Model: "MiniMax-M2.5"}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-usage",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	var complete *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "complete" {
			complete = ev
		}
	}
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if complete.Metadata["usage"] != "150" {
		t.Fatalf("usage metadata = %q, want 150", complete.Metadata["usage"])
	}
}

// D7-S2-A06-T03: Multi-turn tool_use — LLM returns tool_calls → tool round → LLM(text)
func TestOrchestrator_RunTurn_MultiTurn_ToolLoop(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.err = nil // will be overridden per-call via closure

	// First call returns tool calls, second returns text.
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := callCount.Add(1)
		if n == 1 {
			ch := make(chan llmgateway.Chunk, 2)
			ch <- textChunk("let me check that")
			ch <- llmgateway.Chunk{
				ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "t1", Input: `{"path":"/f"}`}},
				Done:      true,
				Usage:     llmgateway.TokenUsage{PromptTokens: 5, CompletionTokens: 3},
			}
			close(ch)
			return ch, nil
		}
		ch := make(chan llmgateway.Chunk, 2)
		ch <- textChunk("result is 42")
		ch <- doneChunk()
		close(ch)
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	tools := &stubTools{results: []ToolResult{{ToolCallID: "t1", Output: "file contents"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-2",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "read /f"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if callCount.Load() != 2 {
		t.Errorf("expected 2 LLM calls, got %d", callCount.Load())
	}
	if !hasType(evs, "tool_call") {
		t.Error("expected tool_call event")
	}
	if !hasType(evs, "tool_result") {
		t.Error("expected tool_result event")
	}
	if !hasType(evs, "text") {
		t.Error("expected text event")
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event")
	}
}

// TestOrchestrator_RunTurn_FinalTextAccumulatesAcrossTurns verifies that the
// complete event's Content carries the LLM-emitted text from EVERY turn of
// the run, not just the last one. Regression guard for the deep-review
// scenario where the conclusion is emitted across multiple turns; without
// accumulation the IM card receives only the final turn's snippet.
func TestOrchestrator_RunTurn_FinalTextAccumulatesAcrossTurns(t *testing.T) {
	// fnCalls is incremented only inside the fn closure so it stays in
	// sync with the per-call switch. llm.calls is incremented by
	// InvokeStream itself and would double-count.
	var fnCalls atomic.Int64
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := fnCalls.Add(1)
		ch := make(chan llmgateway.Chunk, 4)
		switch n {
		case 1:
			ch <- textChunk("first: exploring repo")
			ch <- toolCallChunk("read_1", `{"path":"/a"}`)
		case 2:
			ch <- textChunk("second: analyzing tools")
			ch <- toolCallChunk("grep_2", `{"pat":"ctx.TODO"}`)
		case 3:
			ch <- textChunk("third: writing report")
			ch <- doneChunk()
		default:
			ch <- doneChunk()
		}
		close(ch)
		return ch, nil
	}

	// Each turn's toolCall has a unique ID so stubTools can return a
	// matching result without bleeding across turns.
	tools := &stubTools{}
	tools.fn = func(_ context.Context, req ToolRoundRequest) (ToolRoundResult, error) {
		var results []ToolResult
		for _, c := range req.ToolCalls {
			results = append(results, ToolResult{ToolCallID: c.ID, Output: "ok:" + c.Name})
		}
		return ToolRoundResult{Results: results}, nil
	}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: &stubPersist{}, MaxTurns: 6,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-finaltext",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "deep review"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if got := fnCalls.Load(); got != 3 {
		var summaries []string
		for _, e := range evs {
			summaries = append(summaries, fmt.Sprintf("%s:%q", e.Type, e.Content))
		}
		t.Fatalf("expected 3 LLM calls, got %d\nevents: %v", got, summaries)
	}

	var complete *contracts.EngineEvent
	for _, e := range evs {
		if e.Type == "complete" {
			complete = e
		}
	}
	if complete == nil {
		t.Fatal("no complete event emitted")
	}

	// Each turn emitted its own text chunk via textChunk(...); the final
	// Content must contain ALL three snippets, in order.
	wantSubs := []string{
		"first: exploring repo",
		"second: analyzing tools",
		"third: writing report",
	}
	joined := complete.Content
	if joined == "" {
		t.Fatalf("complete.Content is empty; want accumulated %v", wantSubs)
	}
	prev := -1
	for _, sub := range wantSubs {
		idx := strings.Index(joined, sub)
		if idx < 0 {
			t.Errorf("complete.Content missing %q; got: %q", sub, joined)
			continue
		}
		if idx <= prev {
			t.Errorf("complete.Content out of order for %q at %d (prev %d)", sub, idx, prev)
		}
		prev = idx
	}
}

// Streaming SSE sends the full merged tool-call snapshot on every delta frame.
func TestDedupeToolCalls_should_collapse_by_id(t *testing.T) {
	calls := []llmgateway.ToolCall{
		{ID: "call_1", Name: "grep"},
		{ID: "call_1", Name: "grep"},
		{ID: "call_2", Name: "read"},
	}
	got := dedupeToolCalls(calls)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestOrchestrator_RunTurn_DedupesStreamingToolCalls(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := callCount.Add(1)
		if n == 1 {
			ch := make(chan llmgateway.Chunk, 4)
			ch <- llmgateway.Chunk{ToolCalls: []llmgateway.ToolCall{{ID: "call_1", Name: "grep", Input: `{"q":"todo"}`}}}
			ch <- llmgateway.Chunk{ToolCalls: []llmgateway.ToolCall{{ID: "call_1", Name: "grep", Input: `{"q":"todo"}`}}}
			ch <- llmgateway.Chunk{ToolCalls: []llmgateway.ToolCall{{ID: "call_1", Name: "grep", Input: `{"q":"todo"}`}}, Done: true}
			close(ch)
			return ch, nil
		}
		ch := make(chan llmgateway.Chunk, 2)
		ch <- textChunk("done")
		ch <- doneChunk()
		close(ch)
		return ch, nil
	}

	tools := &stubTools{results: []ToolResult{{ToolCallID: "call_1", Output: "ok"}}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: &stubPersist{}, MaxTurns: 4,
	})
	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-dedup",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "list todos"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	collectEvents(ch)

	if len(tools.lastCalls) != 1 {
		t.Fatalf("ExecuteRound tool call count = %d, want 1", len(tools.lastCalls))
	}
	if callCount.Load() != 2 {
		t.Fatalf("LLM rounds = %d, want 2", callCount.Load())
	}
}

// D7-S2-A06-T02: Cancel between turns — cancelled context emits error before next LLM call.
// The cancel check is at the turn boundary (top of each loop iteration); cancelling
// mid-stream while the LLM is streaming will drain the stream and emit "complete".
func TestOrchestrator_RunTurn_CancelBetweenTurns(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(ctx context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := callCount.Add(1)
		if n == 1 {
			// First turn: return tool calls so the loop continues
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{
				ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "t1", Input: "{}"}},
				Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
			}
			close(ch)
			return ch, nil
		}
		// Second turn: block until cancelled
		ch := make(chan llmgateway.Chunk)
		go func() {
			defer close(ch)
			<-ctx.Done()
		}()
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	tools := &stubTools{results: []ToolResult{{ToolCallID: "t1", Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := orch.RunTurn(ctx, TurnRequest{
		SessionID: "sess-3",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	// Wait for first turn to complete, then cancel before second LLM call
	time.Sleep(50 * time.Millisecond)
	cancel()

	evs := collectEvents(ch)

	// After cancel, the second turn's LLM channel closes → runLoop processes empty chunks
	// → no tool calls → "complete" event (not error, because the cancel is detected
	// after the stream drain, not at the turn boundary)
	if !hasType(evs, "complete") {
		t.Errorf("expected complete after stream drain, got %v", eventTypes(evs))
	}
}

// D7-S2-A06-T02: Context cancelled before first LLM call — check in turn loop
func TestOrchestrator_RunTurn_CancelBeforeLLM(t *testing.T) {
	llm := &stubLLM{}
	llm.fn = func(ctx context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		// Should not be reached — context already cancelled
		t.Error("LLM should not be called with cancelled context")
		return nil, ctx.Err()
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch, err := orch.RunTurn(ctx, TurnRequest{
		SessionID: "sess-cancel-early",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	evs := collectEvents(ch)
	if !hasType(evs, "error") {
		t.Errorf("expected error event for cancelled context, got %v", eventTypes(evs))
	}
}

// --- LLM error path ---

// D7-S2-A07-T01: LLM invoke error — InvokeStream returns error → error event
func TestOrchestrator_RunTurn_LLMInvokeError(t *testing.T) {
	llm := &stubLLM{err: errors.New("breaker open")}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-4",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "error") {
		t.Error("expected error event for LLM invoke error")
	}
	if hasType(evs, "complete") {
		t.Error("unexpected complete event on LLM error")
	}
}

// --- Prepare error path ---

func TestOrchestrator_RunTurn_PrepareError(t *testing.T) {
	ctxPrep := &stubContext{err: errors.New("session not found")}
	llm := &stubLLM{}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-5",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "error") {
		t.Error("expected error event for prepare error")
	}
	if llm.calls.Load() != 0 {
		t.Error("LLM should not be called after prepare error")
	}
}

// --- Tool round error path ---

func TestOrchestrator_RunTurn_ToolRoundError(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		toolCallChunk("read", `{"path":"/f"}`),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	tools := &stubTools{err: errors.New("permission denied")}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-toolerr",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "read /f"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "error") {
		t.Error("expected error event for tool round error")
	}
}

// --- Max turns exceeded ---

func TestOrchestrator_RunTurn_MaxTurnsExceeded(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		callCount.Add(1)
		ch := make(chan llmgateway.Chunk, 1)
		// Always return tool calls so the loop continues
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: "{}"}},
			Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	tools := &stubTools{results: []ToolResult{{ToolCallID: "tx", Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 3,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-max",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "loop"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if callCount.Load() != 3 {
		t.Errorf("expected 3 LLM calls (maxTurns), got %d", callCount.Load())
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event after max turns")
	}
}

// --- Stream chunk types ---

func TestOrchestrator_RunTurn_ThinkingAndTextChunks(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		thinkingChunk("hmm let me think"),
		textChunk("the answer is "),
		thinkingChunk("almost there"),
		textChunk("42"),
		doneChunk(),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-think",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "what is 6*7"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	thinkCount := 0
	textCount := 0
	for _, e := range evs {
		switch e.Type {
		case "thinking":
			thinkCount++
		case "text":
			textCount++
		}
	}
	if thinkCount != 2 {
		t.Errorf("expected 2 thinking events, got %d", thinkCount)
	}
	if textCount != 2 {
		t.Errorf("expected 2 text events, got %d", textCount)
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event")
	}
}

// --- Input validation ---

func TestOrchestrator_RunTurn_EmptySessionID(t *testing.T) {
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: &stubLLM{}, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{},
	})

	_, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err == nil {
		t.Fatal("expected error for empty SessionID")
	}
}

// --- CompressHint produces summarization ---

func TestOrchestrator_RunTurn_CompressHint_LLM(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := callCount.Add(1)
		if n == 1 {
			// First call is the summarization
			ch := make(chan llmgateway.Chunk, 1)
			ch <- llmgateway.Chunk{Content: "summarized context", Done: true}
			close(ch)
			return ch, nil
		}
		// Second call is the main turn
		ch := make(chan llmgateway.Chunk, 2)
		ch <- textChunk("got it")
		ch <- doneChunk()
		close(ch)
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{
		CompressHint: &CompressHint{
			MessagesToSummarize: []types.Message{
				{Role: types.MessageRoleUser, Content: "long history 1"},
				{Role: types.MessageRoleAssistant, Content: "long reply 1"},
			},
			TargetTokenBudget: 2000,
		},
	}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-compress",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "continue"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if callCount.Load() != 2 {
		t.Errorf("expected 2 LLM calls (1 compress + 1 main), got %d", callCount.Load())
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event after compress")
	}
}

// --- CompressHint strips <think> from LLM-generated summary ---
//
// Regression: a previous build stored the LLM's compression summary verbatim
// into the next-turn system message. When the LLM emitted its working notes
// inside <think>...</think> (minimax M2.7, DeepSeek-R1 w/ chat template),
// the system message was polluted with thinking content, which the LLM
// then mirrored in subsequent turns ("<think>用户想...</think>" wrapping
// every answer). Fix: runCompress must call textutil.StripThinkingTags on
// the LLM summary before returning it.

func TestOrchestrator_RunTurn_CompressHint_StripsThinkTags(t *testing.T) {
	callCount := atomic.Int64{}
	var secondCallMessages []types.Message
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := callCount.Add(1)
		if n == 1 {
			// Compression call: LLM emits thinking + actual summary
			ch := make(chan llmgateway.Chunk, 4)
			ch <- llmgateway.Chunk{Content: "<think>"}
			ch <- llmgateway.Chunk{Content: "user asked me to summarize, let me think..."}
			ch <- llmgateway.Chunk{Content: "</think>\n\nThe user is debugging devrix."}
			ch <- llmgateway.Chunk{Done: true}
			close(ch)
			return ch, nil
		}
		// Main turn call: capture what the orchestrator actually sent us
		secondCallMessages = append([]types.Message(nil), req.Messages...)
		ch := make(chan llmgateway.Chunk, 2)
		ch <- textChunk("got it")
		ch <- doneChunk()
		close(ch)
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{
		CompressHint: &CompressHint{
			MessagesToSummarize: []types.Message{
				{Role: types.MessageRoleUser, Content: "long history"},
			},
			TargetTokenBudget: 2000,
		},
	}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-strip-think",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "continue"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", callCount.Load())
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event after compress")
	}

	// Find the system message injected from the compression summary.
	var systemContent string
	for _, m := range secondCallMessages {
		if m.Role == types.MessageRoleSystem {
			systemContent = m.Content
			break
		}
	}
	if systemContent == "" {
		t.Fatal("no system message found in second LLM call")
	}
	if strings.Contains(systemContent, "<think>") {
		t.Errorf("system message still contains <think>: %q", systemContent)
	}
	if strings.Contains(systemContent, "</think>") {
		t.Errorf("system message still contains </think>: %q", systemContent)
	}
	if !strings.Contains(systemContent, "The user is debugging devrix.") {
		t.Errorf("system message lost the actual summary: %q", systemContent)
	}
}

// --- CompressHint empty summary falls through to truncation ---

func TestOrchestrator_RunTurn_CompressHint_TruncationFallback(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := callCount.Add(1)
		if n == 1 {
			// Summarization returns error → triggers truncation
			return nil, errors.New("summarization failed")
		}
		// Main turn
		ch := make(chan llmgateway.Chunk, 2)
		ch <- textChunk("proceeding")
		ch <- doneChunk()
		close(ch)
		return ch, nil
	}

	// 25 messages → exceeds maxTruncatedMessages=20, should truncate to last 20
	msgs := make([]types.Message, 25)
	for i := range msgs {
		msgs[i] = types.Message{
			Role:    types.MessageRoleUser,
			Content: fmt.Sprintf("msg %d: lorem ipsum dolor sit amet consectetur adipiscing elit", i),
		}
	}

	ctxPrep := &stubContext{prepared: PreparedContext{
		CompressHint: &CompressHint{
			MessagesToSummarize: msgs,
			TargetTokenBudget:   2000,
		},
	}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-trunc",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "continue"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	// 1 summarization (failed), 1 main turn
	if callCount.Load() != 2 {
		t.Errorf("expected 2 LLM calls, got %d", callCount.Load())
	}
	if !hasType(evs, "complete") {
		t.Error("expected complete event after truncation fallback")
	}
}

// --- MaxTurns fallback: request 0 + orchestrator default → orchestrator's bound ---
//
// When the caller sets a positive MaxTurns on the orchestrator but leaves
// req.MaxTurns at 0, the orchestrator's bound is the safety net. The
// converse (orchestrator=0) leaves the request unbounded — covered by
// TestNewOrchestrator_DefaultMaxTurns. Both surfaces must agree with the
// "no magic default" rule: callers that want a bound set it explicitly.

func TestOrchestrator_RunTurn_RequestMaxTurnsZero_FallsBackToOrchestratorBound(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 5,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-default",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
		MaxTurns:    0, // orchestrator's MaxTurns=5 is the effective safety net
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "complete") {
		t.Error("expected complete event")
	}
}

// --- MaxTurns in constructor: 0 / negative stays unbounded ---
//
// Aligned with claude-code semantics: the main conversation has no hard
// turn limit. MaxTurns is an optional safety net that callers opt into.
// NewOrchestrator must NOT substitute a magic default when the caller
// leaves MaxTurns at 0 — that would re-introduce the hard ceiling that
// the alignment removed.

func TestNewOrchestrator_DefaultMaxTurns(t *testing.T) {
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: &stubLLM{}, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{},
		MaxTurns: 0,
	})
	if orch.maxTurns != 0 {
		t.Errorf("expected maxTurns=0 (unbounded) when caller passes 0, got %d", orch.maxTurns)
	}
}

// --- Persist error does not block completion ---

func TestOrchestrator_RunTurn_PersistError_StillCompletes(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{textChunk("ok"), doneChunk()}}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{err: errors.New("disk full")}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-persist-err",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "complete") {
		t.Error("expected complete event even when persist fails")
	}
}

// --- Event channel ordering ---

func TestOrchestrator_RunTurn_EventOrdering(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("answer"), doneChunk(),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID: "sess-order",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "q"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if len(evs) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(evs))
	}
	// Last event must be complete
	last := evs[len(evs)-1].Type
	if last != "complete" {
		t.Errorf("last event = %q, want complete", last)
	}
}
// --- D7-S2-A06-T04: SubQuery nested turn ---

func TestOrchestrator_RunTurn_SubQueryScope(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("subquery result"), doneChunk(),
	}}
	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-subquery",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "scan the repo"},
		Scope:       TurnScopeSubQuery,
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "complete") {
		t.Error("expected complete event for subquery turn")
	}
	if hasType(evs, "error") {
		t.Error("unexpected error event for subquery turn")
	}
}

func TestOrchestrator_RunTurn_SameOrchestratorForMainAndSubQuery(t *testing.T) {
	callCount := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		callCount.Add(1)
		ch := make(chan llmgateway.Chunk, 2)
		ch <- textChunk("done")
		ch <- doneChunk()
		close(ch)
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	persist := &stubPersist{}
	tools := &stubTools{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist, MaxTurns: 4,
	})

	// Run main turn
	ch1, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-shared",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "main task"},
		Scope:       TurnScopeMain,
	})
	if err != nil {
		t.Fatalf("RunTurn (main): %v", err)
	}
	evs1 := collectEvents(ch1)
	if !hasType(evs1, "complete") {
		t.Error("main turn should complete")
	}

	// Run subquery turn on the SAME orchestrator
	ch2, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-shared",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "sub task"},
		Scope:       TurnScopeSubQuery,
	})
	if err != nil {
		t.Fatalf("RunTurn (subquery): %v", err)
	}
	evs2 := collectEvents(ch2)
	if !hasType(evs2, "complete") {
		t.Error("subquery turn should complete on same orchestrator")
	}

	if callCount.Load() != 2 {
		t.Errorf("expected 2 total LLM calls, got %d", callCount.Load())
	}
}

// Regression: complete event must surface the final LLM-generated text on its
// Content field so IM adapters (Feishu cardkit streaming finalize, CLI plain
// stdout) can render the conclusion even when LLM produced no interleaved text
// chunks (e.g. thinking model that only emits a Done marker). See feishu.go
// OnMessage case "complete" → finalizeStructuredSession.
func TestOrchestrator_RunTurn_CompleteCarriesFinalText_NoTools(t *testing.T) {
	const wantText = "make lint 通过，无违规"

	// Simulate a clean LLM: emits a final-text chunk then Done.
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk(wantText),
		doneChunk(),
	}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{}, MaxTurns: 4,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-no-text",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "run lint"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	var complete *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "complete" {
			complete = ev
			break
		}
	}
	if complete == nil {
		t.Fatal("expected complete event")
	}
	// When the LLM emits a final-text chunk, the orchestrator forwards it
	// verbatim on complete.Content. The D1 IM adapter renders it as the
	// conclusion card text.
	if complete.Content != wantText {
		t.Errorf("complete.Content = %q, want %q", complete.Content, wantText)
	}
	if complete.SessionID != "sess-no-text" {
		t.Errorf("complete.SessionID = %q, want sess-no-text", complete.SessionID)
	}
}

// Regression: MaxTurns-exceeded path must emit a complete event whose Content
// carries the LAST iteration's accumulated LLM text. Previously the finalText
// was reset to "" at the end of each iteration, so MaxTurns emits were empty
// and IM adapters showed no conclusion card.
func TestOrchestrator_RunTurn_CompleteCarriesLastIterationText_MaxTurns(t *testing.T) {
	const lastIterText = "third attempt summary"

	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := turnIdx.Add(1)
		ch := make(chan llmgateway.Chunk, 4)
		switch n {
		case 1:
			ch <- textChunk("first iter ")
		case 2:
			ch <- textChunk("second iter ")
		case 3:
			ch <- textChunk(lastIterText)
		}
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: "{}"}},
			Done:      true,
			Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	tools := &stubTools{results: []ToolResult{{ToolCallID: "tx", Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: persist, MaxTurns: 3,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-maxtext",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "loop"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if turnIdx.Load() != 3 {
		t.Fatalf("expected 3 LLM calls (MaxTurns), got %d", turnIdx.Load())
	}

	var complete *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "complete" {
			complete = ev
			break
		}
	}
	if complete == nil {
		t.Fatal("expected complete event after MaxTurns")
	}
	if !strings.Contains(complete.Content, lastIterText) {
		t.Errorf("complete.Content = %q, want substring %q (last iteration's text)", complete.Content, lastIterText)
	}
	// Cross-turn accumulation (DM-20260620-002 follow-up): the complete
	// event now carries text from EVERY iteration, not just the last. This
	// is what makes deep-review-style reports render on the IM card even
	// when the LLM emits the conclusion across multiple turns before
	// hitting MaxTurns.
	if !strings.Contains(complete.Content, "first iter") {
		t.Errorf("complete.Content should contain earlier iteration's text (accumulator), got %q", complete.Content)
	}
	if !strings.Contains(complete.Content, "second iter") {
		t.Errorf("complete.Content should contain middle iteration's text (accumulator), got %q", complete.Content)
	}
	// Order is preserved — earlier text must precede later text.
	idxFirst := strings.Index(complete.Content, "first iter")
	idxLast := strings.Index(complete.Content, lastIterText)
	if idxFirst < 0 || idxLast < 0 || idxFirst > idxLast {
		t.Errorf("complete.Content order wrong: first@%d last@%d, got %q", idxFirst, idxLast, complete.Content)
	}
}

type stubResolveAwait struct {
	summary string
}

func (s *stubResolveAwait) AwaitRunningChildren(_ context.Context, _ string) string {
	return s.summary
}

func TestOrchestrator_RunTurn_EmitsResolveAwaitSummary(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{textChunk("done")}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:          llm,
		Context:      &stubContext{},
		Tools:        &stubTools{},
		Persist:      &stubPersist{},
		ResolveAwait: &stubResolveAwait{summary: "Resolve await: child: completed: ok."},
	})
	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-resolve",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	var resolveEv *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "resolve" {
			resolveEv = ev
		}
	}
	if resolveEv == nil {
		t.Fatal("expected resolve event")
	}
	if !strings.Contains(resolveEv.Content, "Resolve await") {
		t.Fatalf("resolve content = %q", resolveEv.Content)
	}
}

// TestResolveFinalText covers the thinking→finalText fallback helper. The
// bug it pins: when the LLM emits only thinking and no clean content
// (typical for providers without a native reasoning field when the
// model is in a tool-call-only final state), the IM adapter receives an
// empty finalText and renders a blank conclusion card. The helper must
// promote the most recent non-empty thinking into finalText and strip
// <think> defensively in case the splitter ever leaks a tag boundary.
//
// The helper also handles the MaxTurns truncation notice: when the loop
// exited on the safety net AND the caller set a positive MaxTurns, a
// user-visible "[max-turns reached...]" prefix is prepended so the IM
// card shows the bound was hit rather than a quiet final-text drop.
// Unbounded turns (MaxTurns ≤ 0) never carry the notice regardless of
// how they exited.
func TestResolveFinalText(t *testing.T) {
	cases := []struct {
		name      string
		finalText string
		thinking  string
		reason    ExitReason
		maxTurns  int
		want      string
	}{
		{
			name:      "non_empty_finalText_wins",
			finalText: "the answer is 42",
			thinking:  "i should think about this",
			reason:    ExitReasonNatural,
			want:      "the answer is 42",
		},
		{
			name:      "whitespace_finalText_falls_back_to_thinking",
			finalText: "\n\n\n",
			thinking:  "let me think: 6*7=42",
			reason:    ExitReasonNatural,
			want:      "let me think: 6*7=42",
		},
		{
			name:      "empty_thinking_keeps_blank",
			finalText: "",
			thinking:  "",
			reason:    ExitReasonNatural,
			want:      "",
		},
		{
			name:      "think_tags_in_thinking_are_stripped",
			finalText: "",
			thinking:  "<think>working notes</think>the final answer",
			reason:    ExitReasonNatural,
			want:      "the final answer",
		},
		{
			name:      "unclosed_think_tag_is_dropped_safely",
			finalText: "",
			thinking:  "<think>incomplete working notes",
			reason:    ExitReasonNatural,
			// StripThinkingTags drops unclosed <think> blocks entirely
			// (no matching closing tag → no safe partial). The helper
			// returns blank in that case, which the IM adapter then
			// handles via the empty-summary footer (D1 fallback).
			want: "",
		},
		{
			name:      "max_turns_with_text_prepends_notice",
			finalText: "third attempt summary",
			thinking:  "",
			reason:    ExitReasonMaxTurns,
			maxTurns:  3,
			want:      "[max-turns reached after 3 iterations; turn truncated]\nthird attempt summary",
		},
		{
			name:      "max_turns_with_thinking_prepends_notice",
			finalText: "",
			thinking:  "已多次调用工具触达 max-turns 兜底",
			reason:    ExitReasonMaxTurns,
			maxTurns:  3,
			want:      "[max-turns reached after 3 iterations; turn truncated]\n已多次调用工具触达 max-turns 兜底",
		},
		{
			name:      "max_turns_with_empty_returns_only_notice",
			finalText: "",
			thinking:  "",
			reason:    ExitReasonMaxTurns,
			maxTurns:  3,
			want:      "[max-turns reached after 3 iterations; turn truncated]",
		},
		{
			name:      "non_max_turns_reason_does_not_prepend_notice",
			finalText: "ok",
			thinking:  "",
			reason:    ExitReasonRepeatedTool,
			maxTurns:  3,
			// Even with MaxTurns>0, the loop exited for a different
			// reason (e.g. repeated tool) — the user did not hit the
			// safety net, so no truncation notice.
			want: "ok",
		},
		{
			name:      "max_turns_unbounded_skips_notice",
			finalText: "ok",
			thinking:  "",
			reason:    ExitReasonMaxTurns,
			maxTurns:  0,
			// Unbounded turns cannot legitimately hit MaxTurns; the
			// helper still treats maxTurns=0 as "no notice" defensively.
			want: "ok",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveFinalText(tc.finalText, tc.thinking, tc.reason, tc.maxTurns)
			if got != tc.want {
				t.Errorf("resolveFinalText(%q, %q, %q, %d) = %q, want %q",
					tc.finalText, tc.thinking, tc.reason, tc.maxTurns, got, tc.want)
			}
		})
	}
}

// TestOrchestrator_RunTurn_PromotesThinkingWhenContentBlank covers the
// max-turns + no-clean-text scenario reported by the user
// ("请尝试多轮工具调用…"). The LLM streams thinking-only chunks on the
// final iteration, hits MaxTurns, and the orchestrator must promote the
// last thinking into complete.Content so the IM adapter does not render
// an empty conclusion card.
func TestOrchestrator_RunTurn_PromotesThinkingWhenContentBlank_MaxTurns(t *testing.T) {
	const tailThinking = "已多次调用工具触达 max-turns 兜底"

	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := turnIdx.Add(1)
		ch := make(chan llmgateway.Chunk, 4)
		if n == 1 {
			ch <- thinkingChunk("first iteration: planning")
		} else {
			ch <- thinkingChunk(tailThinking)
		}
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: "{}"}},
			Done:      true,
			Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	tools := &stubTools{results: []ToolResult{{ToolCallID: "tx", Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: persist, MaxTurns: 3,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-think-only",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "loop with thinking only"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if turnIdx.Load() != 3 {
		t.Fatalf("expected 3 LLM calls (MaxTurns), got %d", turnIdx.Load())
	}

	var complete *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "complete" {
			complete = ev
			break
		}
	}
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if !strings.Contains(complete.Content, tailThinking) {
		t.Errorf("complete.Content = %q, want substring %q (promoted from thinking)", complete.Content, tailThinking)
	}
	// Earlier iteration's planning must NOT leak into the conclusion.
	if strings.Contains(complete.Content, "first iteration") {
		t.Errorf("complete.Content should not contain earlier iteration's thinking, got %q", complete.Content)
	}
}

// TestOrchestrator_RunTurn_PromotesThinkingWhenContentBlank_NoToolCalls
// covers the no-tool-call + thinking-only scenario: the LLM produces
// thinking but no clean text, ends the turn with finish_reason=stop,
// and the orchestrator must still promote the thinking into finalText.
func TestOrchestrator_RunTurn_PromotesThinkingWhenContentBlank_NoToolCalls(t *testing.T) {
	const tailThinking = "reflection without final answer"
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		thinkingChunk("analyzing the request"),
		thinkingChunk(tailThinking),
		doneChunk(),
	}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{},
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-thinkonly-notool",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	var complete *contracts.EngineEvent
	for _, ev := range evs {
		if ev.Type == "complete" {
			complete = ev
			break
		}
	}
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if !strings.Contains(complete.Content, tailThinking) {
		t.Errorf("complete.Content = %q, want substring %q", complete.Content, tailThinking)
	}
}

// ============================================================================
// ExitReason coverage — one test per deterministic exit reason (clawcode alignment).
//
// Each test pins the behaviour the orchestrator must guarantee when that exit
// reason fires. Together they cover every branch in runLoop's terminal block:
//   - natural
//   - max_turns (positive bound + cumulative iteration counter)
//   - repeated_tool (3x same signature in last 5 turns)
//   - tool_failure (3x same tool error)
//   - aborted_user (ctx cancellation between turns)
//   - token_diminishing (≥90% budget + 2 consecutive deltas <500 tokens)
// ============================================================================

// findCompleteEvent returns the first complete event in the slice, or nil.
func findCompleteEvent(evs []*contracts.EngineEvent) *contracts.EngineEvent {
	for _, e := range evs {
		if e.Type == "complete" {
			return e
		}
	}
	return nil
}

func findErrorEvent(evs []*contracts.EngineEvent) *contracts.EngineEvent {
	for _, e := range evs {
		if e.Type == "error" {
			return e
		}
	}
	return nil
}

// --- natural ---

// LLM emits no tool calls on the first iteration → exit_reason=natural,
// no truncation notice, no detector trip. Mirrors the simplest happy path
// and pins the metadata contract for downstream consumers.
func TestOrchestrator_RunTurn_NaturalCompletion_NoMaxTurns(t *testing.T) {
	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("hello world"), doneChunk(),
	}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{},
		// MaxTurns=0 → unbounded; the loop must terminate on the
		// natural LLM finish, not on the safety net.
	})
	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-natural",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	complete := findCompleteEvent(evs)
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if got := complete.Metadata["exit_reason"]; got != string(ExitReasonNatural) {
		t.Errorf("exit_reason = %q, want %q", got, ExitReasonNatural)
	}
	if strings.Contains(complete.Content, "max-turns") {
		t.Errorf("natural completion should not carry truncation notice, got %q", complete.Content)
	}
	if llm.calls.Load() != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.calls.Load())
	}
}

// --- max_turns ---

// With a positive MaxTurns the loop must break on iteration N+1 with
// exit_reason=max_turns, and the final complete.Content must carry the
// "[max-turns reached after N iterations; turn truncated]" notice so
// the IM card surfaces the bound to the user. Last iteration's text
// is preserved after the notice.
func TestOrchestrator_RunTurn_MaxTurnsReached_EmitsTruncationNotice(t *testing.T) {
	const lastIterText = "third attempt summary"

	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		turnIdx.Add(1)
		ch := make(chan llmgateway.Chunk, 4)
		ch <- textChunk(lastIterText)
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: "{}"}},
			Done:      true,
			Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	tools := &stubTools{results: []ToolResult{{ToolCallID: "tx", Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: persist, MaxTurns: 3,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-maxturns",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "loop"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	complete := findCompleteEvent(evs)
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if got := complete.Metadata["exit_reason"]; got != string(ExitReasonMaxTurns) {
		t.Errorf("exit_reason = %q, want %q", got, ExitReasonMaxTurns)
	}
	if !strings.Contains(complete.Content, "[max-turns reached after 3 iterations; turn truncated]") {
		t.Errorf("complete.Content missing truncation notice, got %q", complete.Content)
	}
	if !strings.Contains(complete.Content, lastIterText) {
		t.Errorf("complete.Content = %q, want substring %q", complete.Content, lastIterText)
	}
	if turnIdx.Load() != 3 {
		t.Errorf("expected 3 LLM calls (MaxTurns), got %d", turnIdx.Load())
	}
}

// --- repeated_tool ---

// The same (tool_name|input) signature appears 3+ times in the last
// 5 turns → loop must break with exit_reason=repeated_tool, no
// truncation notice (this is not a MaxTurns exit). 4th iteration
// is the one that fires the detector (history has 3 occurrences).
func TestOrchestrator_RunTurn_RepeatedTool_TriggersAbortedRepeatedTool(t *testing.T) {
	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		turnIdx.Add(1)
		ch := make(chan llmgateway.Chunk, 2)
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: `{"path":"/f"}`}},
			Done:      true,
			Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	tools := &stubTools{results: []ToolResult{{ToolCallID: "tx", Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: persist, MaxTurns: 20,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-repeated",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "stuck"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	complete := findCompleteEvent(evs)
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if got := complete.Metadata["exit_reason"]; got != string(ExitReasonRepeatedTool) {
		t.Errorf("exit_reason = %q, want %q", got, ExitReasonRepeatedTool)
	}
	if strings.Contains(complete.Content, "max-turns") {
		t.Errorf("repeated_tool exit should not carry truncation notice, got %q", complete.Content)
	}
	// 4th iteration: history holds 3 occurrences of the signature, the
	// 4th triggers the detector. Bound is 20 so the safety net never fires.
	if turnIdx.Load() != 4 {
		t.Errorf("expected 4 LLM calls (3 build history + 1 trips detector), got %d", turnIdx.Load())
	}
}

// --- tool_failure ---

// 3 consecutive tool rounds return the same error fingerprint → loop
// must break with exit_reason=tool_failure. A clean round in between
// resets the counter (covered in TestOrchestrator_RunTurn_ToolErrorResets
// in the implementation if needed; here we focus on the streak case).
func TestOrchestrator_RunTurn_ConsecutiveToolErrors_TriggersAbortedToolFailure(t *testing.T) {
	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		turnIdx.Add(1)
		ch := make(chan llmgateway.Chunk, 2)
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: `{"path":"/f"}`}},
			Done:      true,
			Usage:     llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	// Each ExecuteRound returns the SAME error string → same fingerprint.
	tools := &stubTools{results: []ToolResult{{
		ToolCallID: "tx",
		Error:      "permission denied: /f",
	}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: &stubContext{}, Tools: tools, Persist: persist, MaxTurns: 20,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-toolfail",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "loop"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	complete := findCompleteEvent(evs)
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if got := complete.Metadata["exit_reason"]; got != string(ExitReasonToolFailure) {
		t.Errorf("exit_reason = %q, want %q", got, ExitReasonToolFailure)
	}
	// 3 iterations: 1st sets fp+count=1, 2nd increments to 2, 3rd trips
	// detector at count=3.
	if turnIdx.Load() != 3 {
		t.Errorf("expected 3 LLM calls (3rd trips detector), got %d", turnIdx.Load())
	}
}

// --- aborted_user (context cancel between turns) ---

// Cancelling the context between turns must produce an `error` event
// with "turn cancelled" in the content and NO `complete` event. The
// orchestrator detects cancellation at the top of the run-loop body
// and short-circuits with the explicit error event so the IM adapter
// can render the cancellation rather than waiting for an unobservable
// stream close.
//
// To isolate the cancel exit from the other detectors, each iteration
// emits a UNIQUE tool-call signature (so repeated_tool never trips),
// MaxTurns is 0 (unbounded — safety net never fires), tools always
// succeed, and no maxContextTokens is configured (token_diminishing
// stays disabled). With all other exits neutralized, the only way
// the loop can terminate is via the cancel check at the top of the
// run-loop body.
func TestOrchestrator_RunTurn_UserCancel_TriggersAbortedUser(t *testing.T) {
	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := turnIdx.Add(1)
		ch := make(chan llmgateway.Chunk, 2)
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{
				Name:  "read",
				ID:    fmt.Sprintf("tx-%d", n),
				Input: fmt.Sprintf(`{"iter":%d}`, n),
			}},
			Done:  true,
			Usage: llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
		close(ch)
		return ch, nil
	}

	ctxPrep := &stubContext{prepared: PreparedContext{}}
	tools := &stubTools{results: []ToolResult{{Output: "ok"}}}
	persist := &stubPersist{}

	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm, Context: ctxPrep, Tools: tools, Persist: persist,
		// MaxTurns=0 → unbounded; cancel is the only way out.
	})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := orch.RunTurn(ctx, TurnRequest{
		SessionID:   "sess-cancel",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	// Wait for the first iteration to complete, then cancel before the
	// next iteration's cancel check fires.
	time.Sleep(50 * time.Millisecond)
	cancel()

	evs := collectEvents(ch)
	// Contract: no `complete` event is emitted, and an `error` event
	// with "cancelled" must appear.
	if findCompleteEvent(evs) != nil {
		t.Errorf("user cancel must NOT emit complete event, got %v", eventTypes(evs))
	}
	errEv := findErrorEvent(evs)
	if errEv == nil {
		t.Fatalf("expected error event for cancelled context, got %v", eventTypes(evs))
	}
	if !strings.Contains(errEv.Content, "cancel") {
		t.Errorf("error event missing cancel signal, got %q", errEv.Content)
	}
	// The loop must have run at least one iteration before the cancel
	// check fired (otherwise the test is a no-op).
	if turnIdx.Load() < 1 {
		t.Errorf("expected at least 1 LLM call before cancel, got %d", turnIdx.Load())
	}
}

// --- token_diminishing ---

// Cumulative usage crosses 90% of the context budget AND the last
// 2 per-turn deltas are both <500 tokens → loop must break with
// exit_reason=token_diminishing. The detector is the clawcode
// "marginal utility" stop condition in src/query/tokenBudget.ts.
//
// We pre-set maxContextTokens=1000 via the prepared context. Per-turn
// deltas of 400, 400, 200 give cumulative 400, 800, 1000 — 1000 ≥ 90%
// of 1000 (the 90% threshold). The last 2 deltas in the rolling
// window (400, 200) are both <500. Detector fires after the 3rd
// iteration's tool round.
func TestOrchestrator_RunTurn_TokenBudgetDiminishing_StopsLoop(t *testing.T) {
	turnIdx := atomic.Int64{}
	llm := &stubLLM{}
	llm.fn = func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		n := turnIdx.Add(1)
		var usage llmgateway.TokenUsage
		switch n {
		case 1:
			usage = llmgateway.TokenUsage{PromptTokens: 100, CompletionTokens: 300} // total 400
		case 2:
			usage = llmgateway.TokenUsage{PromptTokens: 100, CompletionTokens: 300} // total 800
		default:
			usage = llmgateway.TokenUsage{PromptTokens: 100, CompletionTokens: 100} // total 1000
		}
		ch := make(chan llmgateway.Chunk, 4)
		ch <- llmgateway.Chunk{
			ToolCalls: []llmgateway.ToolCall{{Name: "read", ID: "tx", Input: "{}"}},
			Done:      true,
			Usage:     usage,
		}
		close(ch)
		return ch, nil
	}

	tools := &stubTools{results: []ToolResult{{ToolCallID: "tx", Output: "ok"}}}
	persist := &stubPersist{}

	// maxContextTokens=1000 comes from PreparedContext, not the deps.
	// The deps-level MaxContextTokens is just a fallback for emitComplete.
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm,
		Context: &stubContext{prepared: PreparedContext{
			MaxContextTokens: 1000,
		}},
		Tools:            tools,
		Persist:          persist,
		MaxTurns:         20, // generous so the safety net never fires
		MaxContextTokens: 1000,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-diminishing",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "loop with diminishing returns"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	complete := findCompleteEvent(evs)
	if complete == nil {
		t.Fatal("expected complete event")
	}
	if got := complete.Metadata["exit_reason"]; got != string(ExitReasonTokenDiminishing) {
		t.Errorf("exit_reason = %q, want %q", got, ExitReasonTokenDiminishing)
	}
	if turnIdx.Load() != 3 {
		t.Errorf("expected 3 LLM calls (3rd trips detector), got %d", turnIdx.Load())
	}
}

// TestOrchestrator_RunTurn_NestedBranch_BudgetInjection_DM-20260620-002
//
// Phase C AC1 — nested branch must honor TurnRequest.MaxContextTokens so
// runTokenAudit + ShouldFoldProactively fire normally. Before the fix,
// nested runLoop skipped Prepare and left maxContextTokens at its zero
// value, which made all four budget controls no-op and let 4-parallel
// deep-review prompts grow past 100K tokens.
//
// We pre-load a large assistant message into PreloadedMessages (mirrors a
// mid-flight deep-review where previous tool rounds have accumulated an
// over-budget assistant reply), then drive a SubQuery-scope turn with an
// explicit MaxContextTokens. The audit must detect the oversized
// assistant, fold it via ToolResultStore, and let the run finish.
func TestOrchestrator_RunTurn_NestedBranch_BudgetInjection_DM_20260620_002(t *testing.T) {
	// 80K-char assistant message already sitting in the sub-agent's
	// preloaded history (e.g. accumulated by prior tool rounds).
	oversizedAssistant := strings.Repeat("a", 80000)
	longSystem := strings.Repeat("system-context-line-", 4000) // ~96K chars

	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("sub-agent summary"),
		doneChunk(),
	}}
	tools := &stubTools{}
	stubPersist := &stubPersist{}
	store := persist.NewToolResultStore(t.TempDir())

	orch := NewOrchestrator(OrchestratorDeps{
		LLM:               llm,
		Context:           &stubContext{prepared: PreparedContext{}}, // never invoked in nested branch
		Tools:             tools,
		Persist:           stubPersist,
		MaxTurns:          5,
		MaxContextTokens:  32000, // fallback (not used here, req sets it)
		ToolResultStore:   store,
		MaxAssistantChars: 8000,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-nested-budget",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "review the project"},
		SystemPrompt: longSystem,
		Scope:       TurnScopeSubQuery, // nested branch
		SkipPersist: true,
		PreloadedMessages: []types.Message{
			{Role: types.MessageRoleAssistant, Content: oversizedAssistant},
		},
		MaxContextTokens: 32000, // explicit injection — AC1
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	if hasType(evs, "error") {
		t.Fatalf("unexpected error event: %v", evs)
	}

	// After the proactive fold, the assistant message in PreloadedMessages
	// is replaced with the disk-persisted preview (which carries the
	// "Output too large" marker). The fold happens in-place inside
	// runTokenAudit, mutating messages; the next LLM call sees the
	// trimmed payload and the budget tracker no longer trips.
	if llm.calls.Load() == 0 {
		t.Fatal("LLM was never called")
	}
}

// TestOrchestrator_RunTurn_NestedBranch_FallbackToDeps_PhaseA_AC1_DM-20260620-002
//
// When SubTurnRunner does not set TurnRequest.MaxContextTokens (legacy
// callers), the nested branch must fall back to o.maxContextTokens from
// OrchestratorDeps (the Phase A wiring).
func TestOrchestrator_RunTurn_NestedBranch_FallbackToDeps_PhaseA_AC1_DM_20260620_002(t *testing.T) {
	oversizedAssistant := strings.Repeat("a", 80000)
	longSystem := strings.Repeat("system-context-line-", 4000)

	llm := &stubLLM{chunks: []llmgateway.Chunk{
		textChunk("fallback summary"),
		doneChunk(),
	}}
	tools := &stubTools{}
	stubPersist := &stubPersist{}
	store := persist.NewToolResultStore(t.TempDir())

	orch := NewOrchestrator(OrchestratorDeps{
		LLM:               llm,
		Context:           &stubContext{prepared: PreparedContext{}},
		Tools:             tools,
		Persist:           stubPersist,
		MaxTurns:          5,
		MaxContextTokens:  32000, // <- fallback used when request omits it
		ToolResultStore:   store,
		MaxAssistantChars: 8000,
	})

	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "sess-nested-fallback",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "fallback test"},
		SystemPrompt: longSystem,
		Scope:       TurnScopeSubQuery,
		SkipPersist: true,
		PreloadedMessages: []types.Message{
			{Role: types.MessageRoleAssistant, Content: oversizedAssistant},
		},
		// MaxContextTokens intentionally left zero — fallback path
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	if hasType(evs, "error") {
		t.Fatalf("unexpected error: %v", evs)
	}
	if llm.calls.Load() == 0 {
		t.Fatal("LLM was never called")
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

