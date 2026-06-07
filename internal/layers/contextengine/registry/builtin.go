package registry

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/shared/types"
)

// BuiltinRegistry provides a minimal V1 tool set.
type BuiltinRegistry struct{}

// NewBuiltinRegistry creates the built-in tool registry.
func NewBuiltinRegistry() *BuiltinRegistry {
	return &BuiltinRegistry{}
}

// ListTools returns built-in tool schemas.
func (r *BuiltinRegistry) ListTools(ctx context.Context, workDir string) ([]contextengine.ToolSchema, error) {
	_ = ctx
	_ = workDir
	return []contextengine.ToolSchema{
		{Name: "read_file", Description: "Read a file from the workspace"},
		{Name: "write_file", Description: "Write content to a file"},
		{Name: "bash", Description: "Execute a shell command"},
	}, nil
}

// RiskLevel returns risk for a tool name.
func (r *BuiltinRegistry) RiskLevel(toolName string) types.RiskLevel {
	switch toolName {
	case "bash":
		return types.RiskLevelHigh
	case "write_file":
		return types.RiskLevelMedium
	default:
		return types.RiskLevelLow
	}
}
