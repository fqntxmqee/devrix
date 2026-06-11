package builtin

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

const (
	AgentExplore = "Explore"
	AgentPlan    = "Plan"
)

var exploreSystemPrompt = `You are the Explore sub-agent. Investigate the codebase read-only.
Use read_file, glob, grep, list_dir, and bash (read-only commands) to gather facts.
Return concise findings; do not modify files.`

var planSystemPrompt = `You are the Plan sub-agent. Produce an implementation plan from exploration context.
Use read-only tools only. Output structured steps and critical files; do not edit source files.`

// RunExplore runs a read-only exploration sub-query.
func RunExplore(ctx context.Context, deps query.LoopDeps, parent *types.SessionContext, prompt string, tools []query.ToolSchema, maxTurns int) (*query.SubQueryResult, error) {
	return runBuiltin(ctx, deps, parent, AgentExplore, exploreSystemPrompt, prompt, tools, maxTurns, true)
}

// RunPlan runs a read-only planning sub-query.
func RunPlan(ctx context.Context, deps query.LoopDeps, parent *types.SessionContext, prompt string, tools []query.ToolSchema, maxTurns int) (*query.SubQueryResult, error) {
	return runBuiltin(ctx, deps, parent, AgentPlan, planSystemPrompt, prompt, tools, maxTurns, true)
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
	})
}
