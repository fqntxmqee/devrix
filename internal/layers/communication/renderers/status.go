package renderers

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// StatusRenderer renders session status
type StatusRenderer struct {
	ansi config.ANSIConfig
}

// NewStatusRenderer creates a new StatusRenderer
func NewStatusRenderer(ansi config.ANSIConfig) *StatusRenderer {
	return &StatusRenderer{ansi: ansi}
}

// RenderStatusBadge renders a status badge
func (r *StatusRenderer) RenderStatusBadge(sessionID string, state types.SessionState) {
	stateText, stateColor := r.getStateInfo(state)
	fmt.Printf("\r%s[%s]%s Session: %s ", stateColor, stateText, r.ansi.Reset, sessionID[:min(8, len(sessionID))])
}

// ClearStatus clears the status line
func (r *StatusRenderer) ClearStatus() {
	fmt.Print("\r" + clearLine())
}

// getStateInfo returns the display text and color for a state
func (r *StatusRenderer) getStateInfo(state types.SessionState) (string, string) {
	switch state {
	case types.SessionStateIdle:
		return "IDLE", ""
	case types.SessionStateThinking:
		return "THINKING", r.ansi.Warning
	case types.SessionStateStreaming:
		return "STREAMING", r.ansi.Assistant
	case types.SessionStateToolExecuting:
		return "TOOL", r.ansi.Warning
	case types.SessionStateWaitingPermission:
		return "PERMISSION", r.ansi.Warning
	case types.SessionStateCompleted:
		return "DONE", r.ansi.Assistant
	case types.SessionStateFailed:
		return "FAILED", r.ansi.Error
	default:
		return "UNKNOWN", r.ansi.Error
	}
}

// clearLine clears the current terminal line
func clearLine() string {
	return "\r\033[K"
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
