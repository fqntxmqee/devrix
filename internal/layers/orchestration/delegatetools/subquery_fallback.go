package delegatetools

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubQueryRunner runs D2 SubQuery when D4 delegate is unavailable.
// Implements hubspoke.SubQueryRunner.
type SubQueryRunner struct {
	LoopDeps enforce.LoopDeps
}

// RunSubQuery implements hubspoke.SubQueryRunner.
//
// DM-20260617-008 W2: caller is responsible for setting LoopDeps.FlowReporter
// (was previously auto-derived from flow.GlobalHub when nil).
func (f *SubQueryRunner) RunSubQuery(
	ctx context.Context,
	parent *types.SessionContext,
	role, directive, taskID string,
	maxTurns int,
) (string, error) {
	if f == nil || parent == nil {
		return "", fmt.Errorf("subquery fallback: parent context is nil")
	}
	deps := f.LoopDeps
	if maxTurns <= 0 {
		maxTurns = 50
	}
	var (
		res *enforce.SubQueryResult
		err error
	)
	switch WorkerRole(role) {
	case WorkerRoleExplore:
		res, err = RunExplore(ctx, deps, parent, directive, nil, maxTurns)
	case WorkerRolePlan:
		res, err = RunPlan(ctx, deps, parent, directive, nil, maxTurns)
	case WorkerRoleImplement:
		res, err = RunImplement(ctx, deps, parent, directive, nil, maxTurns)
	default:
		res, err = enforce.Run(ctx, deps, enforce.SubQueryParams{
			ParentSC:       parent,
			AgentID:        fmt.Sprintf("implement_%s", taskID),
			AgentName:      "implement",
			Role:           role,
			TaskID:         taskID,
			SystemPrompt:   systemPromptForRole(role),
			PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: directive, SessionID: parent.SessionID}},
			MaxTurns:       maxTurns,
			FlowReporter:   deps.FlowReporter,
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

func systemPromptForRole(role string) string {
	switch WorkerRole(role) {
	case WorkerRoleExplore:
		return explorePrompt
	case WorkerRolePlan:
		return planPrompt
	default:
		return implementPrompt
	}
}

// BuildSubQueryRunner creates a hubspoke.SubQueryRunner from QueryLoop deps.
func BuildSubQueryRunner(deps enforce.LoopDeps) hubspoke.SubQueryRunner {
	return &SubQueryRunner{LoopDeps: deps}
}
