package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// IPermissionGate approves tool execution before running.
//
// DSAFT: D1-S1-A03-F01 (PermissionGate) — defined in shared/contracts so D1
// implements and D2 consumes without D2→D1 import.
type IPermissionGate interface {
	Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool
}

// FileAutoApprover is implemented by gates that skip plan-mode write restrictions in WorkDir.
type FileAutoApprover interface {
	AutoApproveFiles() bool
}
