package run

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

var validTransitions = map[multiagent.AgentState][]multiagent.AgentState{
	multiagent.AgentStateCreated: {
		multiagent.AgentStateRunning,
		multiagent.AgentStateTerminated,
	},
	multiagent.AgentStateRunning: {
		multiagent.AgentStateIterating,
		multiagent.AgentStateTerminated,
	},
	multiagent.AgentStateIterating: {
		multiagent.AgentStateIterating,
		multiagent.AgentStateWaitingPermission,
		multiagent.AgentStateTerminated,
	},
	multiagent.AgentStateWaitingPermission: {
		multiagent.AgentStateIterating,
		multiagent.AgentStateTerminated,
	},
}

func transition(from, to multiagent.AgentState) error {
	if from == multiagent.AgentStateTerminated {
		return sharederrors.NewAgentAlreadyTerminatedError("")
	}
	allowed := validTransitions[from]
	for _, next := range allowed {
		if next == to {
			return nil
		}
	}
	return sharederrors.NewAgentInvalidTransitionError(from.String(), to.String())
}
