package turn

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

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
}

func (s *stubTools) ExecuteRound(_ context.Context, req ToolRoundRequest) (ToolRoundResult, error) {
	s.lastCalls = append([]llmgateway.ToolCall(nil), req.ToolCalls...)
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

// --- MaxTurns default when request has 0 ---

func TestOrchestrator_RunTurn_DefaultMaxTurns(t *testing.T) {
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
		MaxTurns:    0, // should use default 5
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)

	if !hasType(evs, "complete") {
		t.Error("expected complete event")
	}
}

// --- Default MaxTurns in constructor when 0 passed ---

func TestNewOrchestrator_DefaultMaxTurns(t *testing.T) {
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: &stubLLM{}, Context: &stubContext{}, Tools: &stubTools{}, Persist: &stubPersist{},
		MaxTurns: 0,
	})
	if orch.maxTurns != 8 {
		t.Errorf("expected default maxTurns=8, got %d", orch.maxTurns)
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

