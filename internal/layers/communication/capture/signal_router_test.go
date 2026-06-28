package capture

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D1-RF-T07 — S15-A01 milestone_progress presenter
func TestSignalRouter_Dispatch_MilestoneProgress(t *testing.T) {
	emit := &mockRouterEmitter{}
	session := &types.Session{SessionID: "sess_1", ChatID: "chat_1"}
	event := &contracts.EngineEvent{
		Type:      "milestone_progress",
		Content:   "50% done",
		SessionID: "sess_1",
		Metadata: map[string]string{
			"progress": "50%",
			"task":     "排查图标",
		},
	}

	var router SignalRouter
	sig := contracts.IMOutboundSignal{Kind: contracts.SignalTask, Sequence: 1, SourceEventID: "ev-1"}
	router.Dispatch(SignalInput{Session: session, Event: event, Signal: sig, HasSignal: true}, emit)

	if emit.msg == nil {
		t.Fatal("expected outbound message for milestone_progress")
	}
	if emit.msg.Metadata["event_type"] != "milestone_progress" {
		t.Fatalf("event_type=%q want milestone_progress", emit.msg.Metadata["event_type"])
	}
	if emit.msg.Metadata["signal_kind"] != string(contracts.SignalTask) {
		t.Fatalf("signal_kind=%q want task", emit.msg.Metadata["signal_kind"])
	}
	if emit.msg.Metadata["progress"] != "50%" {
		t.Fatalf("progress=%q want 50%%", emit.msg.Metadata["progress"])
	}
	if emit.msg.Metadata["task"] != "排查图标" {
		t.Fatalf("task=%q want 排查图标", emit.msg.Metadata["task"])
	}
}

type mockRouterEmitter struct {
	msg *types.OutboundMessage
}

func (m *mockRouterEmitter) OnMessage(msg *types.OutboundMessage) { m.msg = msg }
func (m *mockRouterEmitter) OnStatus(string, types.SessionState)   {}
