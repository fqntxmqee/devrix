package permission

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// InitSessionPermission applies default permission mode and plan file path.
//
// DSAFT: D2-S3-A01 (CheckPermission)
func InitSessionPermission(sc *types.SessionContext, cfg config.ContextPermissionConfig) {
	if sc == nil {
		return
	}
	if sc.PermissionMode == "" {
		sc.PermissionMode = types.PermissionMode(cfg.DefaultMode)
		if sc.PermissionMode == "" {
			sc.PermissionMode = types.PermissionDefault
		}
	}
	if sc.PlanFilePath == "" && cfg.Plan.PlanFileDir != "" {
		sc.PlanFilePath = fmt.Sprintf("%s/%s.md", cfg.Plan.PlanFileDir, sc.SessionID)
	}
}
