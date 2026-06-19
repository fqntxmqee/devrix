package adapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/textutil"
)

const sseDataPrefix = "data: "

// streamAccumulator merges incremental SSE deltas into chunks.
type streamAccumulator struct {
	toolCalls map[int]*mergedToolCall
	// lastUsage 缓存最近一次出现的 Usage（OpenAI 兼容协议中通常出现在
	// finish_reason 帧或独立 usage 帧，且早于 [DONE] 哨兵）。
	lastUsage        llmgateway.TokenUsage
	hasUsage         bool
	lastFinishReason string
	// thinkSplitter routes XML <think>...</think> blocks out of delta.Content
	// when the provider lacks a native reasoning field. The decision is
	// made per-event from the SSE payload: if delta.ReasoningContent or
	// delta.Thinking is non-empty, that wins and the splitter is bypassed.
	// Only when the provider sends reasoning inline (catalog capability
	// native_thinking: false) does the splitter run. The catalog itself
	// is consulted by the upstream adapter (see internal/layers/llmgateway/
	// configure/catalog.go) but the splitter stays on for safety — providers
	// can change behavior, and the worst case is "extra splitter work".
	// Stateful across all events so chunks straddling a tag boundary still
	// split correctly.
	thinkSplitter *textutil.ThinkTagSplitter
}

type mergedToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{
		toolCalls:     make(map[int]*mergedToolCall),
		thinkSplitter: &textutil.ThinkTagSplitter{},
	}
}

func (a *streamAccumulator) apply(event openAIStreamEvent) *llmgateway.Chunk {
	if len(event.Choices) == 0 && event.Usage == nil {
		return nil
	}

	chunk := &llmgateway.Chunk{}
	hasDelta := false

	for _, choice := range event.Choices {
		delta := choice.Delta
		thinking := delta.ReasoningContent
		if thinking == "" {
			thinking = delta.Thinking
		}
		if thinking != "" {
			// Provider-native reasoning field (DeepSeek-R1 reasoning_content,
			// Anthropic thinking). Trust the protocol: content is clean.
			chunk.Thinking += thinking
			hasDelta = true
			if delta.Content != "" {
				chunk.Content += delta.Content
				hasDelta = true
			}
		} else if delta.Content != "" {
			// No provider-native reasoning field on this delta — the provider
			// either uses inline <think> tags (catalog capability
			// native_thinking: false) or is unknown to the catalog. Route
			// through the stateful splitter so chunks straddling a
			// <think> / </think> boundary still separate correctly.
			thinkDelta, contentDelta := a.thinkSplitter.Push(delta.Content)
			if thinkDelta != "" {
				chunk.Thinking += thinkDelta
				hasDelta = true
			}
			if contentDelta != "" {
				chunk.Content += contentDelta
				hasDelta = true
			}
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
			chunk.FinishReason = *choice.FinishReason
			a.lastFinishReason = *choice.FinishReason
		}
	}

	if len(a.toolCalls) > 0 {
		chunk.ToolCalls = a.mergedToolCalls()
		hasDelta = true
	}

	if event.Usage != nil {
		usage := llmgateway.TokenUsage{
			PromptTokens:     event.Usage.PromptTokens,
			CompletionTokens: event.Usage.CompletionTokens,
			TotalTokens:      event.Usage.TotalTokens,
		}
		if event.Usage.PromptTokensDetails != nil {
			usage.CacheReadTokens = event.Usage.PromptTokensDetails.CachedTokens
		}
		if event.Usage.CompletionTokensDetails != nil {
			usage.ReasoningTokens = event.Usage.CompletionTokensDetails.ReasoningTokens
		}
		chunk.Usage = usage
		a.lastUsage = usage
		a.hasUsage = true
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
	sort.Ints(indices)

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
			// Flush any trailing thinking/content that the splitter was still
			// holding when the stream ended (e.g. a <think> block that never
			// closed — the splitter buffers it until Flush()).
			if thinkTail, contentTail := acc.thinkSplitter.Flush(); thinkTail != "" || contentTail != "" {
				tail := &llmgateway.Chunk{
					Thinking: thinkTail,
					Content:  contentTail,
				}
				if err := emit(tail); err != nil {
					return err
				}
			}
			final := &llmgateway.Chunk{Done: true, FinishReason: acc.lastFinishReason}
			if acc.hasUsage {
				final.Usage = acc.lastUsage
			}
			if err := emit(final); err != nil {
				return err
			}
			return nil
		}

		var event openAIStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return err
		}
		if event.Error != nil {
			slog.Warn("llm: provider SSE error",
				"message", event.Error.Message,
				"type", event.Error.Type,
				"code", event.Error.Code,
			)
			return fmt.Errorf("provider SSE error: %s (type=%s, code=%s)",
				event.Error.Message, event.Error.Type, event.Error.Code)
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
