package sessionorchestrator

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// RunParallelExplore executes an ephemeral ScenarioPlan probe batch and writes
// the summary back to the parent LastRound (design §6 D3 — no child WorkItems).
func (r *ItemPipelineRunner) RunParallelExplore(
	ctx context.Context,
	sessionID string,
	item *workmodel.WorkItem,
	round *workmodel.WorkItemPipelineRound,
) error {
	if r == nil || item == nil || round == nil || r.Router == nil {
		return nil
	}
	base := itemDirective(item)
	pl := &plan.Plan{
		ID:        "ephemeral_explore_" + item.ID,
		SessionID: sessionID,
		Kind:      plan.ScenarioPlan,
		Steps: []plan.Step{
			{
				ID:              "probe_a",
				Directive:       base + " — parallel probe A",
				ToolName:        workItemExecuteTool,
				IdempotencyKey:  "ephem_a_" + item.ID,
				EstimatedTokens: 50,
			},
			{
				ID:              "probe_b",
				Directive:       base + " — parallel probe B",
				ToolName:        workItemExecuteTool,
				IdempotencyKey:  "ephem_b_" + item.ID,
				EstimatedTokens: 50,
			},
		},
		BlastRadius: plan.BlastRadius{
			PersistScope: plan.PersistTransient,
		},
	}
	art, err := r.Router.Route(ctx, pl, execute.ChannelRequest{SessionID: sessionID})
	summary := "parallel explore completed"
	if err != nil {
		summary = fmt.Sprintf("parallel explore error: %v", err)
	} else if art != nil && art.Summary != "" {
		summary = art.Summary
	} else if art != nil && art.TaskID != "" {
		summary = fmt.Sprintf("parallel explore artifact %s", art.TaskID)
	}
	round.SpawnRationale = round.SpawnRationale + "; ephemeral: " + summary
	return r.Tasks.Tree().ApplyPipelineRound(sessionID, item.ID, round, workmodel.RoundPhaseIdle)
}
