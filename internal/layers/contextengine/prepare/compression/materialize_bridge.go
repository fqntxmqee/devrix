package compression

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// DefaultMaterializeToolResultBudget is the per-tool token cap before persist
// on WorkItem private chains (~50K chars, mirrors clawcode DEFAULT_MAX).
const DefaultMaterializeToolResultBudget = 12_500

// PersistBudgetConfig configures persist + per-message budget for Materialize.
type PersistBudgetConfig struct {
	ProjectDir       string
	SessionID        string
	ToolResultBudget int
	PerMessageBudget *PerMessageBudget
	Counter          contracts.ITokenCounter
}

// ApplyPersistBudget runs toolResultBudget then optional per-message budget.
// Does not tail-drop messages — callers run compressMessages afterward.
func ApplyPersistBudget(msgs []types.Message, cfg PersistBudgetConfig) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	counter := cfg.Counter
	if counter == nil {
		counter = token.NewCounter()
	}
	maxPer := cfg.ToolResultBudget
	if maxPer <= 0 {
		maxPer = DefaultMaterializeToolResultBudget
	}
	out := toolResultBudget(counter, msgs, maxPer, cfg.ProjectDir, cfg.SessionID)
	if cfg.PerMessageBudget != nil {
		out = applyPerMessageBudget(cfg.PerMessageBudget, out)
	}
	return out
}
