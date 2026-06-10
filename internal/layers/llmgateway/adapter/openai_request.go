package adapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	metaToolCallID = "tool_call_id"
	metaToolCalls  = "tool_calls"
)

func buildOpenAIChatRequest(req *llmgateway.Request) (*openAIChatRequest, error) {
	if req == nil {
		return nil, errNilRequest
	}
	out := &openAIChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}
	if req.SystemPrompt != "" {
		out.Messages = append(out.Messages, openAIMessage{
			Role:    string(types.MessageRoleSystem),
			Content: req.SystemPrompt,
		})
	}
	for _, m := range req.Messages {
		om := mapOpenAIMessage(m)
		// Legacy sessions may contain duplicate tool rows without tool_call_id.
		if m.Role == types.MessageRoleTool && om.ToolCallID == "" {
			continue
		}
		out.Messages = append(out.Messages, om)
	}
	for _, tool := range req.Tools {
		params, err := parseToolParameters(tool.Parameters)
		if err != nil {
			return nil, err
		}
		out.Tools = append(out.Tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			},
		})
	}
	if !req.Stream {
		out.Stream = false
	}
	return out, nil
}

func mapOpenAIMessage(m types.Message) openAIMessage {
	out := openAIMessage{
		Role:    string(m.Role),
		Content: m.Content,
	}
	if m.Metadata == nil {
		return out
	}
	if raw, ok := m.Metadata[metaToolCalls]; ok && raw != "" {
		var calls []openAIToolCallMsg
		if err := json.Unmarshal([]byte(raw), &calls); err == nil && len(calls) > 0 {
			out.ToolCalls = sanitizeOpenAIToolCalls(calls)
		}
	}
	if id, ok := m.Metadata[metaToolCallID]; ok {
		id = strings.TrimSpace(id)
		if id == "" && out.Role == string(types.MessageRoleTool) {
			id = fmt.Sprintf("call_%d", time.Now().UnixNano())
		}
		if id != "" {
			out.ToolCallID = id
		}
	}
	return out
}

// sanitizeOpenAIToolCalls ensures provider-bound messages never send empty tool call IDs
// (MiniMax returns 400: tool call id is empty).
func sanitizeOpenAIToolCalls(calls []openAIToolCallMsg) []openAIToolCallMsg {
	out := make([]openAIToolCallMsg, 0, len(calls))
	for i, call := range calls {
		if strings.TrimSpace(call.ID) == "" {
			call.ID = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)
		}
		if call.Type == "" {
			call.Type = "function"
		}
		out = append(out, call)
	}
	return out
}

func parseToolParameters(raw string) (any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var params any
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil, err
	}
	return params, nil
}
