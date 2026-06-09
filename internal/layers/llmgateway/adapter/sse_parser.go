package adapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

const sseDataPrefix = "data: "

// streamAccumulator merges incremental SSE deltas into chunks.
type streamAccumulator struct {
	toolCalls map[int]*mergedToolCall
}

type mergedToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{toolCalls: make(map[int]*mergedToolCall)}
}

func (a *streamAccumulator) apply(event openAIStreamEvent) *llmgateway.Chunk {
	if len(event.Choices) == 0 && event.Usage == nil {
		return nil
	}

	chunk := &llmgateway.Chunk{}
	hasDelta := false

	for _, choice := range event.Choices {
		delta := choice.Delta
		if delta.Content != "" {
			chunk.Content += delta.Content
			hasDelta = true
		}
		thinking := delta.ReasoningContent
		if thinking == "" {
			thinking = delta.Thinking
		}
		if thinking != "" {
			chunk.Thinking += thinking
			hasDelta = true
		}
		for _, tc := range delta.ToolCalls {
			merged := a.toolCalls[tc.Index]
			if merged == nil {
				merged = &mergedToolCall{}
				a.toolCalls[tc.Index] = merged
			}
			if tc.ID != "" {
				merged.id = tc.ID
			}
			if tc.Function.Name != "" {
				merged.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				merged.arguments.WriteString(tc.Function.Arguments)
			}
			hasDelta = true
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			chunk.Done = true
		}
	}

	if len(a.toolCalls) > 0 {
		chunk.ToolCalls = a.mergedToolCalls()
		hasDelta = true
	}

	if event.Usage != nil {
		chunk.Usage = llmgateway.TokenUsage{
			PromptTokens:     event.Usage.PromptTokens,
			CompletionTokens: event.Usage.CompletionTokens,
			TotalTokens:      event.Usage.TotalTokens,
		}
		hasDelta = true
	}

	if !hasDelta {
		return nil
	}
	return chunk
}

func (a *streamAccumulator) mergedToolCalls() []llmgateway.ToolCall {
	indices := make([]int, 0, len(a.toolCalls))
	for idx := range a.toolCalls {
		indices = append(indices, idx)
	}
	sortInts(indices)

	out := make([]llmgateway.ToolCall, 0, len(indices))
	for _, idx := range indices {
		merged := a.toolCalls[idx]
		id := strings.TrimSpace(merged.id)
		if id == "" {
			id = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), idx)
		}
		out = append(out, llmgateway.ToolCall{
			ID:    id,
			Name:  merged.name,
			Input: merged.arguments.String(),
		})
	}
	return out
}

func sortInts(nums []int) {
	for i := 0; i < len(nums); i++ {
		for j := i + 1; j < len(nums); j++ {
			if nums[j] < nums[i] {
				nums[i], nums[j] = nums[j], nums[i]
			}
		}
	}
}

func streamOpenAISSE(reader io.Reader, emit func(*llmgateway.Chunk) error) error {
	scanner := bufio.NewScanner(reader)
	acc := newStreamAccumulator()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, sseDataPrefix) {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, sseDataPrefix))
		if payload == "[DONE]" {
			final := &llmgateway.Chunk{Done: true}
			if err := emit(final); err != nil {
				return err
			}
			return nil
		}

		var event openAIStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return err
		}
		chunk := acc.apply(event)
		if chunk == nil {
			continue
		}
		if err := emit(chunk); err != nil {
			return err
		}
	}
	return scanner.Err()
}
