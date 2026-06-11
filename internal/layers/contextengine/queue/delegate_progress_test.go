package queue_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/queue"
)

func TestRenderNotifications_should_format_delegate_progress(t *testing.T) {
	raw := `{"session_id":"sess1","worker_id":"explore_1","kind":"tool_call","summary":"grep auth","source":"subquery","role":"explore"}`
	msgs := queue.RenderNotifications("sess1", []queue.QueuedCommand{{
		Value: raw,
		Mode:  queue.ModeDelegateProgress,
	}})
	if len(msgs) != 1 {
		t.Fatalf("msgs = %d", len(msgs))
	}
	if msgs[0].Content == "" || msgs[0].Metadata["queue_mode"] != string(queue.ModeDelegateProgress) {
		t.Fatalf("msg = %+v", msgs[0])
	}
}

// Covers: L5-4-10-04
func TestDrain_should_only_give_delegate_progress_to_leader_main_thread(t *testing.T) {
	q := queue.NewSessionQueue()
	q.Enqueue("sess1", queue.QueuedCommand{
		Value: `{"kind":"started","summary":"explore"}`,
		Mode:  queue.ModeDelegateProgress,
	})
	q.Enqueue("sess1", queue.QueuedCommand{
		Value: "worker note",
		Mode:  queue.ModeTaskNotification,
		AgentID: "worker_1",
	})

	workerDrain := q.Drain("sess1", "worker_1", false)
	if len(workerDrain) != 1 || workerDrain[0].Mode != queue.ModeTaskNotification {
		t.Fatalf("worker drain = %+v", workerDrain)
	}

	leaderDrain := q.Drain("sess1", "", true)
	if len(leaderDrain) != 1 || leaderDrain[0].Mode != queue.ModeDelegateProgress {
		t.Fatalf("leader drain = %+v", leaderDrain)
	}
}
