package bootstrap

import (
	"log/slog"

	milestonebridge "github.com/devrix/devrix/internal/bridges/milestone"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireContextV3 builds optional Plan planner and LongTerm memory from config.
func WireContextV3(
	ctxCfg *config.ContextEngineConfig,
	milestoneSvc milestone.IMilestoneService,
) (contracts.IMilestonePlanner, memory.ILongTermMemory) {
	if ctxCfg == nil {
		ctxCfg = config.DefaultContextEngineConfig()
	}

	var planner contracts.IMilestonePlanner
	if ctxCfg.Plan.Enabled && milestoneSvc != nil {
		planner = milestonebridge.NewPlannerAdapter(milestoneSvc)
	}

	longTerm, err := memory.NewLongTermFromConfig(ctxCfg.LongTerm)
	if err != nil {
		slog.Warn("longterm memory init failed, using disabled stub", "error", err)
		longTerm = memory.NewDisabledLongTermMemory()
	}
	return planner, longTerm
}
