package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// IPermissionGate approves tool execution before running.
//
// DSAFT: D1-S1-A03-F01 (PermissionGate) — defined in shared/contracts so D1
// implements and D2 consumes without D2→D1 import.
//
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002) adds CheckPermission as a
// per-tool hook that supplements the turn-level Request. The 3-state
// Decision (Allow/Deny/Ask) is finer-grained than the boolean Request
// can express — the gate can deny on spec.OpenWorld=true in plan mode
// even if the turn-level Request returned true.
type IPermissionGate interface {
	// Request is the turn-level decision (DM-006). Returns true if the
	// tool is allowed for this turn; false triggers a hard deny. Many
	// implementations consult user YOLO preferences here.
	Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool

	// CheckPermission is the per-tool decision. Called by
	// turn_adapter.ExecuteRound when surface.CheckPermission returns
	// DecisionAsk (or before, for plan-mode OpenWorld denial).
	// Default Risk → Decision mapping:
	//   Risk=low       → Allow
	//   Risk=medium    → Ask
	//   Risk=high      → Ask
	// Plan-mode OpenWorld=true (no allowlist match) → Deny.
	CheckPermission(ctx context.Context, spec ToolSpec) Decision
}

// FileAutoApprover is implemented by gates that skip plan-mode write restrictions in WorkDir.
type FileAutoApprover interface {
	AutoApproveFiles() bool
}
