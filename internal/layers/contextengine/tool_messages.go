package contextengine

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

const (
	metaToolCallID = "tool_call_id"
	metaToolCalls  = "tool_calls"
)

type serializedToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func ensureToolCallID(call ToolCall, index int) string {
	if id := call.ID; id != "" {
		return id
	}
	return fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), index)
}

func buildAssistantToolCallsMessage(sessionID string, calls []ToolCall) types.Message {
	refs := make([]serializedToolCall, len(calls))
	for i, call := range calls {
		refs[i].ID = ensureToolCallID(call, i)
		refs[i].Type = "function"
		refs[i].Function.Name = call.Name
		refs[i].Function.Arguments = normalizeToolArguments(call.Input)
	}
	raw, _ := json.Marshal(refs)
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleAssistant,
		Metadata:  map[string]string{metaToolCalls: string(raw)},
		Timestamp: time.Now(),
	}
}

func buildToolResultMessage(sessionID, toolCallID, content string) types.Message {
	msg := types.NewMessage(
		fmt.Sprintf("tool_%d", time.Now().UnixNano()),
		sessionID,
		types.MessageRoleTool,
		content,
	)
	msg.Metadata = map[string]string{metaToolCallID: toolCallID}
	return *msg
}

func normalizeToolArguments(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	if json.Valid([]byte(raw)) {
		return raw
	}
	// Streaming deltas may yield truncated JSON; never forward invalid args to the provider.
	return "{}"
}

func dedupeToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) <= 1 {
		return calls
	}
	seen := make(map[string]struct{}, len(calls))
	out := make([]ToolCall, 0, len(calls))
	for i, call := range calls {
		id := ensureToolCallID(call, i)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		call.ID = id
		out = append(out, call)
	}
	return out
}

func toolCallIDs(calls []ToolCall) []string {
	ids := make([]string, len(calls))
	for i, call := range calls {
		ids[i] = ensureToolCallID(call, i)
	}
	return ids
}
