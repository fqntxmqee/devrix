package stream

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
)

const tracePreviewLen = 500

type streamResponseCapture struct {
	content      strings.Builder
	thinking     strings.Builder
	toolCalls    []llmgateway.ToolCall
	finishReason string
}

func newStreamResponseCapture() *streamResponseCapture {
	return &streamResponseCapture{}
}

func (c *streamResponseCapture) observe(chunk llmgateway.Chunk) {
	if c == nil {
		return
	}
	if chunk.Content != "" {
		c.content.WriteString(chunk.Content)
	}
	if chunk.Thinking != "" {
		c.thinking.WriteString(chunk.Thinking)
	}
	// SSE parser re-emits the full merged tool-call list on every delta
	// frame. Keep the latest batch (same as WorkItemExecutor.streamLLM)
	// so Jaeger llm.response_json does not duplicate identical call IDs.
	if len(chunk.ToolCalls) > 0 {
		c.toolCalls = chunk.ToolCalls
	}
	if chunk.FinishReason != "" {
		c.finishReason = chunk.FinishReason
	}
}

func truncateForTrace(s string, full bool) string {
	if full || len(s) <= tracePreviewLen {
		return s
	}
	return s[:tracePreviewLen] + "..."
}

func contentDigest(s string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func buildStreamResponseInfo(
	err error,
	usage llmgateway.TokenUsage,
	provider, model string,
	cap *streamResponseCapture,
) map[string]interface{} {
	full := incident.LLMLogContentEnabled()
	info := map[string]interface{}{
		"provider":          provider,
		"model":             model,
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
	}
	if err != nil {
		info["error"] = err.Error()
	}
	if cap == nil {
		return info
	}

	content := cap.content.String()
	thinking := cap.thinking.String()
	info["content_len"] = len(content)
	info["content_hash"] = contentDigest(content)
	info["content_preview"] = truncateForTrace(content, false)
	if full {
		info["content"] = content
	}
	if thinking != "" {
		info["thinking_len"] = len(thinking)
		info["thinking_preview"] = truncateForTrace(thinking, false)
		if full {
			info["thinking"] = thinking
		}
	}
	if cap.finishReason != "" {
		info["finish_reason"] = cap.finishReason
	}
	toolCalls := dedupeToolCallsForTrace(cap.toolCalls)
	if len(toolCalls) > 0 {
		info["tool_calls_count"] = len(toolCalls)
		info["tool_calls"] = summarizeToolCallsForTrace(toolCalls, full)
	}
	return info
}

func dedupeToolCallsForTrace(calls []llmgateway.ToolCall) []llmgateway.ToolCall {
	if len(calls) <= 1 {
		return calls
	}
	seen := make(map[string]struct{}, len(calls))
	out := make([]llmgateway.ToolCall, 0, len(calls))
	for i, tc := range calls {
		id := strings.TrimSpace(tc.ID)
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(tc.ID) == "" {
			tc.ID = id
		}
		out = append(out, tc)
	}
	return out
}

func summarizeToolCallsForTrace(calls []llmgateway.ToolCall, full bool) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(calls))
	for _, tc := range calls {
		entry := map[string]interface{}{
			"id":   tc.ID,
			"name": tc.Name,
		}
		input := strings.TrimSpace(tc.Input)
		if input != "" {
			entry["input_preview"] = truncateForTrace(input, false)
			if full {
				entry["input"] = input
			}
		}
		out = append(out, entry)
	}
	return out
}
