package delegatetools

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
)

// Deps wires delegate tool handlers.
type Deps struct {
	Dispatcher *hubspoke.Dispatcher
	Leader     LeaderResolver
}

// LeaderResolver returns the session leader agent when D4 is active.
type LeaderResolver interface {
	Leader(sessionID string) (multiagent.Agent, bool)
}

var globalDeps Deps

// SetDeps configures delegate_* tool handlers.
func SetDeps(deps Deps) {
	globalDeps = deps
}
