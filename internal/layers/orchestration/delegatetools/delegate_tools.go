// Package delegatetools — D7 orchestration F: delegate_* tool routing (DM-20260614-011).
//
// DSAFT: D7-S2/S5 F — routes Leader tool calls through hubspoke.Dispatcher.
// Moved from contextengine (v2.0) to keep D2 as Execution Follower only.
package delegatetools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// WorkerRole identifies the delegated worker specialization.
type WorkerRole string

const (
	WorkerRoleExplore   WorkerRole = "explore"
	WorkerRolePlan      WorkerRole = "plan"
	WorkerRoleImplement WorkerRole = "implement"
)

// RegisterTools registers hub-spoke delegate tools when enabled.
func RegisterTools(reg *toolrunner.ToolRegistry, maCfg *config.MultiAgentConfig) error {
	if reg == nil || maCfg == nil || !maCfg.Delegate.Enabled {
		return nil
	}
	for _, runner := range []toolrunner.PluginRunner{
		&delegateToolRunner{name: "delegate_explore", role: WorkerRoleExplore},
		&delegateToolRunner{name: "delegate_plan", role: WorkerRolePlan},
		&delegateToolRunner{name: "delegate_implement", role: WorkerRoleImplement},
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
	role WorkerRole
}

func (r *delegateToolRunner) Name() string { return r.name }

func (r *delegateToolRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *delegateToolRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name:        r.name,
		Description: delegateToolDescription(r.role),
		Parameters:  delegateToolParameters(),
	}
}

func delegateToolParameters() string {
	return `{"type":"object","required":["directive"],"properties":{"directive":{"type":"string","description":"Clear, self-contained instruction for the worker (goal, scope, files/modules, expected output)."},"task_id":{"type":"string","description":"Optional TaskManager id; omit to auto-create from directive."},"sandbox_slug":{"type":"string","description":"Optional isolated worker directory slug for parallel implement tasks."},"worktree_slug":{"type":"string","description":"Deprecated alias for sandbox_slug."},"async":{"type":"boolean","description":"When true, return immediately and poll delegate_status for progress. Prefer async for explore/plan that may take many tool rounds."}}}`
}

func delegateToolDescription(role WorkerRole) string {
	switch role {
	case WorkerRoleExplore:
		return "Spawn a read-only Explore worker to investigate the codebase (grep, read, list — no writes). " +
			"Do NOT use for trivial single-file edits or pure Q&A you can answer from known context. " +
			"Returns a concise summary — prefer async=true for broad exploration. " +
			"After explore, use todo_write to capture tasks or delegate_plan if the approach is still unclear."
	case WorkerRolePlan:
		return "Spawn a read-only Plan worker to produce a structured implementation plan from research context. " +
			"Do NOT use for single obvious edits; do NOT use plan instead of implement when the user wants code changed now. " +
			"Returns phases, files, dependencies, and test notes — then break into todo_write items and delegate_implement per task."
	case WorkerRoleImplement:
		return "Spawn an Implement worker to execute one scoped task (create/edit files, run tests). " +
			"Use for concrete coding work; pass one task per call with file paths and acceptance criteria in directive. " +
			"Do NOT bundle unrelated features; do NOT implement before exploring unfamiliar areas. " +
			"Link task_id from todo_write when available. Prefer async=true when changing many files or running long test suites."
	default:
		return "Delegate a task to a worker agent (" + string(role) + ")"
	}
}

func (r *delegateToolRunner) Execute(ctx context.Context, _, input string) (*toolrunner.ToolResult, error) {
	sc := toolrunner.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &toolrunner.ToolResult{Error: r.name + ": session context unavailable"}, nil
	}
	if sc.IsWorker || sc.AgentID != "" {
		return &toolrunner.ToolResult{Error: r.name + ": not allowed from worker context"}, nil
	}
	disp := globalDeps.Dispatcher
	if disp == nil {
		return &toolrunner.ToolResult{Error: r.name + ": dispatcher not configured"}, nil
	}
	fields := toolrunner.ParseToolInput(input)
	directive := fields["directive"]
	if directive == "" {
		return &toolrunner.ToolResult{Error: r.name + ": directive is required"}, nil
	}

	sessionID := sc.SessionID
	if globalDeps.Leader != nil {
		if leader, ok := globalDeps.Leader.Leader(sessionID); ok {
			sessionID = leader.Config().SessionID
		}
	}

	req := hubspoke.DispatchRequest{
		SessionID:    sessionID,
		ParentSC:     sc,
		Role:         string(r.role),
		Directive:    directive,
		TaskID:       resolveDelegateTaskID(sc.SessionID, fields["task_id"], directive, workmodel.ResolveFocusKind(string(r.role))),
		SandboxSlug: resolveSandboxSlug(fields),
		Async:        fields["async"] == "true",
	}

	runID, _ := workmodel.SpawnForWorkItem(sessionID, req.TaskID, string(r.role), globalDeps.Tasks)

	res, err := disp.Dispatch(ctx, req)
	if err != nil {
		if runID != "" && runregistry.Global != nil {
			runregistry.Global.SetTerminal(runID, runregistry.StatusFailed, "", err.Error())
		}
		return &toolrunner.ToolResult{Error: err.Error()}, nil
	}
	if runID != "" && runregistry.Global != nil && !req.Async {
		st := runregistry.StatusCompleted
		errStr := ""
		if res.Error != nil {
			st = runregistry.StatusFailed
			errStr = res.Error.Error()
		}
		runregistry.Global.SetTerminal(runID, st, res.Summary, errStr)
	}

	out := strings.TrimSpace(res.Summary)
	if out == "" && res.Error == nil {
		out = "(subagent completed without a textual summary)"
	}
	return &toolrunner.ToolResult{Output: out}, nil
}

type delegateStatusRunner struct{}

func newDelegateStatusRunner() *delegateStatusRunner { return &delegateStatusRunner{} }

func (r *delegateStatusRunner) Name() string { return "delegate_status" }

func (r *delegateStatusRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *delegateStatusRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name: "delegate_status",
		Description: "Read the WorkPlan snapshot for this session (running/completed workers, summaries, errors). " +
			"Use after async delegate_* calls instead of re-delegating blindly; use when the user asks for progress " +
			"or before starting the next implement task that depends on a prior worker.",
		Parameters: `{"type":"object","properties":{}}`,
	}
}

func (r *delegateStatusRunner) Execute(ctx context.Context, _, _ string) (*toolrunner.ToolResult, error) {
	sc := toolrunner.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &toolrunner.ToolResult{Error: "delegate_status: session context unavailable"}, nil
	}
	snap := globalDeps.Dispatcher.Hub().Snapshot(sc.SessionID)
	data, err := json.Marshal(snap)
	if err != nil {
		return &toolrunner.ToolResult{Error: err.Error()}, nil
	}
	return &toolrunner.ToolResult{Output: string(data)}, nil
}

func resolveDelegateTaskID(sessionID, taskID, directive string, kind workmodel.WorkKind) string {
	if id := strings.TrimSpace(taskID); id != "" {
		return id
	}
	tm := globalDeps.Tasks
	if tm == nil || sessionID == "" {
		return ""
	}
	subject := strings.TrimSpace(directive)
	if subject == "" {
		subject = "delegated work"
	}
	if len(subject) > 120 {
		subject = subject[:117] + "..."
	}
	goal, _ := tm.EnsureGoal(sessionID, subject)
	parentID := ""
	if goal != nil {
		parentID = goal.ID
	}
	item, err := tm.CreateWorkItem(sessionID, workmodel.CreateWorkItemInput{
		ParentID:  parentID,
		Kind:      kind,
		Title:     subject,
		Directive: directive,
	})
	if err != nil || item == nil {
		return tm.Create(sessionID, subject, directive).ID
	}
	return item.ID
}

func resolveSandboxSlug(fields map[string]string) string {
	if v := strings.TrimSpace(fields["sandbox_slug"]); v != "" {
		return v
	}
	return strings.TrimSpace(fields["worktree_slug"])
}
