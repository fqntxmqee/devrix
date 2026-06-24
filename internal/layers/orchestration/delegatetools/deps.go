package delegatetools

import (
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// Deps wires delegate tool handlers.
//
// DM-20260617-008 W4: Tasks is the explicit task store (was: read from
// workmodel.GlobalTaskManager process-wide singleton).
type Deps struct {
	Dispatcher *sessionorchestrator.Dispatcher
	Leader     LeaderResolver
	Tasks      *workmodel.TaskManager
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
