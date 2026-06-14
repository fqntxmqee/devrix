package conversation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

const (
	MetaToolCalls  = "tool_calls"
	MetaToolCallID = "tool_call_id"
	MetaIsMeta     = "is_meta"
	MetaAttachment = "attachment_type"
)

// StripSystem removes leading system messages (LLM view uses separate SystemPrompt field).
func StripSystem(msgs []types.Message) []types.Message {
	out := make([]types.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == types.MessageRoleSystem {
			continue
		}
		out = append(out, m)
	}
	return out
}

// PrependMetaUser inserts a meta user message at the front of LLM-bound messages.
func PrependMetaUser(msgs []types.Message, content string) []types.Message {
	if strings.TrimSpace(content) == "" {
		return msgs
	}
	meta := types.Message{
		ID:        fmt.Sprintf("meta_%d", time.Now().UnixNano()),
		Role:      types.MessageRoleUser,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  map[string]string{MetaIsMeta: "true"},
	}
	return append([]types.Message{meta}, msgs...)
}

// HasMetaUserContext reports whether snapshot messages already contain a prepend block.
func HasMetaUserContext(msgs []types.Message) bool {
	for _, m := range msgs {
		if m.Role == types.MessageRoleUser && m.Metadata[MetaIsMeta] == "true" {
			if strings.Contains(m.Content, "<system-reminder>") {
				return true
			}
		}
	}
	return false
}

// ToolCallsFromAssistant extracts tool calls encoded in assistant metadata.
func ToolCallsFromAssistant(m types.Message) []ToolCallRef {
	if m.Role != types.MessageRoleAssistant {
		return nil
	}
	raw := m.Metadata[MetaToolCalls]
	if raw == "" {
		return nil
	}
	var refs []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if err := json.Unmarshal([]byte(raw), &refs); err != nil {
		return nil
	}
	out := make([]ToolCallRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, ToolCallRef{
			ID:    r.ID,
			Name:  r.Function.Name,
			Input: r.Function.Arguments,
		})
	}
	return out
}

// ToolCallRef is a lightweight tool_use reference for conversation inspection.
type ToolCallRef struct {
	ID    string
	Name  string
	Input string
}

// ToolCallIDFromResult reads tool_call_id from a tool role message.
func ToolCallIDFromResult(m types.Message) string {
	if m.Role != types.MessageRoleTool {
		return ""
	}
	return m.Metadata[MetaToolCallID]
}

type serializedToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// BuildAssistantToolCallsMessage encodes assistant tool_use for providers.
func BuildAssistantToolCallsMessage(sessionID, assistantText string, calls []ToolCallRef) types.Message {
	refs := make([]serializedToolCall, len(calls))
	for i, call := range calls {
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)
		}
		refs[i].ID = id
		refs[i].Type = "function"
		refs[i].Function.Name = call.Name
		refs[i].Function.Arguments = normalizeArgs(call.Input)
	}
	raw, _ := json.Marshal(refs)
	return types.Message{
		SessionID: sessionID,
		Role:      types.MessageRoleAssistant,
		Content:   assistantText,
		Metadata:  map[string]string{MetaToolCalls: string(raw)},
		Timestamp: time.Now(),
	}
}

// BuildToolResultMessage creates a tool role message linked to tool_call_id.
func BuildToolResultMessage(sessionID, toolCallID, content string) types.Message {
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		toolCallID = fmt.Sprintf("call_%d", time.Now().UnixNano())
	}
	msg := types.NewMessage(
		fmt.Sprintf("tool_%d", time.Now().UnixNano()),
		sessionID,
		types.MessageRoleTool,
		content,
	)
	msg.Metadata = map[string]string{MetaToolCallID: toolCallID}
	return *msg
}

func normalizeArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}"
	}
	if json.Valid([]byte(raw)) {
		return raw
	}
	return "{}"
}

// DedupeToolCalls removes duplicate tool calls by ID.
func DedupeToolCalls(calls []ToolCallRef) []ToolCallRef {
	if len(calls) <= 1 {
		return calls
	}
	seen := make(map[string]struct{}, len(calls))
	out := make([]ToolCallRef, 0, len(calls))
	for i, call := range calls {
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i)
			call.ID = id
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, call)
	}
	return out
}
