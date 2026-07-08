package testutil

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// D7LLMStub is a minimal IAdapter for D7 integration tests.
type D7LLMStub struct {
	Response string
	// Delay blocks Stream until elapsed or ctx is cancelled (interrupt tests).
	Delay time.Duration
	// CallCount tracks total Stream invocations across all sessions. Tests
	// use this to verify orthogonal dispatch (e.g. CommandHandler must not
	// call LLM; CallCount stays at 0).
	CallCount atomic.Int64
}

func (s *D7LLMStub) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	s.CallCount.Add(1)
	if s.Delay > 0 {
		timer := time.NewTimer(s.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	resp := s.Response
	if resp == "" {
		resp = "D7 integration OK"
	}
	ch := make(chan *llmgateway.AdapterChunk, 2)
	ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{Content: resp}}
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{
			Done:  true,
			Usage: llmgateway.TokenUsage{PromptTokens: 5, CompletionTokens: 3},
		},
	}
	close(ch)
	return ch, nil
}

func (s *D7LLMStub) Provider() string { return "deepseek" }

func (s *D7LLMStub) Protocol() string { return adapter.ProtocolStub }

// SequenceLLMStub is an IAdapter that returns different Responses for each
// Stream call. Each element of Responses is a sequence of chunks emitted
// for one invocation (call 1 → Responses[0], call 2 → Responses[1], etc.).
// If more calls are made than configured Responses, it reuses the last entry.
//
// FrameDeltaInject (DM-20260706-001, AC1) is an OPTIONAL test-only callback
// invoked once per Stream call with the call index. When non-nil, its return
// value is captured into LastFrameDelta (read by tests asserting the LLM
// stub emitted a non-zero Plan-side FrameDelta). TESTUTIL ONLY — production
// code never reads it; the callback exists to let integration tests assert
// "Plan LLM output included execution_mode / deliverable_contract" without
// standing up a real LLM.
type SequenceLLMStub struct {
	Responses [][]llmgateway.Chunk
	CallCount atomic.Int64

	// FrameDeltaInject is called per Stream call with idx (0-based). When
	// non-nil, the returned FrameDelta is stored into LastFrameDelta so
	// tests can assert the Plan LLM emitted a typed Plan-side delta.
	// TESTUTIL ONLY — not read by production code.
	FrameDeltaInject func(idx int) interfaces.FrameDelta

	// LastFrameDelta holds the most recent value returned by
	// FrameDeltaInject. atomic.Pointer so concurrent Stream goroutines
	// don't race when reading it from the test main goroutine.
	LastFrameDelta atomic.Pointer[interfaces.FrameDelta]
}

func (s *SequenceLLMStub) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	idx := int(s.CallCount.Add(1)) - 1
	if s.FrameDeltaInject != nil {
		fd := s.FrameDeltaInject(idx)
		s.LastFrameDelta.Store(&fd)
	}
	chunks := s.pickResponses(idx)

	ch := make(chan *llmgateway.AdapterChunk, len(chunks)+1)
	for _, c := range chunks {
		ch <- &llmgateway.AdapterChunk{Parsed: &llmgateway.Chunk{
			Content:   c.Content,
			Thinking:  c.Thinking,
			ToolCalls: c.ToolCalls,
			Done:      c.Done,
			Usage:     c.Usage,
		}}
		if c.Done {
			close(ch)
			return ch, nil
		}
	}
	// If no Done chunk in the sequence, append one
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{
			Done:  true,
			Usage: llmgateway.TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		},
	}
	close(ch)
	return ch, nil
}

func (s *SequenceLLMStub) pickResponses(idx int) []llmgateway.Chunk {
	if len(s.Responses) == 0 {
		return []llmgateway.Chunk{{Content: "D7 integration OK"}}
	}
	if idx < len(s.Responses) {
		return s.Responses[idx]
	}
	return s.Responses[len(s.Responses)-1]
}

func (s *SequenceLLMStub) Provider() string { return "deepseek" }

func (s *SequenceLLMStub) Protocol() string { return adapter.ProtocolStub }
