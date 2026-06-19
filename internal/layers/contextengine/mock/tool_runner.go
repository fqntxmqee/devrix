// Package mockctx provides test doubles for D2 ContextEngine ports.
//
// mock/ structure (2026-06-19 v2.2 closure + dead-code cleanup):
//
//   tool_runner.go    — S18 ports D2 owns: IToolRunner, IPermissionGate test doubles
//   summarizer.go     — Cross-domain fixture: contracts.Summarizer (D7-S2-A07 owner; D2 consumer)
//   prepared_turn.go  — Cross-domain fixture: contracts.PreparedTurnRunner (D7-S2-A06 owner; D2 consumer)
//
// D2 cannot import D7 (D2 is Follower; D7 is Leader), so cross-domain fixtures
// must live in D2/mock/ for D2 tests to use them.
//
// Stay-in-domain rationale: cmd/obs-verify/main.go imports mockctx as a
// non-test smoke-test fixture. Moving to tests/testutil/ would force cmd/
// to import tests/, which violates Go's test-only-imports convention.
// See devrix-d2-structure-closure (DM-20260619-007) P2-T3 decision.
package mockctx

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ToolRunner is a no-op IToolRunner test double for S18.
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

// AllowAllPermission is an IPermissionGate test double that always Approves.
//
// S18-A01 (EnforceExecutionPolicy permission check).
type AllowAllPermission struct{}

// Request implements legacy Request() — for code paths that predate TOOL-SURFACE-1.
func (AllowAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return true
}

// CheckPermission implements the post-TOOL-SURFACE-1 permission contract (DM-20260618-002).
func (AllowAllPermission) CheckPermission(_ context.Context, _ contracts.ToolSpec) contracts.Decision {
	return contracts.DecisionAllow
}

// DenyAllPermission is an IPermissionGate test double that always Denies.
//
// S18-A01 (EnforceExecutionPolicy permission check).
type DenyAllPermission struct{}

// Request implements legacy Request() — for code paths that predate TOOL-SURFACE-1.
func (DenyAllPermission) Request(context.Context, string, string, string, types.RiskLevel) bool {
	return false
}

// CheckPermission implements the post-TOOL-SURFACE-1 permission contract (DM-20260618-002).
func (DenyAllPermission) CheckPermission(_ context.Context, _ contracts.ToolSpec) contracts.Decision {
	return contracts.DecisionDeny
}
