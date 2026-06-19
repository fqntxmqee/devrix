// Package sessionqueue — D7-S4 session command queue (Hub-Spoke drain).
//
// DSAFT: D7-S4 F — enqueues delegate-progress from flow.Hub; D7 turn runtime drains
// via contracts.SessionCommandQueue injection (no D2→D7 import).
package sessionqueue

import (
	"sync"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// SessionQueue holds per-session command queues (was process-global singleton
// pattern; DM-20260617-008 W3 removes the global var — callers now create
// a local instance via NewSessionQueue).
type SessionQueue struct {
	mu   sync.Mutex
	cmds map[string][]contracts.QueuedCommand
}

// NewSessionQueue creates an empty session queue.
func NewSessionQueue() *SessionQueue {
	return &SessionQueue{cmds: make(map[string][]contracts.QueuedCommand)}
}

// Enqueue adds a command for a session.
func (q *SessionQueue) Enqueue(sessionID string, cmd contracts.QueuedCommand) {
	if q == nil || sessionID == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cmds[sessionID] = append(q.cmds[sessionID], cmd)
}

// Drain removes and returns commands addressed to the current loop actor.
func (q *SessionQueue) Drain(sessionID, agentID string, mainThread bool) []contracts.QueuedCommand {
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
