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
//
// DM-20260620-001-B (AC6 + AC10) — mode (brief/fork/full) selects sub-agent
// context inheritance; empty defers to SubagentConfig.DefaultMode.
func RunExplore(ctx context.Context, deps enforce.SubQueryDeps, parent *types.SessionContext, prompt string, tools []contracts.ToolSchema, maxTurns int, mode contracts.SubAgentMode) (*enforce.SubQueryResult, error) {
	return runSubagent(ctx, deps, parent, AgentExplore, agent.ExplorePrompt, prompt, tools, maxTurns, true, true, mode)
}

// RunPlan runs a read-only planning sub-query via D2 nested execution.
func RunPlan(ctx context.Context, deps enforce.SubQueryDeps, parent *types.SessionContext, prompt string, tools []contracts.ToolSchema, maxTurns int, mode contracts.SubAgentMode) (*enforce.SubQueryResult, error) {
	return runSubagent(ctx, deps, parent, AgentPlan, agent.PlanPrompt, prompt, tools, maxTurns, true, true, mode)
}

// RunImplement runs a read-write implementation sub-query via D2 nested execution.
func RunImplement(ctx context.Context, deps enforce.SubQueryDeps, parent *types.SessionContext, prompt string, tools []contracts.ToolSchema, maxTurns int, mode contracts.SubAgentMode) (*enforce.SubQueryResult, error) {
	return runSubagent(ctx, deps, parent, AgentImplement, agent.ImplementPrompt, prompt, tools, maxTurns, false, false, mode)
}

// runSubagent executes one built-in D2 nested sub-query. Read-only variants
// (Explore/Plan) set OmitClaudeMd=true + ReadOnlyTools=true so the worker
// runs without project memory and only with read tools; the Implement
// variant runs full access.
func runSubagent(
	ctx context.Context,
	deps enforce.SubQueryDeps,
	parent *types.SessionContext,
	name, systemPrompt, prompt string,
	tools []contracts.ToolSchema,
	maxTurns int,
	readOnly, omitClaudeMd bool,
	mode contracts.SubAgentMode,
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
		OmitClaudeMd:   omitClaudeMd,
		ReadOnlyTools:  readOnly,
		ModelTier:      parent.ModelTier,
		Mode:           mode,
		FlowReporter:   deps.FlowReporter,
	})
}
