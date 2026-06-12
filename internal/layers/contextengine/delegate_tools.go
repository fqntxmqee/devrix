package contextengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/builtin"
	"github.com/devrix/devrix/internal/layers/multiagent/delegate"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// SubQueryFallbackAdapter runs L1 SubQuery when D4 is unavailable.
type SubQueryFallbackAdapter struct {
	LoopDeps query.LoopDeps
}

// RunSubQuery implements delegate.SubQueryFallback.
func (a *SubQueryFallbackAdapter) RunSubQuery(ctx context.Context, parent *types.SessionContext, spec delegate.WorkerSpec) (string, error) {
	if a == nil || parent == nil {
		return "", fmt.Errorf("subquery fallback: parent context is nil")
	}
	deps := a.LoopDeps
	deps.FlowHub = flow.GlobalHub
	maxTurns := spec.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 50
	}
	var (
		res *query.SubQueryResult
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
		res, err = query.Run(ctx, deps, query.SubQueryParams{
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
func BuildSubQueryFallback(deps query.LoopDeps) delegate.SubQueryFallback {
	return &SubQueryFallbackAdapter{LoopDeps: deps}
}

// DelegateToolsDeps wires delegate tool handlers.
type DelegateToolsDeps struct {
	Service *delegate.Service
	Leader  delegate.LeaderResolver
}

var globalDelegateTools DelegateToolsDeps

// SetDelegateToolsDeps configures delegate_* tool handlers.
func SetDelegateToolsDeps(deps DelegateToolsDeps) {
	globalDelegateTools = deps
}

// RegisterDelegateTools registers hub-spoke delegate tools when enabled.
func RegisterDelegateTools(reg *ToolRegistry, maCfg *config.MultiAgentConfig) error {
	if reg == nil || maCfg == nil || !maCfg.Delegate.Enabled {
		return nil
	}
	for _, runner := range []PluginRunner{
		&delegateToolRunner{name: "delegate_explore", role: delegate.WorkerRoleExplore},
		&delegateToolRunner{name: "delegate_plan", role: delegate.WorkerRolePlan},
		&delegateToolRunner{name: "delegate_implement", role: delegate.WorkerRoleImplement},
		newDelegateStatusRunner(),
	} {
		if err := reg.Register(runner); err != nil {
			return err
		}
	}
	return nil
}

type delegateToolRunner struct {
	name string
	role delegate.WorkerRole
}

func (r *delegateToolRunner) Name() string { return r.name }

func (r *delegateToolRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *delegateToolRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        r.name,
		Description: "Delegate a task to a worker agent (" + string(r.role) + ")",
		Parameters:  `{"type":"object","required":["directive"],"properties":{"directive":{"type":"string"},"task_id":{"type":"string"},"worktree_slug":{"type":"string"},"async":{"type":"boolean"}}}`,
	}
}

func (r *delegateToolRunner) Execute(ctx context.Context, _, input string) (*ToolResult, error) {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &ToolResult{Error: r.name + ": session context unavailable"}, nil
	}
	if sc.IsWorker || sc.AgentID != "" {
		return &ToolResult{Error: r.name + ": not allowed from worker context"}, nil
	}
	svc := globalDelegateTools.Service
	if svc == nil {
		return &ToolResult{Error: r.name + ": delegate service not configured"}, nil
	}
	fields := parseToolInput(input)
	directive := fields["directive"]
	if directive == "" {
		return &ToolResult{Error: r.name + ": directive is required"}, nil
	}
	spec := delegate.WorkerSpec{
		Role:         r.role,
		Directive:    directive,
		TaskID:       resolveDelegateTaskID(sc.SessionID, fields["task_id"], directive),
		WorktreeSlug: fields["worktree_slug"],
		Async:        fields["async"] == "true",
	}
	var leader multiagent.Agent
	if globalDelegateTools.Leader != nil {
		leader, _ = globalDelegateTools.Leader.Leader(sc.SessionID)
	}
	res, err := svc.DelegateOrFallback(ctx, leader, sc, spec)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	out := strings.TrimSpace(res.Summary)
	if out == "" && res.Error == nil {
		out = "(subagent completed without a textual summary)"
	}
	return &ToolResult{Output: out}, nil
}

type delegateStatusRunner struct{}

func newDelegateStatusRunner() *delegateStatusRunner { return &delegateStatusRunner{} }

func (r *delegateStatusRunner) Name() string { return "delegate_status" }

func (r *delegateStatusRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *delegateStatusRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "delegate_status",
		Description: "Read WorkPlan execution flow snapshot for the session",
		Parameters:  `{"type":"object","properties":{}}`,
	}
}

func (r *delegateStatusRunner) Execute(ctx context.Context, _, _ string) (*ToolResult, error) {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &ToolResult{Error: "delegate_status: session context unavailable"}, nil
	}
	snap := flow.GlobalHub.Snapshot(sc.SessionID)
	data, err := json.Marshal(snap)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	return &ToolResult{Output: string(data)}, nil
}

func resolveDelegateTaskID(sessionID, taskID, directive string) string {
	if id := strings.TrimSpace(taskID); id != "" {
		return id
	}
	if tasks.GlobalTaskManager == nil || sessionID == "" {
		return ""
	}
	subject := strings.TrimSpace(directive)
	if subject == "" {
		subject = "delegated work"
	}
	if len(subject) > 120 {
		subject = subject[:117] + "..."
	}
	return tasks.GlobalTaskManager.Create(sessionID, subject, directive).ID
}
