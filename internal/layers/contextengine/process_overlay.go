package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

type processOverlayKey struct{}

// ProcessOverlay carries per-Process worker identity for D4 agents.
type ProcessOverlay struct {
	AgentID      string
	IsWorker     bool
	WorkerRole   string
	SystemPrompt string
}

// WithProcessOverlay attaches worker metadata to a Process call.
func WithProcessOverlay(ctx context.Context, ov ProcessOverlay) context.Context {
	return context.WithValue(ctx, processOverlayKey{}, ov)
}

// ProcessOverlayFromContext returns worker metadata when present.
func ProcessOverlayFromContext(ctx context.Context) (ProcessOverlay, bool) {
	ov, ok := ctx.Value(processOverlayKey{}).(ProcessOverlay)
	return ov, ok
}

func forkWorkerSessionContext(parent *types.SessionContext, ov ProcessOverlay) *types.SessionContext {
	if parent == nil {
		return &types.SessionContext{
			AgentID:    ov.AgentID,
			IsWorker:   ov.IsWorker,
			WorkerRole: ov.WorkerRole,
		}
	}
	chainID := parent.QueryChainID
	if chainID == "" {
		chainID = parent.SessionID
	}
	sc := &types.SessionContext{
		SessionID:      parent.SessionID,
		WorkDir:        parent.WorkDir,
		Model:          parent.Model,
		PermissionMode: parent.PermissionMode,
		PlanFilePath:   parent.PlanFilePath,
		TokenBudget:    parent.TokenBudget,
		AgentID:        ov.AgentID,
		IsWorker:       ov.IsWorker,
		WorkerRole:     ov.WorkerRole,
		QueryChainID:   chainID,
		QueryDepth:     parent.QueryDepth + 1,
		Messages:       []types.Message{},
		SystemPrompt:   parent.SystemPrompt,
	}
	if ov.SystemPrompt != "" {
		sc.SystemPrompt = ov.SystemPrompt
	}
	return sc
}
