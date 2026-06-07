package adapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

func deepseekTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, sharedconfig.LLMProviderRuntimeConfig) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL:      srv.URL + "/v1",
		APIKeyEnv:    "DEEPSEEK_TEST_KEY",
		DefaultModel: "deepseek-v4-flash",
		MaxTokens:    1024,
		Temperature:  0.7,
	}
	t.Setenv("DEEPSEEK_TEST_KEY", "test-key")
	return srv, cfg
}

func collectChunks(t *testing.T, ch <-chan *llmgateway.AdapterChunk) (text string, usage llmgateway.TokenUsage, tools []llmgateway.ToolCall) {
	t.Helper()
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		if chunk.Parsed == nil {
			continue
		}
		text += chunk.Parsed.Content
		if chunk.Parsed.Usage.PromptTokens > 0 {
			usage = chunk.Parsed.Usage
		}
		if len(chunk.Parsed.ToolCalls) > 0 {
			tools = chunk.Parsed.ToolCalls
		}
	}
	return text, usage, tools
}

// Covers: L5-LLM-01
func TestDeepSeekAdapter_should_stream_v4_pro_response(t *testing.T) {
	srv, cfg := deepseekTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("auth: %s", auth)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "deepseek-v4-pro" {
			t.Errorf("model: %v", body["model"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":1,\"total_tokens\":5}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	ad := adapter.NewDeepSeekAdapter(cfg).WithHTTPClient(srv.Client())
	ch, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model: "deepseek-v4-pro",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "hello"),
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	text, usage, _ := collectChunks(t, ch)
	if text != "Hi" {
		t.Errorf("text: %q", text)
	}
	if usage.PromptTokens != 4 {
		t.Errorf("usage: %+v", usage)
	}
}

// Covers: L5-LLM-01
func TestDeepSeekAdapter_should_stream_v4_flash_response(t *testing.T) {
	srv, cfg := deepseekTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "deepseek-v4-flash" {
			t.Errorf("model: %v", body["model"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"fast\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	ad := adapter.NewDeepSeekAdapter(cfg).WithHTTPClient(srv.Client())
	ch, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model:        "deepseek-v4-flash",
		SystemPrompt: "You are helpful.",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "ping"),
		},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	text, _, _ := collectChunks(t, ch)
	if text != "fast" {
		t.Errorf("text: %q", text)
	}
}

// Covers: L5-LLM-01
func TestDeepSeekAdapter_should_stream_tool_calls(t *testing.T) {
	srv, cfg := deepseekTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"tc1","function":{"name":"bash","arguments":"{\"cmd\":\"ls\"}"}}]}}]}`+"\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	ad := adapter.NewDeepSeekAdapter(cfg).WithHTTPClient(srv.Client())
	ch, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model: "deepseek-v4-pro",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "run ls"),
		},
		Tools: []llmgateway.ToolSchema{{
			Name: "bash", Description: "shell", Parameters: `{"type":"object"}`,
		}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	_, _, tools := collectChunks(t, ch)
	if len(tools) != 1 || tools[0].Name != "bash" {
		t.Errorf("tools: %+v", tools)
	}
}

func TestDeepSeekAdapter_should_fail_without_api_key(t *testing.T) {
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL:   "http://example.com/v1",
		APIKeyEnv: "MISSING_DEEPSEEK_KEY",
	}
	ad := adapter.NewDeepSeekAdapter(cfg)
	_, err := ad.Stream(context.Background(), &llmgateway.Request{Model: "deepseek-v4-pro"})
	if err == nil {
		t.Fatal("expected auth error")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) || llmErr.Code != sharederrors.CodeLLMAuthFailed {
		t.Errorf("err: %v", err)
	}
}

func TestDeepSeekAdapter_should_map_auth_http_status(t *testing.T) {
	srv, cfg := deepseekTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	defer srv.Close()

	ad := adapter.NewDeepSeekAdapter(cfg).WithHTTPClient(srv.Client())
	_, err := ad.Stream(context.Background(), &llmgateway.Request{Model: "deepseek-v4-pro"})
	if err == nil {
		t.Fatal("expected error")
	}
	var llmErr *sharederrors.LLMError
	if !errors.As(err, &llmErr) || llmErr.Code != sharederrors.CodeLLMAuthFailed {
		t.Errorf("err: %v", err)
	}
}

func TestDeepSeekAdapter_should_respect_context_cancel(t *testing.T) {
	srv, cfg := deepseekTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"slow\"}}]}\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ad := adapter.NewDeepSeekAdapter(cfg).WithHTTPClient(srv.Client())
	ch, err := ad.Stream(ctx, &llmgateway.Request{Model: "deepseek-v4-flash"})
	if err != nil {
		// immediate cancel may fail at Do() or first read
		return
	}
	for range ch {
	}
}
