package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/policy/toolrunner"
	"github.com/devrix/devrix/internal/shared/types"
)

// ToolRunner is a no-op tool runner for tests.
type ToolRunner struct {
	Output string
	Err    error
}

// Execute returns configured output.
func (t *ToolRunner) Execute(ctx context.Context, call toolrunner.ToolCall) (*toolrunner.ToolResult, error) {
	if t.Err != nil {
		return nil, t.Err
	}
	out := t.Output
	if out == "" {
		out = "ok"
	}
	return &toolrunner.ToolResult{Output: out}, nil
}

// AllowAllPermission always approves.
type AllowAllPermission struct{}

func (AllowAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return true
}

// DenyAllPermission always denies.
type DenyAllPermission struct{}

func (DenyAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return false
}
