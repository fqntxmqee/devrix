package query_test

import (
	"sync"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// testSessionQueue is a minimal contracts.SessionCommandQueue for query tests.
type testSessionQueue struct {
	mu   sync.Mutex
	cmds map[string][]contracts.QueuedCommand
}

func newTestSessionQueue() *testSessionQueue {
	return &testSessionQueue{cmds: make(map[string][]contracts.QueuedCommand)}
}

func (q *testSessionQueue) Enqueue(sessionID string, cmd contracts.QueuedCommand) {
	if q == nil || sessionID == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cmds[sessionID] = append(q.cmds[sessionID], cmd)
}

func (q *testSessionQueue) Drain(sessionID, agentID string, mainThread bool) []contracts.QueuedCommand {
	if q == nil || sessionID == "" {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	pending := q.cmds[sessionID]
	if len(pending) == 0 {
		return nil
	}
	var kept, out []contracts.QueuedCommand
	for _, cmd := range pending {
		if mainThread {
			if cmd.AgentID != "" {
				kept = append(kept, cmd)
				continue
			}
			out = append(out, cmd)
			continue
		}
		if cmd.Mode == contracts.ModeTaskNotification && cmd.AgentID == agentID {
			out = append(out, cmd)
			continue
		}
		kept = append(kept, cmd)
	}
	q.cmds[sessionID] = kept
	return out
}
