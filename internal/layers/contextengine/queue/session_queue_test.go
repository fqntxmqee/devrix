package queue_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/queue"
)

func TestSessionQueue_drain_main_thread_only_own_commands(t *testing.T) {
	q := queue.NewSessionQueue()
	q.Enqueue("sess", queue.QueuedCommand{Value: "user prompt", Mode: queue.ModePrompt})
	q.Enqueue("sess", queue.QueuedCommand{Value: "subagent note", Mode: queue.ModeTaskNotification, AgentID: "agent_1"})

	out := q.Drain("sess", "", true)
	if len(out) != 1 || out[0].Value != "user prompt" {
		t.Fatalf("main thread should drain only unscoped commands, got %+v", out)
	}

	sub := q.Drain("sess", "agent_1", false)
	if len(sub) != 1 || sub[0].Value != "subagent note" {
		t.Fatalf("subagent should drain its first notification, got %+v", sub)
	}

	q.Enqueue("sess", queue.QueuedCommand{Value: "done", Mode: queue.ModeTaskNotification, AgentID: "agent_1"})
	sub2 := q.Drain("sess", "agent_1", false)
	if len(sub2) != 1 || sub2[0].Value != "done" {
		t.Fatalf("subagent should drain follow-up notification, got %+v", sub2)
	}
}

func TestRenderNotifications_should_produce_meta_user_messages(t *testing.T) {
	msgs := queue.RenderNotifications("sess", []queue.QueuedCommand{
		{Value: "task finished", Mode: queue.ModeTaskNotification},
	})
	if len(msgs) != 1 {
		t.Fatal("expected one rendered message")
	}
	if msgs[0].Metadata["queue_mode"] != string(queue.ModeTaskNotification) {
		t.Fatalf("unexpected metadata: %+v", msgs[0].Metadata)
	}
}
