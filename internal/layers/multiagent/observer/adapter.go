package observer

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
)

// ErrorSink receives agent error events (subset of contextengine.IObserver).
type ErrorSink interface {
	EmitErrorOccurred(sessionID string, code string, err error)
}

// Adapter bridges agent events to an error sink without importing contextengine.
type Adapter struct {
	inner ErrorSink
}

// NewAdapter creates an observer adapter.
func NewAdapter(inner ErrorSink) *Adapter {
	if inner == nil {
		return &Adapter{}
	}
	return &Adapter{inner: inner}
}

func (a *Adapter) EmitAgentEvent(ev multiagent.AgentEvent) {
	if a.inner == nil || ev.EventType != "agent.error" || ev.Metadata == nil {
		return
	}
	err, ok := ev.Metadata["error"].(error)
	if !ok {
		return
	}
	code := "AGT_ERROR"
	if c, ok := ev.Metadata["code"].(string); ok && c != "" {
		code = c
	}
	a.inner.EmitErrorOccurred(ev.SessionID, code, err)
}
