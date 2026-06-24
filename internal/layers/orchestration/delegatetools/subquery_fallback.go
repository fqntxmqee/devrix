package delegatetools

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubQueryRunner runs D2 SubQuery when D4 delegate is unavailable.
// Implements hubspoke.SubQueryRunner.
type SubQueryRunner struct {
	LoopDeps enforce.SubQueryDeps
}

// RunSubQuery implements hubspoke.SubQueryRunner.
//
// DM-20260617-008 W2: caller is responsible for setting LoopDeps.FlowReporter
// (was previously auto-derived from flow.GlobalHub when nil).
//
// DM-20260620-001-B (AC6 + AC10) — mode selects sub-agent context inheritance;
// empty defers to SubagentConfig.DefaultMode.
func (f *SubQueryRunner) RunSubQuery(
	ctx context.Context,
	parent *types.SessionContext,
	role, directive, taskID string,
	maxTurns int,
	mode contracts.SubAgentMode,
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
		res, err = RunExplore(ctx, deps, parent, directive, nil, maxTurns, mode)
	case WorkerRolePlan:
		res, err = RunPlan(ctx, deps, parent, directive, nil, maxTurns, mode)
	case WorkerRoleImplement:
		res, err = RunImplement(ctx, deps, parent, directive, nil, maxTurns, mode)
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
			Mode:           mode,
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

// BuildSubQueryRunner creates a hubspoke.SubQueryRunner from turn-runtime deps.
func BuildSubQueryRunner(deps enforce.SubQueryDeps) sessionorchestrator.SubQueryRunner {
	return &SubQueryRunner{LoopDeps: deps}
}
