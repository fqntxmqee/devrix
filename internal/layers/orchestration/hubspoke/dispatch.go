package hubspoke

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// LeaderResolver returns the session leader agent when D4 is active.
type LeaderResolver interface {
	Leader(sessionID string) (multiagent.Agent, bool)
}

// SubQueryRunner is the D2 fallback when D4 delegate is unavailable.
type SubQueryRunner interface {
	RunSubQuery(ctx context.Context, parent *types.SessionContext, role, directive, taskID string, maxTurns int) (summary string, err error)
}

// DispatchRequest is the unified entry for delegate_* tool calls.
type DispatchRequest struct {
	SessionID    string
	ParentSC     *types.SessionContext
	Role         string
	Directive    string
	TaskID       string
	WorktreeSlug string
	Async        bool
	MaxTurns     int
	ModelTier    string
}

// DispatchResult is returned after a Spoke completes execution.
type DispatchResult struct {
	WorkerID string
	Role     string
	Summary  string
	Error    error
}

// Dispatcher is the D7 Hub-Spoke entry point.
//
// It owns Spoke selection (D4 Worker vs D2 SubQuery), FlowBridge wiring,
// and terminal FlowEvent publishing.
//
// DSAFT: D7-S2-A04 (DispatchWorker)
type Dispatcher struct {
	cfg       config.DelegateConfig
	executor  execute.WorkerExecutor
	subQuery  SubQueryRunner
	hub       contracts.ExecutionFlowHub
	leaderRes LeaderResolver
}

// NewDispatcher creates a SpokeDispatcher.
func NewDispatcher(
	cfg config.DelegateConfig,
	exec execute.WorkerExecutor,
	subQuery SubQueryRunner,
	hub contracts.ExecutionFlowHub,
	leaderRes LeaderResolver,
) *Dispatcher {
	if hub == nil {
		hub = flow.GlobalHub
	}
	return &Dispatcher{
		cfg:       cfg,
		executor:  exec,
		subQuery:  subQuery,
		hub:       hub,
		leaderRes: leaderRes,
	}
}

// Dispatch routes a delegate request to the appropriate Spoke.
func (d *Dispatcher) Dispatch(ctx context.Context, req DispatchRequest) (DispatchResult, error) {
	if d.cfg.Enabled {
		// Try D4 Worker Spoke first
		leader, hasLeader := d.leaderRes.Leader(req.SessionID)
		if hasLeader && leader != nil {
			return d.dispatchToD4(ctx, leader, req)
		}
	}
	// Fallback to D2 SubQuery Spoke
	return d.dispatchToD2(ctx, req)
}

func (d *Dispatcher) dispatchToD4(ctx context.Context, leader multiagent.Agent, req DispatchRequest) (DispatchResult, error) {
	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}

	// Create a per-call AgentBridge so D7 owns FlowEvent publishing.
	bridge := NewAgentBridge(d.hub, req.SessionID, "", "", req.TaskID, req.Role)

	spec := execute.WorkerRunSpec{
		Role:         req.Role,
		Directive:    req.Directive,
		TaskID:       req.TaskID,
		WorktreeSlug: req.WorktreeSlug,
		MaxTurns:     maxTurns,
		ModelTier:    req.ModelTier,
		Observer:     bridge,
	}

	if req.Async {
		workerID, err := d.executor.ExecuteAsync(ctx, leader, spec)
		if err != nil {
			return DispatchResult{}, err
		}
		return DispatchResult{WorkerID: workerID, Role: req.Role, Summary: "async worker started: " + workerID}, nil
	}

	result, err := d.executor.ExecuteSync(ctx, leader, spec)
	if err != nil {
		return DispatchResult{}, err
	}
	return DispatchResult{
		WorkerID: result.WorkerID,
		Role:     req.Role,
		Summary:  result.Summary,
		Error:    result.Error,
	}, nil
}

func (d *Dispatcher) dispatchToD2(ctx context.Context, req DispatchRequest) (DispatchResult, error) {
	if d.subQuery == nil || req.ParentSC == nil {
		return DispatchResult{}, fmt.Errorf("hubspoke: no leader and no subquery fallback")
	}
	summary, err := d.subQuery.RunSubQuery(ctx, req.ParentSC, req.Role, req.Directive, req.TaskID, req.MaxTurns)
	return DispatchResult{Role: req.Role, Summary: summary, Error: err}, err
}

// Hub returns the execution flow hub for publishing bridge events.
func (d *Dispatcher) Hub() contracts.ExecutionFlowHub {
	return d.hub
}
