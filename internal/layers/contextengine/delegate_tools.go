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
		Description: delegateToolDescription(r.role),
		Parameters:  delegateToolParameters(),
	}
}

func delegateToolParameters() string {
	return `{"type":"object","required":["directive"],"properties":{"directive":{"type":"string","description":"Clear, self-contained instruction for the worker (goal, scope, files/modules, expected output)."},"task_id":{"type":"string","description":"Optional TaskManager id; omit to auto-create from directive."},"worktree_slug":{"type":"string","description":"Optional isolated worktree slug for parallel implement tasks."},"async":{"type":"boolean","description":"When true, return immediately and poll delegate_status for progress. Prefer async for explore/plan that may take many tool rounds."}}}`
}

func delegateToolDescription(role delegate.WorkerRole) string {
	switch role {
	case delegate.WorkerRoleExplore:
		return "Spawn a read-only Explore worker to investigate the codebase (grep, read, list — no writes). " +
			"Use when you lack context, must verify assumptions across modules, or the task likely touches 3+ files. " +
			"Do NOT use for trivial single-file edits or pure Q&A you can answer from known context. " +
			"Returns a concise summary — prefer async=true for broad exploration. " +
			"After explore, use todo_write to capture tasks or delegate_plan if the approach is still unclear."
	case delegate.WorkerRolePlan:
		return "Spawn a read-only Plan worker to produce a structured implementation plan from research context. " +
			"Use after explore (or when the user asks for a design) and before multi-step implement work. " +
			"Do NOT use for single obvious edits; do NOT use plan instead of implement when the user wants code changed now. " +
			"Returns phases, files, dependencies, and test notes — then break into todo_write items and delegate_implement per task."
	case delegate.WorkerRoleImplement:
		return "Spawn an Implement worker to execute one scoped task (create/edit files, run tests). " +
			"Use for concrete coding work; pass one task per call with file paths and acceptance criteria in directive. " +
			"Do NOT bundle unrelated features; do NOT implement before exploring unfamiliar areas. " +
			"Link task_id from todo_write when available. Prefer async=true when changing many files or running long test suites."
	default:
		return "Delegate a task to a worker agent (" + string(role) + ")"
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
		Name: "delegate_status",
		Description: "Read the WorkPlan snapshot for this session (running/completed workers, summaries, errors). " +
			"Use after async delegate_* calls instead of re-delegating blindly; use when the user asks for progress " +
			"or before starting the next implement task that depends on a prior worker.",
		Parameters: `{"type":"object","properties":{}}`,
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
