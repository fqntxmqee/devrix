package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/contracts"
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

// CheckPermission always Allow — for tests that want the gate to
// never block. TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (AllowAllPermission) CheckPermission(_ context.Context, _ contracts.ToolSpec) contracts.Decision {
	return contracts.DecisionAllow
}

// DenyAllPermission always denies.
type DenyAllPermission struct{}

func (DenyAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return false
}

// CheckPermission always Deny — for tests that want the gate to
// always block. TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
func (DenyAllPermission) CheckPermission(_ context.Context, _ contracts.ToolSpec) contracts.Decision {
	return contracts.DecisionDeny
}
