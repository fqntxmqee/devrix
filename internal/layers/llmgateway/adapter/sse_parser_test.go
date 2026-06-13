package adapter

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

func TestStreamOpenAISSE_should_parse_content_and_usage(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"Hello"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":" world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var content strings.Builder
	var usage llmgateway.TokenUsage
	var done bool

	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		content.WriteString(chunk.Content)
		if chunk.Usage.PromptTokens > 0 {
			usage = chunk.Usage
		}
		if chunk.Done {
			done = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if content.String() != "Hello world" {
		t.Errorf("content: %q", content.String())
	}
	if usage.PromptTokens != 3 || usage.CompletionTokens != 2 {
		t.Errorf("usage: %+v", usage)
	}
	if !done {
		t.Error("expected done")
	}
}

func TestStreamOpenAISSE_should_parse_usage_token_details(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cached_tokens":80},"completion_tokens_details":{"reasoning_tokens":30}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var usage llmgateway.TokenUsage
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		if chunk.Usage.PromptTokens > 0 {
			usage = chunk.Usage
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if usage.CacheReadTokens != 80 {
		t.Fatalf("cache read: %d", usage.CacheReadTokens)
	}
	if usage.ReasoningTokens != 30 {
		t.Fatalf("reasoning: %d", usage.ReasoningTokens)
	}
}

func TestStreamOpenAISSE_should_merge_tool_call_deltas(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"bash","arguments":"hel"}}]}}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"lo"}}]}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var lastTools []llmgateway.ToolCall
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		if len(chunk.ToolCalls) > 0 {
			lastTools = chunk.ToolCalls
		}
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(lastTools) != 1 {
		t.Fatalf("tools: %v", lastTools)
	}
	if lastTools[0].Name != "bash" || lastTools[0].Input != "hello" {
		t.Errorf("tool: %+v", lastTools[0])
	}
}

func TestStreamOpenAISSE_should_return_error_on_error_event(t *testing.T) {
	body := strings.Join([]string{
		`data: {"error":{"message":"tool call result does not follow tool call (2013)","type":"invalid_request_error","code":"2013"}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error from SSE error event")
	}
	if !strings.Contains(err.Error(), "2013") {
		t.Fatalf("error should contain error code: %v", err)
	}
	if !strings.Contains(err.Error(), "tool call result does not follow") {
		t.Fatalf("error should contain error message: %v", err)
	}
}

func TestStreamOpenAISSE_should_ignore_error_when_event_has_no_error_field(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"ok"}}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var content string
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		content += chunk.Content
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if content != "ok" {
		t.Fatalf("content = %q", content)
	}
}

func TestStreamOpenAISSE_should_parse_reasoning_content(t *testing.T) {
	body := "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think\"}}]}\n\ndata: [DONE]\n\n"
	var thinking string
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		thinking += chunk.Thinking
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if thinking != "think" {
		t.Errorf("thinking: %q", thinking)
	}
}
