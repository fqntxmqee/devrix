package testutil

import (
	"context"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
)

// D7LLMStub is a minimal IAdapter for D7 integration tests.
type D7LLMStub struct {
	Response string
	// Delay blocks Stream until elapsed or ctx is cancelled (interrupt tests).
	Delay time.Duration
}

func (s *D7LLMStub) Stream(ctx context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
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
