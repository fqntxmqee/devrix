package sessionorchestrator

import (
	"context"

	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// RunParallelExplore is a no-op stub preserved for API stability.
//
// DM-20260626-009: the legacy Router-based ephemeral probe path is
// decommissioned by the fresh WorkItemExecutor design. ItemPipelineRunner
// no longer holds a Router/ChannelRouter, so the previous
// r.Router.Route(... ScenarioPlan) call cannot resolve. RunParallelExplore
// returns nil so callers (e.g. session_turn_loop.go) continue to compile
// and behave as "no extra probe round was added"; a successor that drives
// ephemeral probes via WorkItemExecutor can be added later without
// touching the call sites.
func (r *ItemPipelineRunner) RunParallelExplore(
	_ context.Context,
	_ string,
	_ *workmodel.WorkItem,
	_ *workmodel.WorkItemPipelineRound,
) error {
	return nil
}