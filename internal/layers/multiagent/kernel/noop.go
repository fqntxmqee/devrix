package kernel

// NoOpAgentObserver discards agent events.
type NoOpAgentObserver struct{}

func (NoOpAgentObserver) EmitAgentEvent(AgentEvent) {}
