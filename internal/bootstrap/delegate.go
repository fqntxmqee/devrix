package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/execute"
	"github.com/devrix/devrix/internal/layers/orchestration/delegatetools"
	"github.com/devrix/devrix/internal/layers/orchestration/hubspoke"
	"github.com/devrix/devrix/internal/shared/config"
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
func WireDelegate(
	ctxCfg *config.ContextEngineConfig,
	maCfg *config.MultiAgentConfig,
	gw *capture.CommunicationGateway,
	engine *contextengine.ContextEngine,
	toolReg contextengine.IToolRegistry,
) {
	if ctxCfg == nil || maCfg == nil || !maCfg.Delegate.Enabled {
		return
	}
	var subQuery hubspoke.SubQueryRunner
	if engine != nil && ctxCfg.QueryLoop.Enabled {
		subQuery = delegatetools.BuildSubQueryRunner(enforce.LoopDeps{
			Loop: engine.QueryLoop(),
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
		nil, // uses flow.GlobalHub by default
		gatewayLeaderResolver{gw: gw},
	)

	delegatetools.SetDeps(delegatetools.Deps{
		Dispatcher: disp,
		Leader:     gatewayLeaderResolver{gw: gw},
	})

	if reg, ok := toolReg.(*contextengine.ToolRegistry); ok {
		if err := delegatetools.RegisterTools(reg, maCfg); err != nil {
			slog.Error("register delegate tools", "error", err)
		}
	}
	slog.Info("d4 delegate enabled (hubspoke)",
		"allow_async", maCfg.Delegate.AllowAsync,
		"worktree", ctxCfg.Worktree.Enabled,
	)
}
