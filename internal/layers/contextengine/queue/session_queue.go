package queue

import (
	"fmt"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/shared/types"
)

// CommandMode classifies queued loop inputs (CC queue aligned).
type CommandMode string

const (
	ModePrompt           CommandMode = "prompt"
	ModeTaskNotification CommandMode = "task-notification"
	ModeDelegateProgress CommandMode = "delegate-progress"
)

// QueuedCommand is a pending prompt or task notification for QueryLoop drain.
type QueuedCommand struct {
	Value   string
	Mode    CommandMode
	AgentID string
}

// SessionQueue holds per-session command queues (process-global singleton pattern).
type SessionQueue struct {
	mu   sync.Mutex
	cmds map[string][]QueuedCommand
}

// GlobalSessionQueue is the default queue instance.
var GlobalSessionQueue = NewSessionQueue()

// NewSessionQueue creates an empty session queue.
func NewSessionQueue() *SessionQueue {
	return &SessionQueue{cmds: make(map[string][]QueuedCommand)}
}

// Enqueue adds a command for a session.
func (q *SessionQueue) Enqueue(sessionID string, cmd QueuedCommand) {
	if q == nil || sessionID == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cmds[sessionID] = append(q.cmds[sessionID], cmd)
}

// Drain removes and returns commands addressed to the current loop actor.
func (q *SessionQueue) Drain(sessionID, agentID string, mainThread bool) []QueuedCommand {
	if q == nil || sessionID == "" {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	pending := q.cmds[sessionID]
	if len(pending) == 0 {
		return nil
	}
	var kept, out []QueuedCommand
	for _, cmd := range pending {
		if mainThread {
			if cmd.AgentID != "" {
				kept = append(kept, cmd)
				continue
			}
			out = append(out, cmd)
			continue
		}
		if cmd.Mode == ModeTaskNotification && cmd.AgentID == agentID {
			out = append(out, cmd)
			continue
		}
		kept = append(kept, cmd)
	}
	q.cmds[sessionID] = kept
	return out
}

// RenderNotifications converts drained commands to meta user messages.
func RenderNotifications(sessionID string, cmds []QueuedCommand) []types.Message {
	if len(cmds) == 0 {
		return nil
	}
	out := make([]types.Message, 0, len(cmds))
	for i, cmd := range cmds {
		body := strings.TrimSpace(cmd.Value)
		if body == "" {
			continue
		}
		if cmd.Mode == ModeDelegateProgress {
			body = formatDelegateProgressReminder(body)
		}
		out = append(out, types.Message{
			ID:        fmt.Sprintf("queue_%d_%d", i, len(out)),
			SessionID: sessionID,
			Role:      types.MessageRoleUser,
			Content:   fmt.Sprintf("<system-reminder>\n[%s]\n%s\n</system-reminder>", cmd.Mode, body),
			Metadata:  map[string]string{"is_meta": "true", "queue_mode": string(cmd.Mode)},
		})
	}
	return out
}
