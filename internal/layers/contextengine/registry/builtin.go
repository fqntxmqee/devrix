package registry

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// BuiltinRegistry delegates to the contextengine built-in tool registry.
type BuiltinRegistry struct {
	inner contextengine.IToolRegistry
}

// NewBuiltinRegistry creates the built-in tool registry with default config.
func NewBuiltinRegistry() (*BuiltinRegistry, error) {
	return NewBuiltinRegistryFromConfig(nil)
}

// NewBuiltinRegistryFromConfig creates the built-in registry from tool config.
func NewBuiltinRegistryFromConfig(toolCfg *config.ToolConfig) (*BuiltinRegistry, error) {
	inner, err := contextengine.NewBuiltinToolRegistry(toolCfg)
	if err != nil {
		return nil, err
	}
	return &BuiltinRegistry{inner: inner}, nil
}

// ListTools returns registered tool schemas.
func (r *BuiltinRegistry) ListTools(ctx context.Context, workDir string) ([]contextengine.ToolSchema, error) {
	return r.inner.ListTools(ctx, workDir)
}

// RiskLevel returns risk for a tool name.
func (r *BuiltinRegistry) RiskLevel(toolName string) types.RiskLevel {
	return r.inner.RiskLevel(toolName)
}
