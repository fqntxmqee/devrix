package delegate

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// Covers: async delegate Leader drain notification
func TestNotifyLeaderAsyncComplete_should_enqueue_main_thread(t *testing.T) {
	q := sessionqueue.NewSessionQueue()
	s := &Service{queue: q}
	s.notifyLeaderAsyncComplete("sess_async", "worker-1", WorkerSpec{Role: WorkerRoleExplore}, "explore done", nil)

	drained := q.Drain("sess_async", "", true)
	if len(drained) != 1 || drained[0].Mode != contracts.ModeTaskNotification {
		t.Fatalf("drain = %+v", drained)
	}
	if drained[0].AgentID != "" {
		t.Fatalf("expected leader main-thread notification, AgentID=%q", drained[0].AgentID)
	}
}

func TestNotifyLeaderAsyncComplete_should_skip_when_queue_nil(t *testing.T) {
	s := &Service{}
	s.notifyLeaderAsyncComplete("sess", "w", WorkerSpec{}, "ok", nil)
}
