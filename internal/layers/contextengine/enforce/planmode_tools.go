package enforce

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// RegisterPlanModeTools registers enter/exit plan mode tools.
// Task tools (task_*) are registered via workmodel.RegisterTaskTools in bootstrap.
func RegisterPlanModeTools(reg *tools.ToolRegistry, cfg *config.ContextEngineConfig) error {
	if reg == nil || cfg == nil {
		return nil
	}
	for _, runner := range []tools.PluginRunner{
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

func (r *enterPlanModeRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "enter_plan_mode",
		Description: "Enter read-only plan mode to explore and draft a plan before implementation",
		Parameters:  `{"type":"object","properties":{"plan_file_path":{"type":"string"}}}`,
	}
}

func (r *enterPlanModeRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *enterPlanModeRunner) Execute(ctx context.Context, workDir, input string) (*tools.ToolResult, error) {
	sc := tools.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &tools.ToolResult{Error: "enter_plan_mode: session context unavailable"}, nil
	}
	if sc.AgentID != "" {
		return &tools.ToolResult{Error: "enter_plan_mode: not allowed from sub-agent context"}, nil
	}
	planPath := tools.ToolInputString(input, "plan_file_path")
	if planPath == "" {
		planPath = sc.PlanFilePath
	}
	if planPath == "" && r.cfg.Plan.PlanFileDir != "" {
		planPath = fmt.Sprintf("%s/%s.md", r.cfg.Plan.PlanFileDir, sc.SessionID)
	}
	permission.EnterPlan(sc, planPath)
	return &tools.ToolResult{Output: fmt.Sprintf("Entered plan mode. Plan file: %s", sc.PlanFilePath)}, nil
}

type exitPlanModeRunner struct{}

func newExitPlanModeRunner() *exitPlanModeRunner { return &exitPlanModeRunner{} }

func (r *exitPlanModeRunner) Name() string { return "exit_plan_mode" }

func (r *exitPlanModeRunner) Schema() tools.ToolSchema {
	return tools.ToolSchema{
		Name:        "exit_plan_mode",
		Description: "Exit plan mode and request approval to begin implementation",
		Parameters:  `{"type":"object","properties":{}}`,
	}
}

func (r *exitPlanModeRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *exitPlanModeRunner) Execute(ctx context.Context, _, _ string) (*tools.ToolResult, error) {
	sc := tools.ToolSessionContextFromContext(ctx)
	if sc == nil {
		return &tools.ToolResult{Error: "exit_plan_mode: session context unavailable"}, nil
	}
	prev := permission.ExitPlan(sc)
	return &tools.ToolResult{Output: fmt.Sprintf("Exited plan mode (restored %s). Awaiting user approval.", prev)}, nil
}
