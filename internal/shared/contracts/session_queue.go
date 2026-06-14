package contracts

import (
	"fmt"
	"strings"

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

// SessionCommandQueue holds per-session command queues for Hub-Spoke drain.
type SessionCommandQueue interface {
	Enqueue(sessionID string, cmd QueuedCommand)
	Drain(sessionID, agentID string, mainThread bool) []QueuedCommand
}

// RenderQueueNotifications converts drained commands to meta user messages.
func RenderQueueNotifications(sessionID string, cmds []QueuedCommand) []types.Message {
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
			body = FormatDelegateProgressReminder(body)
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
