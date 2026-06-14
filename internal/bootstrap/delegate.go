package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/nested"
	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/delegate"
	"github.com/devrix/devrix/internal/layers/orchestration/delegatetools"
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

// WireDelegate wires D4 delegate service and tools.
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
	var fallback delegate.SubQueryFallback
	if engine != nil && ctxCfg.QueryLoop.Enabled {
		fallback = delegatetools.BuildSubQueryFallback(nested.LoopDeps{
			Loop: engine.QueryLoop(),
		})
	}
	var wt *worktree.Manager
	if ctxCfg.Worktree.Enabled {
		wt = worktree.NewManager(ctxCfg.Worktree)
	}
	svc := delegate.NewService(maCfg.Delegate, fallback, wt, nil)
	delegatetools.SetDeps(delegatetools.Deps{
		Service: svc,
		Leader:  gatewayLeaderResolver{gw: gw},
	})
	if reg, ok := toolReg.(*contextengine.ToolRegistry); ok {
		if err := delegatetools.RegisterTools(reg, maCfg); err != nil {
			slog.Error("register delegate tools", "error", err)
		}
	}
	slog.Info("d4 delegate enabled",
		"allow_async", maCfg.Delegate.AllowAsync,
		"worktree", ctxCfg.Worktree.Enabled,
	)
}
