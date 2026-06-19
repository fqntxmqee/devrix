package enforce

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// Test doubles for S18 ports owned by D2 (enforce/).
//
// These live in the enforce/ package alongside the interfaces they implement
// (IToolRunner in enforce/tools, IPermissionGate in shared/contracts). They
// are not production code — they are zero-config fakes for tests and the
// cmd/obs-verify smoke binary. Production callers must use the real
// ToolRegistry / PermissionGate implementations.
//
// Migrated from internal/layers/contextengine/mock/tool_runner.go during
// the 2026-06-19 D2 mock/ cleanup. The mock/ package was semantically
// misleading — it bundled D2-owned doubles with cross-domain fixtures; this
// split puts each in its semantic home.

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
