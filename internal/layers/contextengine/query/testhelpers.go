package query

import (
	"context"
	"sync"
)

// SequentialLLM is a test double that returns scripted chunk sequences per call.
type SequentialLLM struct {
	mu        sync.Mutex
	Responses []LLMScript
	Calls     int
}

// LLMScript defines one LLM call outcome.
type LLMScript struct {
	Content   string
	ToolCalls []ToolCall
}

// Call implements contracts.LLMCaller for test doubles.
func (m *SequentialLLM) Call(ctx context.Context, _ LLMRequest) (<-chan LLMChunk, error) {
	m.mu.Lock()
	idx := m.Calls
	m.Calls++
	var script LLMScript
	if idx < len(m.Responses) {
		script = m.Responses[idx]
	}
	m.mu.Unlock()

	ch := make(chan LLMChunk, 2)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		ch <- LLMChunk{
			Content:   script.Content,
			ToolCalls: script.ToolCalls,
			Done:      true,
			Usage:     TokenUsage{PromptTokens: 1, CompletionTokens: 1},
		}
	}()
	return ch, nil
}

// RecordingToolExecutor records executions and returns configured output.
type RecordingToolExecutor struct {
	Calls  []ToolCall
	Output string
}

func (r *RecordingToolExecutor) Execute(_ context.Context, call ToolCall) (string, string, error) {
	r.Calls = append(r.Calls, call)
	out := r.Output
	if out == "" {
		out = "tool-ok"
	}
	return out, "", nil
}

// AllowPermission always approves.
type AllowPermission struct{}

func (AllowPermission) Request(context.Context, string, string, string) bool { return true }
