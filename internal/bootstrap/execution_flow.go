package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/layers/orchestration/imsink"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireExecutionFlow configures the global ExecutionFlowHub (Hub-Spoke v2).
func WireExecutionFlow(ctxCfg *config.ContextEngineConfig, gw *capture.CommunicationGateway, obsBridge *observability.Bridge) {
	if ctxCfg == nil {
		return
	}
	cfg := config.NormalizeExecutionFlowConfig(ctxCfg.ExecutionFlow)
	if !cfg.Enabled {
		flow.SetGlobalHub(nil)
		return
	}
	var im flow.IMSink
	if cfg.IMProgress && gw != nil {
		im = imsink.NewGatewaySink(gatewayEngineSink{gw: gw})
	}
	hub := flow.NewHub(flow.HubDeps{
		Config: cfg,
		Queue:  queue.GlobalSessionQueue,
		Tasks:  tasks.GlobalTaskManager,
		IM:     im,
	})
	flow.SetGlobalHub(hub)
	slog.Info("execution flow hub enabled",
		"link_tasks", cfg.LinkTasks,
		"im_progress", cfg.IMProgress,
	)
}

type gatewayEngineSink struct {
	gw *capture.CommunicationGateway
}

func (s gatewayEngineSink) Emit(ev *contracts.EngineEvent) {
	if s.gw == nil || ev == nil {
		return
	}
	s.gw.PublishEngineEvent(ev)
}
