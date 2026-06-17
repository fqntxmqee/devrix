package contextengine

import (
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/contracts"
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

// Surfaces returns the list of ToolSurface instances the engine was
// constructed with. Returns nil if the engine was built without the
// TOOL-SURFACE-1 (W8) inputs — in that case the legacy
// Tools/ToolsReg path is still in use.
//
// DSAFT: TOOL-SURFACE-1-A03-F04
func (e *ContextEngine) Surfaces() []contracts.ToolSurface {
	if e == nil {
		return nil
	}
	return e.surfaces
}

// Filters returns the filter chain the engine was constructed with.
// Returns nil if the engine was built without the TOOL-SURFACE-1
// (W8) inputs.
func (e *ContextEngine) Filters() []contracts.ToolFilter {
	if e == nil {
		return nil
	}
	return e.filters
}

// HasSurfaces reports whether the engine has a non-nil surface list.
// Used by callers that need to choose between the legacy IToolRunner
// path and the new surface dispatch path.
func (e *ContextEngine) HasSurfaces() bool {
	return e != nil && len(e.surfaces) > 0
}
