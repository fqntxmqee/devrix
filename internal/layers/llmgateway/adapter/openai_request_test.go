package adapter

import (
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestBuildOpenAIChatRequest_should_include_tools(t *testing.T) {
	req, err := buildOpenAIChatRequest(&llmgateway.Request{
		Model: "deepseek-v4-pro",
		Tools: []llmgateway.ToolSchema{{
			Name: "bash", Description: "run", Parameters: `{"type":"object","properties":{}}`,
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "bash" {
		t.Errorf("tools: %+v", req.Tools)
	}
	if !req.Stream {
		t.Error("expected stream true")
	}
}

func TestBuildOpenAIChatRequest_should_map_tool_call_messages(t *testing.T) {
	toolCallsJSON := `[{"id":"call_abc","type":"function","function":{"name":"call_cursor","arguments":"{\"task\":\"hi\"}"}}]`
	req, err := buildOpenAIChatRequest(&llmgateway.Request{
		Model: "MiniMax-M2.7-highspeed",
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "test cursor"},
			{
				Role:     types.MessageRoleAssistant,
				Metadata: map[string]string{"tool_calls": toolCallsJSON},
			},
			{
				Role:     types.MessageRoleTool,
				Content:  "ok",
				Metadata: map[string]string{"tool_call_id": "call_abc"},
			},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(req.Messages[1].ToolCalls) != 1 || req.Messages[1].ToolCalls[0].ID != "call_abc" {
		t.Fatalf("assistant tool_calls: %+v", req.Messages[1].ToolCalls)
	}
	if req.Messages[2].ToolCallID != "call_abc" {
		t.Fatalf("tool_call_id = %q", req.Messages[2].ToolCallID)
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("invalid json: %s", body)
	}
}

func TestBuildOpenAIChatRequest_should_skip_legacy_tool_message_without_call_id(t *testing.T) {
	req, err := buildOpenAIChatRequest(&llmgateway.Request{
		Model: "MiniMax-M2.7-highspeed",
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "test"},
			{Role: types.MessageRoleTool, Content: "orphan"},
			{
				Role:     types.MessageRoleTool,
				Content:  "ok",
				Metadata: map[string]string{"tool_call_id": "call_abc"},
			},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages = %d, want 2 (orphan skipped)", len(req.Messages))
	}
	if req.Messages[1].ToolCallID != "call_abc" {
		t.Fatalf("tool_call_id = %q", req.Messages[1].ToolCallID)
	}
}

func TestBuildOpenAIChatRequest_should_fill_empty_tool_call_ids(t *testing.T) {
	toolCallsJSON := `[{"id":"","type":"function","function":{"name":"read_file","arguments":"{}"}}]`
	req, err := buildOpenAIChatRequest(&llmgateway.Request{
		Model: "MiniMax-M2.7-highspeed",
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "test"},
			{
				Role:     types.MessageRoleAssistant,
				Metadata: map[string]string{"tool_calls": toolCallsJSON},
			},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(req.Messages) != 2 || len(req.Messages[1].ToolCalls) != 1 {
		t.Fatalf("messages: %+v", req.Messages)
	}
	if req.Messages[1].ToolCalls[0].ID == "" {
		t.Fatal("expected synthetic tool call id")
	}
}
