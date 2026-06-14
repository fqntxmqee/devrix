package sessionqueue_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestSessionQueue_drain_main_thread_only_own_commands(t *testing.T) {
	q := sessionqueue.NewSessionQueue()
	q.Enqueue("sess", contracts.QueuedCommand{Value: "user prompt", Mode: contracts.ModePrompt})
	q.Enqueue("sess", contracts.QueuedCommand{Value: "subagent note", Mode: contracts.ModeTaskNotification, AgentID: "agent_1"})

	out := q.Drain("sess", "", true)
	if len(out) != 1 || out[0].Value != "user prompt" {
		t.Fatalf("main thread should drain only unscoped commands, got %+v", out)
	}

	sub := q.Drain("sess", "agent_1", false)
	if len(sub) != 1 || sub[0].Value != "subagent note" {
		t.Fatalf("subagent should drain its first notification, got %+v", sub)
	}

	q.Enqueue("sess", contracts.QueuedCommand{Value: "done", Mode: contracts.ModeTaskNotification, AgentID: "agent_1"})
	sub2 := q.Drain("sess", "agent_1", false)
	if len(sub2) != 1 || sub2[0].Value != "done" {
		t.Fatalf("subagent should drain follow-up notification, got %+v", sub2)
	}
}

func TestRenderQueueNotifications_should_produce_meta_user_messages(t *testing.T) {
	msgs := contracts.RenderQueueNotifications("sess", []contracts.QueuedCommand{
		{Value: "task finished", Mode: contracts.ModeTaskNotification},
	})
	if len(msgs) != 1 {
		t.Fatal("expected one rendered message")
	}
	if msgs[0].Metadata["queue_mode"] != string(contracts.ModeTaskNotification) {
		t.Fatalf("unexpected metadata: %+v", msgs[0].Metadata)
	}
}
