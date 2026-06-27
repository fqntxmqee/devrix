package stream

import (
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
)

func TestBuildStreamResponseInfo_LogContentOn(t *testing.T) {
	incident.ConfigureLLMLogging(incident.LLMLogSettings{LogContent: true})
	t.Cleanup(func() { incident.ConfigureLLMLogging(incident.LLMLogSettings{}) })

	cap := newStreamResponseCapture()
	cap.observe(llmgateway.Chunk{
		Content:      "hello world",
		Thinking:     "plan step",
		FinishReason: "stop",
		ToolCalls: []llmgateway.ToolCall{{
			ID: "tc1", Name: "bash", Input: `{"command":"ls"}`,
		}},
	})
	info := buildStreamResponseInfo(nil, llmgateway.TokenUsage{PromptTokens: 10, CompletionTokens: 5}, "minimax", "MiniMax-M3", cap)
	if info["content"] != "hello world" {
		t.Fatalf("content = %v", info["content"])
	}
	if info["thinking"] != "plan step" {
		t.Fatalf("thinking = %v", info["thinking"])
	}
	bz, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(bz) {
		t.Fatalf("invalid json: %s", bz)
	}
}

func TestBuildStreamResponseInfo_LogContentOff(t *testing.T) {
	incident.ConfigureLLMLogging(incident.LLMLogSettings{LogContent: false})

	cap := newStreamResponseCapture()
	cap.observe(llmgateway.Chunk{Content: "secret full body"})
	info := buildStreamResponseInfo(nil, llmgateway.TokenUsage{}, "p", "m", cap)
	if _, ok := info["content"]; ok {
		t.Fatalf("content should be omitted when log_content off, got %v", info["content"])
	}
	if info["content_preview"] == "" {
		t.Fatal("expected content_preview")
	}
}
