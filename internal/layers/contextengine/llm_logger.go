package contextengine

import (
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/types"
)

// LLMCallLogger captures LLM input/output for observability
type LLMCallLogger struct {
	iteration int
	model     string
}

// LLMCallInfo holds structured LLM call information for logging
type LLMCallInfo struct {
	Iteration     int        `json:"iteration"`
	Model         string     `json:"model"`
	MessageCount  int        `json:"message_count"`
	ToolCount    int        `json:"tool_count"`
	SystemLen    int        `json:"system_prompt_length"`
	Messages     []MsgInfo  `json:"messages,omitempty"`
	Tools        []ToolInfo `json:"tools,omitempty"`
	ResponseLen  int        `json:"response_length"`
	ToolCalls    []ToolCallInfo `json:"tool_calls,omitempty"`
}

// MsgInfo is a truncated message for logging
type MsgInfo struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolInfo is a tool for logging
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolCallInfo captures a tool call for logging
type ToolCallInfo struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// formatMessages truncates messages for logging (avoiding span bloat)
func formatMessages(msgs []types.Message, maxContentLen int) []MsgInfo {
	if len(msgs) == 0 {
		return nil
	}
	result := make([]MsgInfo, 0, len(msgs))
	for _, m := range msgs {
		content := m.Content
		if len(content) > maxContentLen {
			content = content[:maxContentLen] + "..."
		}
		info := MsgInfo{Role: string(m.Role), Content: content}
		// Handle tool calls in metadata
		if m.Metadata != nil {
			if tc, ok := m.Metadata["tool_calls"]; ok {
				info.Content = fmt.Sprintf("[tool_calls: %s]", truncate(tc, 200))
			}
			if tcResult, ok := m.Metadata["tool_call_id"]; ok {
				info.Content = fmt.Sprintf("[tool_result for %s: %s]", tcResult, truncate(content, 200))
			}
		}
		result = append(result, info)
	}
	return result
}

// formatToolCalls formats tool calls for logging
func formatToolCalls(calls []ToolCall, results []ToolResult) []ToolCallInfo {
	if len(calls) == 0 {
		return nil
	}
	info := make([]ToolCallInfo, 0, len(calls))
	for i, c := range calls {
		ci := ToolCallInfo{Name: c.Name, Input: truncate(c.Input, 300)}
		if len(results) > i {
			if results[i].Error != "" {
				ci.Error = truncate(results[i].Error, 200)
			} else {
				ci.Output = truncate(results[i].Output, 300)
			}
		}
		info = append(info, ci)
	}
	return info
}

// truncate truncates a string to maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// AddLLMRequestEvent adds LLM request info as a span event
func AddLLMRequestEvent(span tracer.Span, iter int, model string, req *LLMRequest) {
	if span == nil || !span.IsRecording() {
		return
	}

	info := LLMCallInfo{
		Iteration:    iter,
		Model:       model,
		MessageCount: len(req.Messages),
		ToolCount:   len(req.Tools),
		SystemLen:   len(req.SystemPrompt),
		Messages:    formatMessages(req.Messages, 500),
		Tools:       formatToolSchemas(req.Tools),
	}

	bz, _ := json.Marshal(info)
	span.AddEvent("llm.request", tracer.WithEventAttributes(
		tracer.Attribute{Key: "llm.iteration", Value: iter},
		tracer.Attribute{Key: "llm.model", Value: model},
		tracer.Attribute{Key: "llm.messages_count", Value: len(req.Messages)},
		tracer.Attribute{Key: "llm.tools_count", Value: len(req.Tools)},
		tracer.Attribute{Key: "llm.request_json", Value: string(bz)},
	))
}

// AddLLMResponseEvent adds LLM response info as a span event
func AddLLMResponseEvent(span tracer.Span, iter int, respLen int, usage TokenUsage, toolCalls []ToolCall, toolResults []ToolResult) {
	if span == nil || !span.IsRecording() {
		return
	}

	info := map[string]interface{}{
		"iteration":          iter,
		"response_length":    respLen,
		"prompt_tokens":      usage.PromptTokens,
		"completion_tokens":  usage.CompletionTokens,
		"tool_calls_count":   len(toolCalls),
	}

	bz, _ := json.Marshal(info)
	span.AddEvent("llm.response", tracer.WithEventAttributes(
		tracer.Attribute{Key: "llm.iteration", Value: iter},
		tracer.Attribute{Key: "llm.response_length", Value: respLen},
		tracer.Attribute{Key: "llm.tool_calls_count", Value: len(toolCalls)},
		tracer.Attribute{Key: "llm.response_json", Value: string(bz)},
	))
}

func formatToolSchemas(tools []ToolSchema) []ToolInfo {
	if len(tools) == 0 {
		return nil
	}
	result := make([]ToolInfo, 0, len(tools))
	for _, t := range tools {
		desc := t.Description
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		result = append(result, ToolInfo{Name: t.Name, Description: desc})
	}
	return result
}
