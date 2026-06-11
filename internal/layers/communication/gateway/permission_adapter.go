package gateway

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// PermissionGateAdapter bridges PermissionManager to contextengine.IPermissionGate.
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
