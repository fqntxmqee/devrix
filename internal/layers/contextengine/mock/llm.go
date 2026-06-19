// Package mockctx provides test doubles for D2 ContextEngine ports.
//
// P2-T3 status: kept in domain for now. Originally targeted for move to
// tests/testutil/contextengine/, but cmd/obs-verify/main.go imports mockctx
// directly as a smoke-test fixture (not a _test.go file). Moving the package
// out of domain would force cmd to import tests/testutil/, which violates
// Go's test-only-imports convention. Leaving mock/ here is the pragmatic
// trade-off; the P2-T3 doc is updated accordingly.
package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ToolRunner is a no-op tool runner for tests.
type ToolRunner struct {
	Output string
	Err    error
}

// Execute returns configured output.
func (t *ToolRunner) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	if t.Err != nil {
		return nil, t.Err
	}
	out := t.Output
	if out == "" {
		out = "ok"
	}
	return &tools.ToolResult{Output: out}, nil
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
