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

// TestStreamOpenAISSE_should_separate_distinct_tool_calls_with_same_index
// covers a provider (minimax M2.7) bug: when the model emits two parallel
// tool_use blocks in the same assistant turn, the second delta incorrectly
// reuses index=0 instead of advancing to index=1. A naive parser would
// overwrite the first call's id/name and concatenate the args into a single
// garbled JSON object, which then fails validation server-side with HTTP
// 400 "invalid function arguments json string" (provider code 2013).
func TestStreamOpenAISSE_should_separate_distinct_tool_calls_with_same_index(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_function_aaa","function":{"name":"bash","arguments":"{\"description\": \"list repo\", \"command\": \"ls -la\"}"}}]}}]}`,
		``,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_function_bbb","function":{"name":"glob","arguments":"{\"pattern\": \"**/*.go\"}"}}]}}]}`,
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
	if len(lastTools) != 2 {
		t.Fatalf("expected 2 distinct tool calls, got %d: %+v", len(lastTools), lastTools)
	}
	var bash, globCall *llmgateway.ToolCall
	for i := range lastTools {
		switch lastTools[i].ID {
		case "call_function_aaa":
			bash = &lastTools[i]
		case "call_function_bbb":
			globCall = &lastTools[i]
		}
	}
	if bash == nil || globCall == nil {
		t.Fatalf("missing call ids in: %+v", lastTools)
	}
	if bash.Name != "bash" {
		t.Errorf("bash name = %q, want bash", bash.Name)
	}
	wantBashArgs := `{"description": "list repo", "command": "ls -la"}`
	if bash.Input != wantBashArgs {
		t.Errorf("bash args = %q, want %q", bash.Input, wantBashArgs)
	}
	if globCall.Name != "glob" {
		t.Errorf("glob name = %q, want glob", globCall.Name)
	}
	wantGlobArgs := `{"pattern": "**/*.go"}`
	if globCall.Input != wantGlobArgs {
		t.Errorf("glob args = %q, want %q", globCall.Input, wantGlobArgs)
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

// TestStreamOpenAISSE_should_split_inline_think_tags covers the minimax M2.7
// case: the LLM emits <think>...</think> inside delta.Content (no provider-native
// reasoning field). The splitter must route the inner text to chunk.Thinking
// and the trailing text to chunk.Content.
func TestStreamOpenAISSE_should_split_inline_think_tags(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"<think>用户想让我统计"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"...让我列出来</think>"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"\n\n让我统计一下："}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"\n1. bash"},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var thinking, content strings.Builder
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		thinking.WriteString(chunk.Thinking)
		content.WriteString(chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantThinking := "用户想让我统计...让我列出来"
	if thinking.String() != wantThinking {
		t.Errorf("thinking = %q, want %q", thinking.String(), wantThinking)
	}
	wantContent := "\n\n让我统计一下：\n1. bash"
	if content.String() != wantContent {
		t.Errorf("content = %q, want %q", content.String(), wantContent)
	}
}

// TestStreamOpenAISSE_should_split_think_tag_straddling_chunks covers a
// pathological split: <think> in chunk N, the entire body in chunk N+1, and
// </think> in chunk N+2. The splitter is stateful and must hold the body
// until it sees the closing tag.
func TestStreamOpenAISSE_should_split_think_tag_straddling_chunks(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"<think>"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"deep thought across"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":" multiple chunks"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"</think>answer"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":" tail"},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var thinking, content strings.Builder
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		thinking.WriteString(chunk.Thinking)
		content.WriteString(chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantThinking := "deep thought across multiple chunks"
	if thinking.String() != wantThinking {
		t.Errorf("thinking = %q, want %q", thinking.String(), wantThinking)
	}
	wantContent := "answer tail"
	if content.String() != wantContent {
		t.Errorf("content = %q, want %q", content.String(), wantContent)
	}
}

// TestStreamOpenAISSE_should_flush_unclosed_think_at_stream_end covers a
// pathological case: the LLM emits <think> but never closes it (truncation
// or model laziness). The splitter buffers the body and Flush() must
// release it on [DONE] so the thinking isn't silently dropped.
func TestStreamOpenAISSE_should_flush_unclosed_think_at_stream_end(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"<think>unfinished thought"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":" still going"}}],"finish_reason":"length"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var thinking, content strings.Builder
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		thinking.WriteString(chunk.Thinking)
		content.WriteString(chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Unclosed <think> → all body is thinking, no content.
	if thinking.String() != "unfinished thought still going" {
		t.Errorf("thinking = %q", thinking.String())
	}
	if content.Len() != 0 {
		t.Errorf("content = %q, want empty", content.String())
	}
}

// TestStreamOpenAISSE_should_not_split_when_native_reasoning_present covers
// the DeepSeek-R1 / Anthropic path: when the provider already populates
// delta.ReasoningContent, the splitter MUST NOT touch delta.Content (which
// is already clean). Splitting clean content would still be a no-op for
// well-behaved content, but a regression in the splitter could re-tag
// clean content as thinking. This test pins the precedence.
func TestStreamOpenAISSE_should_not_split_when_native_reasoning_present(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"native","content":"<think>should"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":" not split</think>clean"}}],"finish_reason":"stop"}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var thinking, content strings.Builder
	err := streamOpenAISSE(strings.NewReader(body), func(chunk *llmgateway.Chunk) error {
		thinking.WriteString(chunk.Thinking)
		content.WriteString(chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Native reasoning wins; subsequent <think> tags in delta.Content are
	// passed through verbatim (the splitter was never engaged).
	if thinking.String() != "native" {
		t.Errorf("thinking = %q, want %q", thinking.String(), "native")
	}
	wantContent := "<think>should not split</think>clean"
	if content.String() != wantContent {
		t.Errorf("content = %q, want %q", content.String(), wantContent)
	}
}
