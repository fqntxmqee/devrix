package capture

import (
	"context"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// PermissionGateAdapter bridges PermissionManager to contracts.IPermissionGate.
type PermissionGateAdapter struct {
	mgr *PermissionManager
}

// NewPermissionGateAdapter creates an IPermissionGate adapter.
func NewPermissionGateAdapter(mgr *PermissionManager) *PermissionGateAdapter {
	return &PermissionGateAdapter{mgr: mgr}
}

// Request delegates to PermissionManager.
func (a *PermissionGateAdapter) Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool {
	if a.mgr == nil {
		return false
	}
	return a.mgr.Request(ctx, sessionID, toolName, input, risk)
}

// CheckPermission implements contracts.IPermissionGate (DM-20260618-002).
// Default Risk → Decision mapping; plan-mode OpenWorld denial is
// composed by the bootstrap (PlanModeOpenWorldPolicy), not here.
func (a *PermissionGateAdapter) CheckPermission(_ context.Context, spec contracts.ToolSpec) contracts.Decision {
	switch spec.Risk {
	case types.RiskLevelLow:
		return contracts.DecisionAllow
	default:
		return contracts.DecisionAsk
	}
}

// IsYOLOMode delegates to PermissionManager.
func (a *PermissionGateAdapter) IsYOLOMode() bool {
	if a.mgr == nil {
		return false
	}
	return a.mgr.IsYOLOMode()
}

// AutoApproveFiles delegates to PermissionManager YOLO file policy.
func (a *PermissionGateAdapter) AutoApproveFiles() bool {
	if a.mgr == nil {
		return false
	}
	return a.mgr.AutoApproveFiles()
}
