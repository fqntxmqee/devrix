package pev

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// MilestoneRunFunc executes execute→verify for a single milestone.
type MilestoneRunFunc func(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	m *types.Milestone,
	emit func(*contracts.EngineEvent),
) (passed bool, err error)

// MilestoneProgressFunc emits observability for milestone progress updates.
type MilestoneProgressFunc func(sessionID, milestoneID string, progress float64)

// MilestoneRunner drives PEV execution across a milestone DAG.
type MilestoneRunner struct {
	planner    contracts.IMilestonePlanner
	cfg        config.PlanConfig
	runOne     MilestoneRunFunc
	onProgress MilestoneProgressFunc
}

// NewMilestoneRunner creates a milestone runner.
func NewMilestoneRunner(
	planner contracts.IMilestonePlanner,
	cfg config.PlanConfig,
	runOne MilestoneRunFunc,
	onProgress MilestoneProgressFunc,
) *MilestoneRunner {
	return &MilestoneRunner{
		planner:    planner,
		cfg:        cfg,
		runOne:     runOne,
		onProgress: onProgress,
	}
}

// Run executes milestones in topological order for taskID.
func (r *MilestoneRunner) Run(
	ctx context.Context,
	sc *types.SessionContext,
	view []types.Message,
	taskID string,
	emit func(*contracts.EngineEvent),
) error {
	order, err := r.planner.GetExecutionOrder(taskID)
	if err != nil {
		return err
	}
	if len(order) == 0 {
		return nil
	}

	sc.PEVState.ActiveTaskID = taskID
	total := len(order)

	for i, m := range order {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		sc.PEVState.ActiveMilestoneID = m.ID
		progress := float64(i) / float64(total)
		_ = r.planner.UpdateProgress(m.ID, progress)
		emitMilestoneProgress(emit, sc.SessionID, m, progress)
		if r.onProgress != nil {
			r.onProgress(sc.SessionID, m.ID, progress)
		}

		passed, runErr := r.runOne(ctx, sc, view, m, emit)
		if runErr != nil {
			reason := runErr.Error()
			_ = r.planner.Fail(m.ID, reason)
			emitMilestoneInfo(emit, sc.SessionID, fmt.Sprintf("里程碑 %s 失败，跳过后续任务: %s", m.Name, reason))
			if r.cfg.OnMilestoneFail == "" || r.cfg.OnMilestoneFail == "fail_fast" {
				return runErr
			}
			continue
		}
		if !passed {
			reason := "verify failed"
			_ = r.planner.Fail(m.ID, reason)
			emitMilestoneInfo(emit, sc.SessionID, fmt.Sprintf("里程碑 %s 验证未通过，跳过后续任务", m.Name))
			return nil
		}

		_ = r.planner.Complete(m.ID)
		emitMilestoneProgress(emit, sc.SessionID, m, float64(i+1)/float64(total))
		if r.onProgress != nil {
			r.onProgress(sc.SessionID, m.ID, float64(i+1)/float64(total))
		}
	}

	sc.PEVState.Phase = types.PEVPhaseDone
	return nil
}

func emitMilestoneProgress(emit func(*contracts.EngineEvent), sessionID string, m *types.Milestone, progress float64) {
	if emit == nil || m == nil {
		return
	}
	pct := int(progress * 100)
	if pct > 100 {
		pct = 100
	}
	emit(&contracts.EngineEvent{
		Type:      "milestone_progress",
		SessionID: sessionID,
		Metadata: map[string]string{
			"event_type":   "milestone_progress",
			"milestone_id": m.ID,
			"progress":     fmt.Sprintf("%d%%", pct),
			"task":         m.Name,
		},
	})
}

func emitMilestoneInfo(emit func(*contracts.EngineEvent), sessionID, content string) {
	if emit == nil {
		return
	}
	emit(&contracts.EngineEvent{
		Type:      "info",
		Content:   content,
		SessionID: sessionID,
		Metadata:  map[string]string{"category": "milestone"},
	})
}
