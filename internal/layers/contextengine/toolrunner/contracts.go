// Package toolrunner — D2-S5 tool execution contracts and implementations.
//
// DSAFT: D2-S5 (Registry) tool execution surface consumed by D2-S1/D2-S10.
package toolrunner

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// ToolSchema describes a tool for the LLM or registry listing.
type ToolSchema struct {
	Name        string
	Description string
	Parameters  string
}

// ToolCall is a tool invocation with optional risk metadata.
type ToolCall struct {
	ID        string
	Name      string
	Input     string
	RiskLevel types.RiskLevel
}

// ToolResult is the outcome of tool execution.
type ToolResult struct {
	Output string
	Error  string
}

// IToolRunner executes tool calls.
//
// DSAFT: D2-S5-A02-F01 (ExecuteTool)
type IToolRunner interface {
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
}

// IToolRegistry lists available tools and risk levels.
//
// DSAFT: D2-S5-A01-F01 (ListTools)
type IToolRegistry interface {
	ListTools(ctx context.Context, workDir string) ([]ToolSchema, error)
	RiskLevel(toolName string) types.RiskLevel
}

// PluginRunner is a single pluggable tool implementation.
type PluginRunner interface {
	Name() string
	Schema() ToolSchema
	RiskLevel() types.RiskLevel
	Execute(ctx context.Context, workDir, input string) (*ToolResult, error)
}
