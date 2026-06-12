package contextengine

import (
	"github.com/devrix/devrix/internal/layers/contextengine/query"
)

// QueryLoop returns a QueryLoop instance sharing ContextEngine dependencies.
func (e *ContextEngine) QueryLoop() *query.Loop {
	if e == nil {
		return nil
	}
	return e.queryLoop
}

// ToolRegistry returns the engine tool registry.
func (e *ContextEngine) ToolRegistry() IToolRegistry {
	if e == nil {
		return nil
	}
	return e.toolsReg
}
