package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/bootstrap/sessionagents"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/sandbox"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/delegatetools"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/layers/orchestration/executionflow/bridge"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type gatewayLeaderResolver struct {
	agents *sessionagents.Manager
}

func (r gatewayLeaderResolver) Leader(sessionID string) (multiagent.Agent, bool) {
	if r.agents == nil {
		return nil, false
	}
	return r.agents.Leader(sessionID)
}

// WireDelegate wires D4 delegate execution and D7 dispatcher dispatch.
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
	agents *sessionagents.Manager,
	engine *contextengine.ContextEngine,
	toolReg contextengine.IToolRegistry,
	hub contracts.ExecutionFlowHub,
	tm *workmodel.TaskManager,
) {
	if ctxCfg == nil || maCfg == nil || !maCfg.Delegate.Enabled {
		return
	}
	var subQuery sessionorchestrator.SubQueryRunner
	if st := WiredSubTurn(); st != nil {
		var fr contracts.SubQueryFlowReporter
		if hub != nil {
			fr = bridge.NewFlowReporter(hub)
		}
		subQuery = delegatetools.BuildSubQueryRunner(enforce.SubQueryDeps{
			SubTurn:      st,
			FlowReporter: fr,
		})
	}
	var sb *sandbox.Manager
	if ctxCfg.Sandbox.Enabled {
		sb = sandbox.NewManager(ctxCfg.Sandbox)
	}

	exec := execute.NewExecutor(maCfg.Delegate, sb, nil)
	disp := sessionorchestrator.NewDispatcher(
		maCfg.Delegate,
		exec,
		subQuery,
		hub,
		gatewayLeaderResolver{agents: agents},
		tm.Registry(),
	)

	delegatetools.SetDeps(delegatetools.Deps{
		Dispatcher: disp,
		Leader:     gatewayLeaderResolver{agents: agents},
		Tasks:      tm,
	})

	if reg, ok := toolReg.(*tools.ToolRegistry); ok {
		if err := delegatetools.RegisterTools(reg, maCfg); err != nil {
			slog.Error("register delegate tools", "error", err)
		}
	}
	slog.Info("d4 delegate enabled",
		"allow_async", maCfg.Delegate.AllowAsync,
		"sandbox", ctxCfg.Sandbox.Enabled,
	)
}
