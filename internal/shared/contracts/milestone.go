package contracts

import "github.com/devrix/devrix/internal/shared/types"

// IMilestonePlanner abstracts milestone DAG operations for Layer 2.
// Implemented by bridges/milestone adapter wrapping communication layer service.
type IMilestonePlanner interface {
	CreateBatch(taskID string, milestones []*types.Milestone) error
	GetExecutionOrder(taskID string) ([]*types.Milestone, error)
	UpdateProgress(id string, progress float64) error
	Complete(id string) error
	Fail(id string, reason string) error
}
