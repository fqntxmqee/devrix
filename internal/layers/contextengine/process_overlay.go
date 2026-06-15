package contextengine

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

func forkWorkerSessionContext(parent *types.SessionContext, ov contracts.ProcessOverlay) *types.SessionContext {
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
		ModelTier:      parent.ModelTier,
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
	if ov.ModelTier != "" {
		sc.ModelTier = ov.ModelTier
	}
	return sc
}
