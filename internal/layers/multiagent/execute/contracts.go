// Package execute — D4-S14 ExecuteWorker: pure worker execution contract.
//
// WorkerExecutor accepts a WorkerRunSpec from D7 hubspoke dispatcher,
// performs fork→run→join, and returns WorkerResult.
// It does NOT select Spokes, publish FlowEvents, or interact with ExecutionFlowHub directly.
//
// DSAFT: D4-S14-A01 (ExecuteWorker)
package execute

import (
	"context"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/shared/types"
)

// WorkerRunSpec is the D7 → D4 worker execution request.
// It contains only execution parameters — orchestration decisions
// (Spoke selection, FlowBridge wiring) belong to D7 hubspoke.
//
// Observer is wired per-call by D7 hubspoke so each dispatch
// creates its own AgentBridge without shared mutable state.
type WorkerRunSpec struct {
	Role         string
	Directive    string
	TaskID       string
	SandboxSlug string
	MaxTurns     int
	ModelTier    string
	Observer     WorkerObserver
}

// WorkerResult is the execution outcome returned to D7.
type WorkerResult struct {
	WorkerID string
	Summary  string
	Messages []types.Message
	Error    error
}

// WorkerExecutor performs fork→run→join for a single worker agent.
//
// D7 hubspoke owns FlowBridge wiring and Spoke selection;
// WorkerExecutor owns mechanism correctness (COW isolation, PermissionGate, dedup).
type WorkerExecutor interface {
	ExecuteSync(ctx context.Context, leader multiagent.Agent, spec WorkerRunSpec) (WorkerResult, error)
	ExecuteAsync(ctx context.Context, leader multiagent.Agent, spec WorkerRunSpec) (workerID string, err error)
}
