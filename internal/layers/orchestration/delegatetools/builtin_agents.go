package delegatetools

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompts/agent"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

const (
	AgentExplore   = "Explore"
	AgentPlan      = "Plan"
	AgentImplement = "Implement"
)

// RunExplore runs a read-only exploration sub-query via D2 nested execution.
func RunExplore(ctx context.Context, deps enforce.SubQueryDeps, parent *types.SessionContext, prompt string, tools []contracts.ToolSchema, maxTurns int) (*enforce.SubQueryResult, error) {
	return runBuiltin(ctx, deps, parent, AgentExplore, agent.ExplorePrompt, prompt, tools, maxTurns, true)
}

// RunPlan runs a read-only planning sub-query via D2 nested execution.
func RunPlan(ctx context.Context, deps enforce.SubQueryDeps, parent *types.SessionContext, prompt string, tools []contracts.ToolSchema, maxTurns int) (*enforce.SubQueryResult, error) {
	return runBuiltin(ctx, deps, parent, AgentPlan, agent.PlanPrompt, prompt, tools, maxTurns, true)
}

// RunImplement runs a read-write implementation sub-query via D2 nested execution.
func RunImplement(ctx context.Context, deps enforce.SubQueryDeps, parent *types.SessionContext, prompt string, tools []contracts.ToolSchema, maxTurns int) (*enforce.SubQueryResult, error) {
	if parent == nil {
		return nil, fmt.Errorf("builtin %s: parent context is nil", AgentImplement)
	}
	if maxTurns <= 0 {
		maxTurns = 50
	}
	agentID := fmt.Sprintf("%s_%s", AgentImplement, uuid.New().String()[:8])
	return enforce.Run(ctx, deps, enforce.SubQueryParams{
		ParentSC:       parent,
		AgentID:        agentID,
		AgentName:      AgentImplement,
		SystemPrompt:   agent.ImplementPrompt,
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: prompt, SessionID: parent.SessionID}},
		Tools:          tools,
		MaxTurns:       maxTurns,
		OmitClaudeMd:   false,
		ReadOnlyTools:  false,
		ModelTier:      parent.ModelTier,
		FlowReporter:   deps.FlowReporter,
	})
}

func runBuiltin(
	ctx context.Context,
	deps enforce.SubQueryDeps,
	parent *types.SessionContext,
	name, systemPrompt, prompt string,
	tools []contracts.ToolSchema,
	maxTurns int,
	readOnly bool,
) (*enforce.SubQueryResult, error) {
	if parent == nil {
		return nil, fmt.Errorf("builtin %s: parent context is nil", name)
	}
	if maxTurns <= 0 {
		maxTurns = 50
	}
	agentID := fmt.Sprintf("%s_%s", name, uuid.New().String()[:8])
	return enforce.Run(ctx, deps, enforce.SubQueryParams{
		ParentSC:       parent,
		AgentID:        agentID,
		AgentName:      name,
		SystemPrompt:   systemPrompt,
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: prompt, SessionID: parent.SessionID}},
		Tools:          tools,
		MaxTurns:       maxTurns,
		OmitClaudeMd:   true,
		ReadOnlyTools:  readOnly,
		ModelTier:      parent.ModelTier,
		FlowReporter:   deps.FlowReporter,
	})
}
