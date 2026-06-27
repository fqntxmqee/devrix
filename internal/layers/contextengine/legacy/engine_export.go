package legacy

import (
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
)

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
func (e *ContextEngine) Surfaces() []contracts.ToolSurface {
	if e == nil {
		return nil
	}
	return e.surfaces
}

// Filters returns the filter chain the engine was constructed with.
func (e *ContextEngine) Filters() []contracts.ToolFilter {
	if e == nil {
		return nil
	}
	return e.filters
}

// HasSurfaces reports whether the engine has a non-nil surface list.
func (e *ContextEngine) HasSurfaces() bool {
	return e != nil && len(e.surfaces) > 0
}

// PromptLocale returns the locale used for LLM-facing prompts and tool schemas.
func (e *ContextEngine) PromptLocale() i18n.Locale {
	if e == nil || e.cfg == nil {
		return i18n.DefaultLocale
	}
	return i18n.ParseLanguage(e.cfg.Workspace.Language)
}
