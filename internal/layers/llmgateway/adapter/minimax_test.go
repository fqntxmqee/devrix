package adapter_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func minimaxTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, sharedconfig.LLMProviderRuntimeConfig) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL:      srv.URL + "/v1",
		APIKeyEnv:    "MINIMAX_TEST_KEY",
		DefaultModel: "minimax-3",
		MaxTokens:    1024,
		Temperature:  0.7,
	}
	t.Setenv("MINIMAX_TEST_KEY", "test-key")
	return srv, cfg
}

// T: D3-S1-A01-T02
func TestMiniMaxAdapter_should_stream_minimax_3_response(t *testing.T) {
	srv, cfg := minimaxTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "minimax-3" {
			t.Errorf("model: %v", body["model"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"mini\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	ad := adapter.NewMiniMaxAdapter(cfg).WithHTTPClient(srv.Client())
	ch, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model: "minimax-3",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "hi"),
		},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	text, _, _ := collectChunks(t, ch)
	if text != "mini" {
		t.Errorf("text: %q", text)
	}
}

// T: D3-S1-A01-T02
func TestMiniMaxAdapter_should_stream_highspeed_response(t *testing.T) {
	srv, cfg := minimaxTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "minimax-2.7-highspeed" {
			t.Errorf("model: %v", body["model"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"fast\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	})
	defer srv.Close()

	ad := adapter.NewMiniMaxAdapter(cfg).WithHTTPClient(srv.Client())
	ch, err := ad.Stream(context.Background(), &llmgateway.Request{
		Model: "minimax-2.7-highspeed",
		Messages: []types.Message{
			*types.NewMessage("1", "s", types.MessageRoleUser, "speed"),
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
