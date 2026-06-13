package builtin

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/prompt/agent"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

const (
	AgentExplore   = "Explore"
	AgentPlan      = "Plan"
	AgentImplement = "Implement"
)

// exploreSystemPrompt is the structured prompt for Explore agents,
// loaded from the embedded prompts/explore.md.
var exploreSystemPrompt = agent.ExplorePrompt

// planSystemPrompt is the structured prompt for Plan agents,
// loaded from the embedded prompts/plan.md.
var planSystemPrompt = agent.PlanPrompt

// implementSystemPrompt is the structured prompt for Implement agents,
// loaded from the embedded prompts/implement.md.
var implementSystemPrompt = agent.ImplementPrompt

// RunExplore runs a read-only exploration sub-query.
func RunExplore(ctx context.Context, deps query.LoopDeps, parent *types.SessionContext, prompt string, tools []query.ToolSchema, maxTurns int) (*query.SubQueryResult, error) {
	return runBuiltin(ctx, deps, parent, AgentExplore, exploreSystemPrompt, prompt, tools, maxTurns, true)
}

// RunPlan runs a read-only planning sub-query.
func RunPlan(ctx context.Context, deps query.LoopDeps, parent *types.SessionContext, prompt string, tools []query.ToolSchema, maxTurns int) (*query.SubQueryResult, error) {
	return runBuiltin(ctx, deps, parent, AgentPlan, planSystemPrompt, prompt, tools, maxTurns, true)
}

// RunImplement runs a read-write implementation sub-query.
func RunImplement(ctx context.Context, deps query.LoopDeps, parent *types.SessionContext, prompt string, tools []query.ToolSchema, maxTurns int) (*query.SubQueryResult, error) {
	if parent == nil {
		return nil, fmt.Errorf("builtin %s: parent context is nil", AgentImplement)
	}
	if maxTurns <= 0 {
		maxTurns = 50
	}
	agentID := fmt.Sprintf("%s_%s", AgentImplement, uuid.New().String()[:8])
	return query.Run(ctx, deps, query.SubQueryParams{
		ParentSC:       parent,
		AgentID:        agentID,
		AgentName:      AgentImplement,
		SystemPrompt:   implementSystemPrompt,
		PromptMessages: []types.Message{{Role: types.MessageRoleUser, Content: prompt, SessionID: parent.SessionID}},
		Tools:          tools,
		MaxTurns:       maxTurns,
		OmitClaudeMd:   false,
		ReadOnlyTools:  false,
		ModelTier:      parent.ModelTier,
	})
}

func runBuiltin(
	ctx context.Context,
	deps query.LoopDeps,
	parent *types.SessionContext,
	name, systemPrompt, prompt string,
	tools []query.ToolSchema,
	maxTurns int,
	readOnly bool,
) (*query.SubQueryResult, error) {
	if parent == nil {
		return nil, fmt.Errorf("builtin %s: parent context is nil", name)
	}
	if maxTurns <= 0 {
		maxTurns = 50
	}
	agentID := fmt.Sprintf("%s_%s", name, uuid.New().String()[:8])
	return query.Run(ctx, deps, query.SubQueryParams{
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
	})
}
