package contextengine

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/devrix/devrix/internal/layers/contextengine/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// RegisterQueryLoopTools registers plan-mode and task tools when QueryLoop is enabled.
func RegisterQueryLoopTools(reg *ToolRegistry, cfg *config.ContextEngineConfig, manager *tasks.TaskManager) error {
	if reg == nil || cfg == nil {
		return nil
	}
	for _, runner := range []PluginRunner{
		newEnterPlanModeRunner(cfg.Permission),
		newExitPlanModeRunner(),
	} {
		if err := reg.Register(runner); err != nil {
			return err
		}
	}
	if cfg.Tasks.Mode != "v2" || manager == nil {
		return nil
	}
	for _, runner := range newTaskToolRunners(manager) {
		if err := reg.Register(runner); err != nil {
			return err
		}
	}
	return nil
}

type enterPlanModeRunner struct {
	cfg config.ContextPermissionConfig
}

func newEnterPlanModeRunner(cfg config.ContextPermissionConfig) *enterPlanModeRunner {
	return &enterPlanModeRunner{cfg: cfg}
}

func (r *enterPlanModeRunner) Name() string { return "enter_plan_mode" }

func (r *enterPlanModeRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "enter_plan_mode",
		Description: "Enter read-only plan mode to explore and draft a plan before implementation",
		Parameters:  `{"type":"object","properties":{"plan_file_path":{"type":"string"}}}`,
	}
}

func (r *enterPlanModeRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *enterPlanModeRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &ToolResult{Error: "enter_plan_mode: session context unavailable"}, nil
	}
	if sc.AgentID != "" {
		return &ToolResult{Error: "enter_plan_mode: not allowed from sub-agent context"}, nil
	}
	planPath := toolInputString(input, "plan_file_path")
	if planPath == "" {
		planPath = sc.PlanFilePath
	}
	if planPath == "" && r.cfg.Plan.PlanFileDir != "" {
		planPath = fmt.Sprintf("%s/%s.md", r.cfg.Plan.PlanFileDir, sc.SessionID)
	}
	permission.EnterPlan(sc, planPath)
	return &ToolResult{Output: fmt.Sprintf("Entered plan mode. Plan file: %s", sc.PlanFilePath)}, nil
}

type exitPlanModeRunner struct{}

func newExitPlanModeRunner() *exitPlanModeRunner { return &exitPlanModeRunner{} }

func (r *exitPlanModeRunner) Name() string { return "exit_plan_mode" }

func (r *exitPlanModeRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "exit_plan_mode",
		Description: "Exit plan mode and request approval to begin implementation",
		Parameters:  `{"type":"object","properties":{}}`,
	}
}

func (r *exitPlanModeRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *exitPlanModeRunner) Execute(ctx context.Context, _, _ string) (*ToolResult, error) {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &ToolResult{Error: "exit_plan_mode: session context unavailable"}, nil
	}
	prev := permission.ExitPlan(sc)
	return &ToolResult{Output: fmt.Sprintf("Exited plan mode (restored %s). Awaiting user approval.", prev)}, nil
}

func newTaskToolRunners(manager *tasks.TaskManager) []PluginRunner {
	return []PluginRunner{
		&taskToolRunner{name: tasks.ToolNameTaskCreate, manager: manager, action: taskActionCreate},
		&taskToolRunner{name: tasks.ToolNameTaskGet, manager: manager, action: taskActionGet},
		&taskToolRunner{name: tasks.ToolNameTaskList, manager: manager, action: taskActionList},
		&taskToolRunner{name: tasks.ToolNameTaskUpdate, manager: manager, action: taskActionUpdate},
	}
}

type taskAction int

const (
	taskActionCreate taskAction = iota
	taskActionGet
	taskActionList
	taskActionUpdate
)

type taskToolRunner struct {
	name    string
	manager *tasks.TaskManager
	action  taskAction
}

func (r *taskToolRunner) Name() string { return r.name }

func (r *taskToolRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *taskToolRunner) Schema() ToolSchema {
	switch r.action {
	case taskActionCreate:
		return ToolSchema{
			Name: r.name, Description: "Create a persisted task",
			Parameters: `{"type":"object","required":["subject"],"properties":{"subject":{"type":"string"},"description":{"type":"string"}}}`,
		}
	case taskActionGet:
		return ToolSchema{
			Name: r.name, Description: "Get a task by id",
			Parameters: `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string"}}}`,
		}
	case taskActionList:
		return ToolSchema{Name: r.name, Description: "List all tasks for the session", Parameters: `{"type":"object","properties":{}}`}
	default:
		return ToolSchema{
			Name: r.name, Description: "Update a task",
			Parameters: `{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string"},"status":{"type":"string"},"owner":{"type":"string"},"blocked_by":{"type":"string"}}}`,
		}
	}
}

func (r *taskToolRunner) Execute(ctx context.Context, _, input string) (*ToolResult, error) {
	sessionID := ToolSessionIDFromContext(ctx)
	if sessionID == "" {
		return &ToolResult{Error: r.name + ": session_id unavailable"}, nil
	}
	fields := parseToolInput(input)
	suite := tasks.NewToolSuite(r.manager)
	out, err := suite.Execute(ctx, tasks.TaskToolInput{
		SessionID:   sessionID,
		ToolName:    r.name,
		TaskID:      fields["task_id"],
		Subject:     fields["subject"],
		Description: fields["description"],
		Status:      fields["status"],
		Owner:       fields["owner"],
		BlockedBy:   fields["blocked_by"],
	})
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	if out == nil || !out.Success {
		msg := "task operation failed"
		if out != nil && out.Message != "" {
			msg = out.Message
		}
		return &ToolResult{Error: msg}, nil
	}
	data, _ := json.Marshal(out)
	return &ToolResult{Output: string(data)}, nil
}

func enforcePlanModeWrite(ctx context.Context, targetPath string) *ToolResult {
	sc := ToolSessionContextFromContext(ctx)
	if sc == nil || !sc.PermissionMode.IsPlanMode() {
		return nil
	}
	workDir := ToolWorkDirFromContext(ctx)
	resolved := targetPath
	if workDir != "" && !filepath.IsAbs(resolved) {
		resolved = filepath.Join(workDir, resolved)
	}
	if !permission.CanWritePath(
		sc.PermissionMode, sc.PlanFilePath, resolved, workDir, FilesAutoApprovedFromContext(ctx),
	) {
		return &ToolResult{
			Error: fmt.Sprintf(
				"plan mode: write denied for %s (allowed plan file: %s). %s",
				targetPath, sc.PlanFilePath, permission.PlanModeWriteHint(),
			),
		}
	}
	return nil
}
