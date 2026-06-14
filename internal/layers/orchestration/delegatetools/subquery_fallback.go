package delegatetools

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/nested"
	"github.com/devrix/devrix/internal/layers/multiagent/builtin"
	"github.com/devrix/devrix/internal/layers/multiagent/delegate"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubQueryFallback runs D2 SubQuery when D4 delegate is unavailable.
type SubQueryFallback struct {
	LoopDeps nested.LoopDeps
}

// RunSubQuery implements delegate.SubQueryFallback.
func (f *SubQueryFallback) RunSubQuery(ctx context.Context, parent *types.SessionContext, spec delegate.WorkerSpec) (string, error) {
	if f == nil || parent == nil {
		return "", fmt.Errorf("subquery fallback: parent context is nil")
	}
	deps := f.LoopDeps
	deps.FlowHub = flow.GlobalHub
	maxTurns := spec.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}
	var (
		res *nested.SubQueryResult
		err error
	)
	switch spec.Role {
	case delegate.WorkerRoleExplore:
		res, err = builtin.RunExplore(ctx, deps, parent, spec.Directive, nil, maxTurns)
	case delegate.WorkerRolePlan:
		res, err = builtin.RunPlan(ctx, deps, parent, spec.Directive, nil, maxTurns)
	case delegate.WorkerRoleImplement:
		res, err = builtin.RunImplement(ctx, deps, parent, spec.Directive, nil, maxTurns)
	default:
		res, err = nested.Run(ctx, deps, nested.SubQueryParams{
			ParentSC:       parent,
			AgentID:        fmt.Sprintf("implement_%s", spec.TaskID),
			AgentName:      "implement",
			Role:           string(spec.Role),
			TaskID:         spec.TaskID,
			SystemPrompt:   delegate.SystemPromptForRole(delegate.WorkerRoleImplement),
			PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: spec.Directive, SessionID: parent.SessionID}},
			MaxTurns:       maxTurns,
			FlowHub:        flow.GlobalHub,
		})
	}
	if err != nil {
		return "", err
	}
	if res == nil || res.Result == nil {
		return "", nil
	}
	return res.Result.AssistantText, nil
}

// BuildSubQueryFallback creates a fallback adapter when QueryLoop deps are available.
func BuildSubQueryFallback(deps nested.LoopDeps) delegate.SubQueryFallback {
	return &SubQueryFallback{LoopDeps: deps}
}
