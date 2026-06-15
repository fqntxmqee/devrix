package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// CaptureLLMCaller records the last tool list from each Call.
type CaptureLLMCaller struct {
	Response  string
	Err       error
	LastTools []contracts.ToolSchema
}

// Call implements contracts.LLMCaller and captures req.Tools.
func (c *CaptureLLMCaller) Call(ctx context.Context, req contracts.LLMRequest) (<-chan contracts.LLMChunk, error) {
	c.LastTools = append([]contracts.ToolSchema(nil), req.Tools...)
	return (&StaticLLMCaller{Response: c.Response, Err: c.Err}).Call(ctx, req)
}

type StaticLLMCaller struct {
	Response string
	Err      error
	ToolCall *contracts.ToolCall
}

// Call implements contracts.LLMCaller.
func (m *StaticLLMCaller) Call(ctx context.Context, req contracts.LLMRequest) (<-chan contracts.LLMChunk, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	ch := make(chan contracts.LLMChunk, 2)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if m.ToolCall != nil {
			ch <- contracts.LLMChunk{
				ToolCalls: []contracts.ToolCall{*m.ToolCall},
				Done:      true,
				Usage:     contracts.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
			}
			return
		}
		text := m.Response
		if text == "" {
			text = "I can help you with that."
		}
		ch <- contracts.LLMChunk{
			Content: text,
			Done:    true,
			Usage:   contracts.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
		}
	}()
	return ch, nil
}

// StaticSummarizer is a test double for contracts.Summarizer.
type StaticSummarizer struct {
	Summary string
	Err     error
}

// Summarize implements contracts.Summarizer.
func (s *StaticSummarizer) Summarize(_ context.Context, _, _ string, _ int) (string, error) {
	if s.Err != nil {
		return "", s.Err
	}
	if s.Summary != "" {
		return s.Summary, nil
	}
	return "summary", nil
}

// LLMCallerWithTools returns a caller that emits a bash tool call.
func LLMCallerWithTools() contracts.LLMCaller {
	return &StaticLLMCaller{
		ToolCall: &contracts.ToolCall{ID: "tc1", Name: "bash", Input: "ls"},
	}
}
