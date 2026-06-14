package sessionqueue_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestDelegateProgressReminder_should_render_flow_event(t *testing.T) {
	msgs := contracts.RenderQueueNotifications("sess1", []contracts.QueuedCommand{{
		Value: `{"worker_id":"w1","kind":"progress","summary":"still working"}`,
		Mode:  contracts.ModeDelegateProgress,
	}})
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content == "" || msgs[0].Metadata["queue_mode"] != string(contracts.ModeDelegateProgress) {
		t.Fatalf("unexpected message: %+v", msgs[0])
	}
}

func TestSessionQueue_delegate_progress_routes_to_leader_only(t *testing.T) {
	q := sessionqueue.NewSessionQueue()
	q.Enqueue("sess1", contracts.QueuedCommand{
		Value: "delegate update",
		Mode:  contracts.ModeDelegateProgress,
	})
	q.Enqueue("sess1", contracts.QueuedCommand{
		Value: "worker done",
		Mode:  contracts.ModeTaskNotification,
		AgentID: "w1",
	})

	workerDrain := q.Drain("sess1", "w1", false)
	if len(workerDrain) != 1 || workerDrain[0].Mode != contracts.ModeTaskNotification {
		t.Fatalf("worker drain = %+v", workerDrain)
	}

	leaderDrain := q.Drain("sess1", "", true)
	if len(leaderDrain) != 1 || leaderDrain[0].Mode != contracts.ModeDelegateProgress {
		t.Fatalf("leader drain = %+v", leaderDrain)
	}
}
