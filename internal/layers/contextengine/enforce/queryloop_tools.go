package enforce

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// RegisterQueryLoopTools registers plan-mode tools when QueryLoop is enabled.
// Task tools (task_*) are registered via workmodel.RegisterTaskTools in bootstrap.
func RegisterQueryLoopTools(reg *toolrunner.ToolRegistry, cfg *config.ContextEngineConfig) error {
	if reg == nil || cfg == nil {
		return nil
	}
	for _, runner := range []toolrunner.PluginRunner{
		newEnterPlanModeRunner(cfg.Permission),
		newExitPlanModeRunner(),
	} {
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

func (r *enterPlanModeRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name:        "enter_plan_mode",
		Description: "Enter read-only plan mode to explore and draft a plan before implementation",
		Parameters:  `{"type":"object","properties":{"plan_file_path":{"type":"string"}}}`,
	}
}

func (r *enterPlanModeRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *enterPlanModeRunner) Execute(ctx context.Context, workDir, input string) (*toolrunner.ToolResult, error) {
	sc := toolrunner.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &toolrunner.ToolResult{Error: "enter_plan_mode: session context unavailable"}, nil
	}
	if sc.AgentID != "" {
		return &toolrunner.ToolResult{Error: "enter_plan_mode: not allowed from sub-agent context"}, nil
	}
	planPath := toolrunner.ToolInputString(input, "plan_file_path")
	if planPath == "" {
		planPath = sc.PlanFilePath
	}
	if planPath == "" && r.cfg.Plan.PlanFileDir != "" {
		planPath = fmt.Sprintf("%s/%s.md", r.cfg.Plan.PlanFileDir, sc.SessionID)
	}
	permission.EnterPlan(sc, planPath)
	return &toolrunner.ToolResult{Output: fmt.Sprintf("Entered plan mode. Plan file: %s", sc.PlanFilePath)}, nil
}

type exitPlanModeRunner struct{}

func newExitPlanModeRunner() *exitPlanModeRunner { return &exitPlanModeRunner{} }

func (r *exitPlanModeRunner) Name() string { return "exit_plan_mode" }

func (r *exitPlanModeRunner) Schema() toolrunner.ToolSchema {
	return toolrunner.ToolSchema{
		Name:        "exit_plan_mode",
		Description: "Exit plan mode and request approval to begin implementation",
		Parameters:  `{"type":"object","properties":{}}`,
	}
}

func (r *exitPlanModeRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *exitPlanModeRunner) Execute(ctx context.Context, _, _ string) (*toolrunner.ToolResult, error) {
	sc := toolrunner.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &toolrunner.ToolResult{Error: "exit_plan_mode: session context unavailable"}, nil
	}
	prev := permission.ExitPlan(sc)
	return &toolrunner.ToolResult{Output: fmt.Sprintf("Exited plan mode (restored %s). Awaiting user approval.", prev)}, nil
}
