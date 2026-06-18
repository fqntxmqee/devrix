package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/delegatetools"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type gatewayLeaderResolver struct {
	gw *capture.CommunicationGateway
}

func (r gatewayLeaderResolver) Leader(sessionID string) (multiagent.Agent, bool) {
	if r.gw == nil {
		return nil, false
	}
	ag := r.gw.SessionAgent(sessionID)
	return ag, ag != nil
}

// WireDelegate wires D4 delegate execution and D7 hubspoke dispatch.
//
// DM-20260617-008 W2: accepts the ExecutionFlowHub explicitly (caller
// receives it from WireExecutionFlow). No longer reads flow.GlobalHub.
//
// DM-20260617-008 W4: accepts the *workmodel.TaskManager explicitly for
// delegatetools.SetDeps. Caller (cmd/devrix/main.go) constructs tm once and
// shares it with InitOrchestration + WireDelegate + NewCLIAdapter. Pass nil
// if /task is not expected (delegate_explore/plan/implement will skip the
// auto-create TaskManager path).
func WireDelegate(
	ctxCfg *config.ContextEngineConfig,
	maCfg *config.MultiAgentConfig,
	gw *capture.CommunicationGateway,
	engine *contextengine.ContextEngine,
	toolReg contextengine.IToolRegistry,
	hub contracts.ExecutionFlowHub,
	tm *workmodel.TaskManager,
) {
	if ctxCfg == nil || maCfg == nil || !maCfg.Delegate.Enabled {
		return
	}
	var subQuery hubspoke.SubQueryRunner
	if engine != nil && ctxCfg.QueryLoop.Enabled {
		var fr contracts.SubQueryFlowReporter
		if hub != nil {
			fr = hubspoke.NewFlowReporter(hub)
		}
		subQuery = delegatetools.BuildSubQueryRunner(enforce.LoopDeps{
			Loop:         engine.QueryLoop(),
			FlowReporter: fr,
		})
	}
	var wt *worktree.Manager
	if ctxCfg.Worktree.Enabled {
		wt = worktree.NewManager(ctxCfg.Worktree)
	}

	exec := execute.NewExecutor(maCfg.Delegate, wt, nil)
	disp := hubspoke.NewDispatcher(
		maCfg.Delegate,
		exec,
		subQuery,
		hub,
		gatewayLeaderResolver{gw: gw},
	)

	delegatetools.SetDeps(delegatetools.Deps{
		Dispatcher: disp,
		Leader:     gatewayLeaderResolver{gw: gw},
		Tasks:      tm,
	})

	if reg, ok := toolReg.(*toolrunner.ToolRegistry); ok {
		if err := delegatetools.RegisterTools(reg, maCfg); err != nil {
			slog.Error("register delegate tools", "error", err)
		}
		if ctxCfg != nil {
			if err := workmodel.RegisterUnifiedTaskTools(reg, ctxCfg, tm); err != nil {
				slog.Error("register unified task tools", "error", err)
			}
			workmodel.SetUnifiedToolRegistry(reg)
		}
	}
	slog.Info("d4 delegate enabled (hubspoke)",
		"allow_async", maCfg.Delegate.AllowAsync,
		"worktree", ctxCfg.Worktree.Enabled,
	)
}
