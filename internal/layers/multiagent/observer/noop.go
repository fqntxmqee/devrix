package observer

import "github.com/devrix/devrix/internal/layers/multiagent"

// NoOpAgentObserver discards agent events.
type NoOpAgentObserver struct{}

func (NoOpAgentObserver) EmitAgentEvent(multiagent.AgentEvent) {}
