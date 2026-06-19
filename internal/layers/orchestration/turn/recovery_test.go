package turn

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/contracts"
	sherrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestIsContextLengthError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"code", sherrors.WithCode(sherrors.CodeContextExceeded, "ctx exceeded", errors.New("x")), true},
		{"413 text", errors.New("HTTP 413 prompt too long"), true},
		{"other", errors.New("500 internal"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContextLengthError(tc.err); got != tc.want {
				t.Fatalf("IsContextLengthError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsOverloadOr5xx(t *testing.T) {
	if !IsOverloadOr5xx(errors.New("503 service unavailable")) {
		t.Fatal("expected overload detection")
	}
	if IsOverloadOr5xx(sherrors.WithCode(sherrors.CodeContextExceeded, "ctx exceeded", errors.New("x"))) {
		t.Fatal("context length must not count as overload")
	}
}

func TestInvokeStreamWithRecovery_CompressesOn413(t *testing.T) {
	msgs := make([]types.Message, 21)
	for i := range msgs {
		msgs[i] = types.Message{Role: types.MessageRoleUser, Content: "msg"}
	}
	llm := &recoveryStubLLM{
		errs: []error{
			errors.New("context_length_exceeded"),
			nil,
			nil,
		},
		chunks: []llmgateway.Chunk{textChunk("compressed"), doneChunk()},
	}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM:      llm,
		Context:  &stubContext{prepared: PreparedContext{SystemPrompt: "sys"}},
		Tools:    &stubTools{},
		Persist:  &stubPersist{},
		MaxTurns: 1,
	})

	ch, err := orch.invokeStreamWithRecovery(context.Background(), TurnRequest{SessionID: "s1"}, LLMInvokeRequest{
		SessionID: "s1",
		Messages:  msgs,
	})
	if err != nil {
		t.Fatalf("invokeStreamWithRecovery: %v", err)
	}
	for range ch {
	}
	if llm.calls < 2 {
		t.Fatalf("llm calls = %d, want >= 2 (retry after compress)", llm.calls)
	}
}

func TestNeedsMaxOutputTokenRecovery(t *testing.T) {
	if !NeedsMaxOutputTokenRecovery("length") {
		t.Fatal("finish_reason=length should trigger recovery")
	}
	if NeedsMaxOutputTokenRecovery("stop") {
		t.Fatal("finish_reason=stop should not trigger recovery")
	}
}

func TestEmitStreamRecoveryTombstones(t *testing.T) {
	out := make(chan *contracts.EngineEvent, 4)
	emitStreamRecoveryTombstones(out, "s1", partialStreamEmit{
		hadText:     true,
		hadThinking: true,
		toolCalls:   []llmgateway.ToolCall{{Name: "read", ID: "c1"}},
	})
	close(out)
	var types []string
	for ev := range out {
		types = append(types, ev.Type+":"+ev.Metadata["rollback"])
	}
	want := []string{"tombstone:thinking", "tombstone:text", "tombstone:tool_call"}
	if len(types) != len(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events = %v, want %v", types, want)
		}
	}
}

func TestRunTurn_MaxOutputTokensRecovery_TombstoneAndRetry(t *testing.T) {
	calls := 0
	llm := &stubLLM{fn: func(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
		calls++
		ch := make(chan llmgateway.Chunk, 2)
		if calls == 1 {
			ch <- llmgateway.Chunk{Content: "partial", FinishReason: "length", Done: true}
		} else {
			ch <- llmgateway.Chunk{Content: "continued", FinishReason: "stop", Done: true}
		}
		close(ch)
		return ch, nil
	}}
	orch := NewOrchestrator(OrchestratorDeps{
		LLM: llm,
		Context: &stubContext{prepared: PreparedContext{
			SystemPrompt: "sys",
			Messages:     nil,
		}},
		Tools:    &stubTools{},
		Persist:  &stubPersist{},
		MaxTurns: 1,
	})
	ch, err := orch.RunTurn(context.Background(), TurnRequest{
		SessionID:   "s1",
		UserMessage: types.Message{Role: types.MessageRoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	evs := collectEvents(ch)
	if calls < 2 {
		t.Fatalf("llm calls = %d, want >= 2", calls)
	}
	if !hasType(evs, "tombstone") {
		t.Fatalf("expected tombstone event, got %v", eventTypes(evs))
	}
	if !hasType(evs, "complete") {
		t.Fatalf("expected complete event, got %v", eventTypes(evs))
	}
}

type recoveryStubLLM struct {
	errs   []error
	chunks []llmgateway.Chunk
	calls  int
}

func (s *recoveryStubLLM) InvokeStream(_ context.Context, _ LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.calls++
	idx := s.calls - 1
	if idx < len(s.errs) && s.errs[idx] != nil {
		return nil, s.errs[idx]
	}
	ch := make(chan llmgateway.Chunk, len(s.chunks))
	for _, c := range s.chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}
