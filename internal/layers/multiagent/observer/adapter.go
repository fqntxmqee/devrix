package observer

import (
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent"
)

// Adapter bridges agent events to contextengine.IObserver error channel.
type Adapter struct {
	inner contextengine.IObserver
}

// NewAdapter creates an observer adapter.
func NewAdapter(inner contextengine.IObserver) *Adapter {
	if inner == nil {
		inner = contextengine.NoOpObserver{}
	}
	return &Adapter{inner: inner}
}

func (a *Adapter) EmitAgentEvent(ev multiagent.AgentEvent) {
	if ev.EventType == "agent.error" && ev.Metadata != nil {
		if err, ok := ev.Metadata["error"].(error); ok {
			code := "AGT_ERROR"
			if c, ok := ev.Metadata["code"].(string); ok && c != "" {
				code = c
			}
			a.inner.EmitErrorOccurred(ev.SessionID, code, err)
		}
	}
}
