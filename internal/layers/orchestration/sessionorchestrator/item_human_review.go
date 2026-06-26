package sessionorchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/types"
)

// runHumanReviewAwait pauses pipeline on pending human-review WorkItems (TD-WT-05).
func (r *ItemPipelineRunner) runHumanReviewAwait(
	ctx context.Context,
	sessionID string,
	item *workmodel.WorkItem,
) (*workmodel.WorkItemPipelineRound, error) {
	if r == nil || r.Tasks == nil || item == nil {
		return nil, fmt.Errorf("item_pipeline: human review gate unavailable")
	}
	_ = ctx
	started := time.Now()
	round := &workmodel.WorkItemPipelineRound{
		RoundNo:        1,
		WorkItemID:     item.ID,
		SessionID:      sessionID,
		VerdictKind:    types.VerdictIndeterminate,
		VerdictID:      "human_review_pending",
		SpawnPolicy:    workmodel.SpawnAwait,
		SpawnRationale: fmt.Sprintf("Awaiting human review for %s — use /task review approve %s", item.Title, item.ID),
		StartedAt:      started,
		CompletedAt:    time.Now(),
	}
	_ = r.Tasks.Tree().SetRoundPhase(sessionID, item.ID, workmodel.RoundPhaseAwaitChild)
	if err := r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, workmodel.RoundPhaseAwaitChild); err != nil {
		return nil, err
	}
	item.LastRound = round
	item.RoundPhase = workmodel.RoundPhaseAwaitChild
	return round, nil
}
