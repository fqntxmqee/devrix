package contextengine

import (
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	defaultMsgTruncate    = 500
	defaultRespTruncate   = 2000
	defaultToolTruncate   = 300
	defaultToolDescLen    = 100
	spanPreviewTruncate   = 16384
)

// LLMCallLogger captures LLM input/output for observability.
type LLMCallLogger struct {
	iteration int
	model     string
}

// LLMCallInfo holds structured LLM call information for logging.
type LLMCallInfo struct {
	Iteration    int            `json:"iteration"`
	Model        string         `json:"model"`
	MessageCount int            `json:"message_count"`
	ToolCount    int            `json:"tool_count"`
	SystemLen    int            `json:"system_prompt_length"`
	SystemPrompt string         `json:"system_prompt,omitempty"`
	Messages     []MsgInfo      `json:"messages,omitempty"`
	Tools        []ToolInfo     `json:"tools,omitempty"`
	ResponseLen  int            `json:"response_length,omitempty"`
	Response     string         `json:"response,omitempty"`
	ToolCalls    []ToolCallInfo `json:"tool_calls,omitempty"`
	PromptTokens int            `json:"prompt_tokens,omitempty"`
	CompletionTokens int        `json:"completion_tokens,omitempty"`
}

// MsgInfo is a message snapshot for logging.
type MsgInfo struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolInfo is a tool schema snapshot for logging.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolCallInfo captures a tool call for logging.
type ToolCallInfo struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

func contentLimit(full bool, spanPreview bool) int {
	if full {
		return 0
	}
	if spanPreview {
		return spanPreviewTruncate
	}
	return defaultMsgTruncate
}

func responseLimit(full bool, spanPreview bool) int {
	if full {
		return 0
	}
	if spanPreview {
		return spanPreviewTruncate
	}
	return defaultRespTruncate
}

func toolFieldLimit(full bool) int {
	if full {
		return 0
	}
	return defaultToolTruncate
}

func toolDescLimit(full bool) int {
	if full {
		return 0
	}
	return defaultToolDescLen
}

// formatMessages truncates messages when maxContentLen > 0.
func formatMessages(msgs []types.Message, maxContentLen int) []MsgInfo {
	if len(msgs) == 0 {
		return nil
	}
	result := make([]MsgInfo, 0, len(msgs))
	for _, m := range msgs {
		content := m.Content
		if maxContentLen > 0 && len(content) > maxContentLen {
			content = content[:maxContentLen] + "..."
		}
		info := MsgInfo{Role: string(m.Role), Content: content}
		if m.Metadata != nil {
			if tc, ok := m.Metadata["tool_calls"]; ok {
				metaLimit := maxContentLen
				if metaLimit <= 0 {
					metaLimit = 200
				}
				info.Content = fmt.Sprintf("[tool_calls: %s]", truncate(tc, metaLimit))
			}
			if tcResult, ok := m.Metadata["tool_call_id"]; ok {
				metaLimit := maxContentLen
				if metaLimit <= 0 {
					metaLimit = 200
				}
				info.Content = fmt.Sprintf("[tool_result for %s: %s]", tcResult, truncate(content, metaLimit))
			}
		}
		result = append(result, info)
	}
	return result
}

// formatToolCalls formats tool calls for logging.
func formatToolCalls(calls []ToolCall, results []ToolResult, maxFieldLen int) []ToolCallInfo {
	if len(calls) == 0 {
		return nil
	}
	info := make([]ToolCallInfo, 0, len(calls))
	for i, c := range calls {
		ci := ToolCallInfo{Name: c.Name, Input: truncate(c.Input, maxFieldLen)}
		if len(results) > i {
			if results[i].Error != "" {
				ci.Error = truncate(results[i].Error, maxFieldLen)
			} else {
				ci.Output = truncate(results[i].Output, maxFieldLen)
			}
		}
		info = append(info, ci)
	}
	return info
}

// truncate truncates a string when maxLen > 0.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildRequestInfo(iter int, model string, req *llmgateway.Request, full bool, spanPreview bool) LLMCallInfo {
	msgLimit := contentLimit(full, spanPreview)
	tools := make([]ToolSchema, len(req.Tools))
	for i, t := range req.Tools {
		tools[i] = ToolSchema{Name: t.Name, Description: t.Description, Parameters: t.Parameters}
	}
	info := LLMCallInfo{
		Iteration:    iter,
		Model:        model,
		MessageCount: len(req.Messages),
		ToolCount:    len(req.Tools),
		SystemLen:    len(req.SystemPrompt),
		Messages:     formatMessages(req.Messages, msgLimit),
		Tools:        formatToolSchemas(tools, toolDescLimit(full)),
	}
	if full {
		info.SystemPrompt = req.SystemPrompt
	}
	return info
}

func buildResponseInfo(
	iter int,
	responseText string,
	usage TokenUsage,
	toolCalls []ToolCall,
	toolResults []ToolResult,
	full bool,
	spanPreview bool,
) map[string]interface{} {
	respLimit := responseLimit(full, spanPreview)
	toolLimit := toolFieldLimit(full)

	info := map[string]interface{}{
		"iteration":         iter,
		"response_length":   len(responseText),
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"tool_calls_count":  len(toolCalls),
	}
	if full {
		info["response"] = responseText
	} else {
		info["response_preview"] = truncate(responseText, respLimit)
	}
	if tc := formatToolCalls(toolCalls, toolResults, toolLimit); len(tc) > 0 {
		info["tool_calls"] = tc
	}
	return info
}

// AddLLMRequestEvent records LLM request info on the span and optionally to a local file.
func AddLLMRequestEvent(span tracer.Span, sessionID string, iter int, model string, req *llmgateway.Request) {
	settings := currentLLMLogSettings()
	fullForSpan := settings.LogContent
	info := buildRequestInfo(iter, model, req, fullForSpan, !fullForSpan)
	bz, _ := json.Marshal(info)
	observability.RecordLLMSpanPayload(
		span, sessionID, "request", "llm.request", "llm.request_json", string(bz),
		iter, model,
		tracer.Attribute{Key: "llm.iteration", Value: iter},
		tracer.Attribute{Key: "llm.model", Value: model},
		tracer.Attribute{Key: "llm.messages_count", Value: len(req.Messages)},
		tracer.Attribute{Key: "llm.tools_count", Value: len(req.Tools)},
	)
}

// AddLLMResponseEvent records LLM response info on the span and optionally to a local file.
func AddLLMResponseEvent(
	span tracer.Span,
	sessionID string,
	iter int,
	responseText string,
	usage TokenUsage,
	toolCalls []ToolCall,
	toolResults []ToolResult,
) {
	settings := currentLLMLogSettings()
	fullForSpan := settings.LogContent
	info := buildResponseInfo(iter, responseText, usage, toolCalls, toolResults, fullForSpan, !fullForSpan)
	bz, _ := json.Marshal(info)
	observability.RecordLLMSpanPayload(
		span, sessionID, "response", "llm.response", "llm.response_json", string(bz),
		iter, "",
		tracer.Attribute{Key: "llm.iteration", Value: iter},
		tracer.Attribute{Key: "llm.response_length", Value: len(responseText)},
		tracer.Attribute{Key: "llm.tool_calls_count", Value: len(toolCalls)},
	)
}

func formatToolSchemas(tools []ToolSchema, maxDescLen int) []ToolInfo {
	if len(tools) == 0 {
		return nil
	}
	result := make([]ToolInfo, 0, len(tools))
	for _, t := range tools {
		desc := truncate(t.Description, maxDescLen)
		result = append(result, ToolInfo{Name: t.Name, Description: desc})
	}
	return result
}
