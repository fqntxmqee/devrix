package sessionorchestrator

import (
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// M4 (mups-verify-table-driven, DM-20260705-005) — verifyRollupArtifact 走决策表。
// 47 行 → 8 行：nil/Error/ExitCode guard + applyDecisionTable。
// rollup-default-fail catch-all trigger 兜底 Fail(0.85) "rollup deliverable contract not satisfied"。
func verifyRollupArtifact(art *wavescheduler.Artifact, stats workmodel.ChildOutcomeStats) workmodel.Verdict {
	if art == nil || art.Error != "" || art.ExitCode != 0 {
		return verifyArtifact(art)
	}
	ctx := &verifyContext{art: art, stats: stats, id: art.TaskID}
	return applyDecisionTable(rollupDecisionTable, art, ctx)
}
