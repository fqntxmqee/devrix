package nested

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
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
	Tools          []query.ToolSchema
	MaxTurns       int
	Model          string
	ModelTier      string
	OmitClaudeMd   bool
	ReadOnlyTools  bool
	Resume         bool
	TaskID         string
	Role           string
	FlowReporter   contracts.SubQueryFlowReporter
}

// SubQueryResult is the outcome of SubQuery.Run.
type SubQueryResult struct {
	Result  *query.Result
	ChildSC *types.SessionContext
}

// LoopDeps wires dependencies for SubQuery.
type LoopDeps struct {
	Loop         *query.Loop
	Sidechain    SidechainRecorder
	FlowReporter contracts.SubQueryFlowReporter
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

	flowParams := flowParamsFromSubQuery(params)
	reporter := params.FlowReporter
	if reporter == nil {
		reporter = deps.FlowReporter
	}
	if reporter != nil {
		reporter.OnStarted(ctx, flowParams, params.AgentName)
	}

	emit := contracts.EngineEmitFunc(nil)
	if reporter != nil {
		emit = reporter.WrapEmit(ctx, flowParams, nil)
	}

	res, err := deps.Loop.Run(ctx, child, query.Params{
		SystemPrompt: params.SystemPrompt,
		Messages:     initial,
		Tools:        tools,
		MaxTurns:     params.MaxTurns,
	}, query.EmitFunc(emit))
	if err != nil {
		if reporter != nil {
			reporter.OnFailed(ctx, flowParams, err.Error())
		}
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
	if reporter != nil {
		reporter.OnCompleted(ctx, flowParams, summary)
	}

	return &SubQueryResult{Result: res, ChildSC: child}, nil
}

func flowParamsFromSubQuery(params SubQueryParams) contracts.SubQueryFlowParams {
	sessionID := ""
	if params.ParentSC != nil {
		sessionID = params.ParentSC.SessionID
	}
	return contracts.SubQueryFlowParams{
		SessionID: sessionID,
		AgentID:   params.AgentID,
		AgentName: params.AgentName,
		TaskID:    params.TaskID,
		Role:      params.Role,
	}
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

func filterReadOnlyTools(tools []query.ToolSchema) []query.ToolSchema {
	allowed := map[string]bool{
		"read_file": true, "glob": true, "grep": true, "list_dir": true, "bash": true,
	}
	out := make([]query.ToolSchema, 0, len(tools))
	for _, t := range tools {
		if allowed[t.Name] {
			out = append(out, t)
		}
	}
	return out
}
