package taskprogress

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

type captureEmitter struct {
	msg *types.OutboundMessage
}

func (c *captureEmitter) OnMessage(msg *types.OutboundMessage) { c.msg = msg }
func (c *captureEmitter) OnStatus(string, types.SessionState)   {}

func TestEmitToolCall_should_use_ToolInput_when_metadata_input_missing(t *testing.T) {
	emit := &captureEmitter{}
	session := &types.Session{SessionID: "sess_1", ChatID: "chat_1"}
	event := &contracts.EngineEvent{
		Type:      "tool_call",
		ToolName:  "Grep",
		ToolInput: `{"pattern":"auth","path":"."}`,
		SessionID: "sess_1",
	}

	EmitToolCall(session, event, contracts.IMOutboundSignal{}, false, emit)

	if emit.msg == nil {
		t.Fatal("expected outbound message")
	}
	if emit.msg.Metadata["input"] != event.ToolInput {
		t.Fatalf("metadata input = %q, want %q", emit.msg.Metadata["input"], event.ToolInput)
	}
	if emit.msg.Metadata["tool_name"] != "Grep" {
		t.Fatalf("metadata tool_name = %q, want Grep", emit.msg.Metadata["tool_name"])
	}
}
