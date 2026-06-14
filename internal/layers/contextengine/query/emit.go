package query

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func emitThinking(emit EmitFunc, sessionID, content string) {
	emit(&contracts.EngineEvent{Type: "thinking", Content: content, SessionID: sessionID})
}

func emitText(emit EmitFunc, sessionID, content string, complete bool) {
	meta := map[string]string{"is_complete": "false"}
	if complete {
		meta["is_complete"] = "true"
	}
	emit(&contracts.EngineEvent{Type: "text", Content: content, SessionID: sessionID, Metadata: meta})
}

func emitToolCall(emit EmitFunc, sc *types.SessionContext, ref conversation.ToolCallRef) {
	emit(&contracts.EngineEvent{
		Type: "tool_call", ToolName: ref.Name, ToolInput: ref.Input, SessionID: sc.SessionID,
		Metadata: map[string]string{"tool_name": ref.Name, "input": ref.Input},
	})
}

func emitToolResult(emit EmitFunc, sessionID, name, content, errMsg string) {
	emit(&contracts.EngineEvent{
		Type: "tool_result", Content: content, ToolName: name, SessionID: sessionID,
		Metadata: map[string]string{"tool_name": name, "error": errMsg},
	})
}
