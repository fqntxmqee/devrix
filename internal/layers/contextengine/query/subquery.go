package query

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SidechainRecorder persists sub-agent transcript lines.
type SidechainRecorder interface {
	Append(sessionID, agentID string, msg types.Message) error
	Load(sessionID, agentID string) ([]types.Message, error)
}

// SubQueryParams configures a nested agent run (Claude Code runAgent aligned).
type SubQueryParams struct {
	ParentSC       *types.SessionContext
	AgentID        string
	AgentName      string
	SystemPrompt   string
	PromptMessages []types.Message
	ForkMessages   []types.Message
	ForkEnabled    bool
	ForkDirective  string
	Tools          []ToolSchema
	MaxTurns       int
	Model          string
		ModelTier      string
	OmitClaudeMd   bool
	ReadOnlyTools  bool
	Resume         bool
	TaskID         string
	Role           string
	FlowHub        contracts.ExecutionFlowHub
}

// SubQueryResult is the outcome of SubQuery.Run.
type SubQueryResult struct {
	Result  *Result
	ChildSC *types.SessionContext
}

// LoopDeps wires dependencies for SubQuery.
type LoopDeps struct {
	Loop      *Loop
	Sidechain SidechainRecorder
	FlowHub   contracts.ExecutionFlowHub
}

// Run executes a sub-agent query using the shared QueryLoop.
func Run(ctx context.Context, deps LoopDeps, params SubQueryParams) (*SubQueryResult, error) {
	if deps.Loop == nil {
		return nil, fmt.Errorf("subquery: loop is nil")
	}
	if params.ParentSC == nil {
		return nil, fmt.Errorf("subquery: parent session context is nil")
	}

	child := forkChildContext(params)
	initial := buildSubQueryMessages(params, deps.Sidechain)
	tools := params.Tools
	if params.ReadOnlyTools {
		tools = filterReadOnlyTools(tools)
	}

	model := params.Model
	if model == "" {
		model = child.Model
	}
	child.Model = model

	if params.ModelTier != "" {
		child.ModelTier = params.ModelTier
	}

	if deps.Sidechain != nil {
		for _, m := range params.PromptMessages {
			_ = deps.Sidechain.Append(child.SessionID, params.AgentID, m)
		}
	}

	hub := resolveFlowHub(params, deps)
	publishSubQueryFlow(ctx, hub, params, contracts.FlowStarted, params.AgentName, nil)

	res, err := deps.Loop.Run(ctx, child, Params{
		SystemPrompt: params.SystemPrompt,
		Messages:     initial,
		Tools:        tools,
		MaxTurns:     params.MaxTurns,
	}, subQueryFlowEmit(ctx, hub, params, nil))
	if err != nil {
		publishSubQueryFlow(ctx, hub, params, contracts.FlowFailed, err.Error(), nil)
		return nil, err
	}

	if deps.Sidechain != nil && res != nil {
		for _, m := range res.Messages {
			_ = deps.Sidechain.Append(child.SessionID, params.AgentID, m)
		}
	}

	summary := ""
	if res != nil {
		summary = res.AssistantText
	}
	publishSubQueryFlow(ctx, hub, params, contracts.FlowCompleted, summary, nil)

	return &SubQueryResult{Result: res, ChildSC: child}, nil
}

func forkChildContext(params SubQueryParams) *types.SessionContext {
	parent := params.ParentSC
	chainID := parent.QueryChainID
	if chainID == "" {
		chainID = parent.SessionID
	}
	return &types.SessionContext{
		SessionID:      parent.SessionID,
		WorkDir:        parent.WorkDir,
		Model:          parent.Model,
		ModelTier:      parent.ModelTier,
		PermissionMode: parent.PermissionMode,
		PlanFilePath:   parent.PlanFilePath,
		AgentID:        params.AgentID,
		QueryChainID:   chainID,
		QueryDepth:     parent.QueryDepth + 1,
	}
}

func buildSubQueryMessages(params SubQueryParams, sidechain SidechainRecorder) []types.Message {
	if params.ForkEnabled && params.ForkDirective != "" && len(params.ForkMessages) > 0 {
		forked := BuildForkedMessages(params.ForkDirective, params.ForkMessages)
		out := make([]types.Message, 0, len(forked)+len(params.PromptMessages))
		out = append(out, forked...)
		out = append(out, params.PromptMessages...)
		return out
	}

	fork := conversation.FilterIncompleteToolCalls(params.ForkMessages)
	if params.Resume && sidechain != nil && params.AgentID != "" && params.ParentSC != nil {
		if loaded, err := sidechain.Load(params.ParentSC.SessionID, params.AgentID); err == nil && len(loaded) > 0 {
			fork = loaded
		}
	}
	out := make([]types.Message, 0, len(fork)+len(params.PromptMessages))
	out = append(out, fork...)
	out = append(out, params.PromptMessages...)
	return out
}

func filterReadOnlyTools(tools []ToolSchema) []ToolSchema {
	allowed := map[string]bool{
		"read_file": true, "glob": true, "grep": true, "list_dir": true, "bash": true,
	}
	out := make([]ToolSchema, 0, len(tools))
	for _, t := range tools {
		if allowed[t.Name] {
			out = append(out, t)
		}
	}
	return out
}
