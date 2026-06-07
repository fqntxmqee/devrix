package milestonebridge

import (
	"fmt"

	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine/pev"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// PlannerAdapter adapts communication milestone.Service to IMilestonePlanner.
type PlannerAdapter struct {
	svc milestone.IMilestoneService
}

// NewPlannerAdapter wraps milestone service for Layer 2.
func NewPlannerAdapter(svc milestone.IMilestoneService) contracts.IMilestonePlanner {
	return &PlannerAdapter{svc: svc}
}

// CreateBatch creates milestones and wires dependencies.
func (a *PlannerAdapter) CreateBatch(taskID string, milestones []*types.Milestone) error {
	for _, m := range milestones {
		if m.TaskID == "" {
			m.TaskID = taskID
		}
		if err := a.svc.Create(m); err != nil {
			return fmt.Errorf("create milestone %s: %w", m.ID, err)
		}
	}
	for _, m := range milestones {
		for _, dep := range m.Dependencies {
			if err := a.svc.AddDependency(m.ID, dep); err != nil {
				return fmt.Errorf("add dependency %s->%s: %w", m.ID, dep, err)
			}
		}
	}
	return nil
}

// GetExecutionOrder returns milestones for a task in topological order.
func (a *PlannerAdapter) GetExecutionOrder(taskID string) ([]*types.Milestone, error) {
	dag := a.svc.GetDAG()
	if dag == nil {
		return nil, fmt.Errorf("milestone DAG not available")
	}
	var subset []*types.Milestone
	for _, m := range dag.Milestones {
		if m.TaskID == taskID {
			subset = append(subset, m)
		}
	}
	if len(subset) == 0 {
		return nil, fmt.Errorf("no milestones for task %s", taskID)
	}
	return pev.TopologicalSort(subset)
}

// UpdateProgress updates milestone progress.
func (a *PlannerAdapter) UpdateProgress(id string, progress float64) error {
	return a.svc.UpdateProgress(id, progress)
}

// Complete marks a milestone completed.
func (a *PlannerAdapter) Complete(id string) error {
	return a.svc.Complete(id)
}

// Fail marks a milestone failed.
func (a *PlannerAdapter) Fail(id string, reason string) error {
	return a.svc.Fail(id, reason)
}
