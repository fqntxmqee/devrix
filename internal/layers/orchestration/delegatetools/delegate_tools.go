// Package delegatetools — D7 orchestration F: delegate_* tool routing (DM-20260614-011).
//
// DSAFT: D7-S2/S5 F — routes Leader tool calls through hubspoke.Dispatcher.
// Moved from contextengine (v2.0) to keep D2 as Execution Follower only.
package delegatetools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
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
func RegisterTools(reg *tools.ToolRegistry, maCfg *config.MultiAgentConfig) error {
	if reg == nil || maCfg == nil || !maCfg.Delegate.Enabled {
		return nil
	}
	for _, runner := range []tools.PluginRunner{
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

func (r *delegateToolRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        r.name,
		Description: delegateToolDescription(r.role),
		Parameters:  delegateToolParameters(),
	}
}

func delegateToolParameters() string {
	// DM-20260620-001-B (AC10) — `mode` field selects sub-agent context
	// inheritance: brief (default, no parent history), fork (cache-friendly
	// prefix), full (legacy — full parent history).
	return `{"type":"object","required":["directive"],"properties":{"directive":{"type":"string","description":"Clear, self-contained instruction for the worker (goal, scope, files/modules, expected output)."},"task_id":{"type":"string","description":"Optional TaskManager id; omit to auto-create from directive."},"sandbox_slug":{"type":"string","description":"Optional isolated worker directory slug for parallel implement tasks."},"worktree_slug":{"type":"string","description":"Deprecated alias for sandbox_slug."},"async":{"type":"boolean","description":"When true, return immediately and poll delegate_status for progress. Prefer async for explore/plan that may take many tool rounds."},"mode":{"type":"string","enum":["brief","fork","full"],"default":"brief","description":"Sub-agent context inheritance mode (DM-20260620-001-B / AC10). brief = no parent history (default); fork = cache-friendly prefix for sibling workers; full = full parent history (legacy)."}}}`
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

func (r *delegateToolRunner) Execute(ctx context.Context, _, input string) (*tools.ToolResult, error) {
	sc := tools.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &tools.ToolResult{Error: r.name + ": session context unavailable"}, nil
	}
	if sc.IsWorker || sc.AgentID != "" {
		return &tools.ToolResult{Error: r.name + ": not allowed from worker context"}, nil
	}
	disp := globalDeps.Dispatcher
	if disp == nil {
		return &tools.ToolResult{Error: r.name + ": dispatcher not configured"}, nil
	}
	fields := tools.ParseToolInput(input)
	directive := fields["directive"]
	if directive == "" {
		return &tools.ToolResult{Error: r.name + ": directive is required"}, nil
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
		TaskID: func() string {
			id, _ := resolveDelegateTaskID(sc.SessionID, fields["task_id"], directive, workmodel.ResolveFocusKind(string(r.role)))
			return id
		}(),
		SandboxSlug: resolveSandboxSlug(fields),
		Async:        fields["async"] == "true",
		// DM-20260620-001-B (AC6 + AC10) — read `mode` from tool input; empty
		// defers to SubagentConfig.DefaultMode.
		Mode:         parseSubAgentMode(fields["mode"]),
	}

	runID, _ := workmodel.SpawnForWorkItem(sessionID, req.TaskID, string(r.role), globalDeps.Tasks)

	res, err := disp.Dispatch(ctx, req)
	if err != nil {
		if runID != "" && runregistry.Global != nil {
			runregistry.Global.SetTerminal(runID, runregistry.StatusFailed, "", err.Error())
		}
		return &tools.ToolResult{Error: err.Error()}, nil
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
	return &tools.ToolResult{Output: out}, nil
}

type delegateStatusRunner struct{}

func newDelegateStatusRunner() *delegateStatusRunner { return &delegateStatusRunner{} }

func (r *delegateStatusRunner) Name() string { return "delegate_status" }

func (r *delegateStatusRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *delegateStatusRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name: "delegate_status",
		Description: "Read the WorkPlan snapshot for this session (running/completed workers, summaries, errors). " +
			"Use after async delegate_* calls instead of re-delegating blindly; use when the user asks for progress " +
			"or before starting the next implement task that depends on a prior worker.",
		Parameters: `{"type":"object","properties":{}}`,
	}
}

func (r *delegateStatusRunner) Execute(ctx context.Context, _, _ string) (*tools.ToolResult, error) {
	sc := tools.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &tools.ToolResult{Error: "delegate_status: session context unavailable"}, nil
	}
	snap := globalDeps.Dispatcher.Hub().Snapshot(sc.SessionID)
	data, err := json.Marshal(snap)
	if err != nil {
		return &tools.ToolResult{Error: err.Error()}, nil
	}
	return &tools.ToolResult{Output: string(data)}, nil
}

func resolveDelegateTaskID(sessionID, taskID, directive string, kind workmodel.WorkKind) (string, error) {
	if id := strings.TrimSpace(taskID); id != "" {
		return id, nil
	}
	tm := globalDeps.Tasks
	if tm == nil || sessionID == "" {
		return "", nil
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
		fallback, ferr := tm.Create(sessionID, subject, directive)
		if ferr != nil {
			return "", fmt.Errorf("delegate: failed to create task (subject=%q): %w", subject, ferr)
		}
		if fallback == nil {
			return "", fmt.Errorf("delegate: failed to create task (subject=%q): nil task without error", subject)
		}
		return fallback.ID, nil
	}
	return item.ID, nil
}

func resolveSandboxSlug(fields map[string]string) string {
	if v := strings.TrimSpace(fields["sandbox_slug"]); v != "" {
		return v
	}
	return strings.TrimSpace(fields["worktree_slug"])
}

// parseSubAgentMode normalizes the tool-input `mode` string into a
// contracts.SubAgentMode. Unknown / empty values yield "" so SubTurnRunner
// falls back to Cfg.DefaultMode (DM-20260620-001-B / AC6).
func parseSubAgentMode(raw string) contracts.SubAgentMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(contracts.SubAgentModeBrief):
		return contracts.SubAgentModeBrief
	case string(contracts.SubAgentModeFork):
		return contracts.SubAgentModeFork
	case string(contracts.SubAgentModeFull):
		return contracts.SubAgentModeFull
	default:
		return ""
	}
}
