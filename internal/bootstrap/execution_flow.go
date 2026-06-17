package bootstrap

import (
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/flow"
	"github.com/devrix/devrix/internal/layers/orchestration/imsink"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionqueue"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireExecutionFlow constructs the ExecutionFlowHub and returns it.
//
// DM-20260617-008 W2: returns the hub to the caller instead of writing it
// to flow.GlobalHub (the process-wide global has been removed). Callers
// pass the hub to downstream wiring (e.g. WireDelegate).
func WireExecutionFlow(
	ctxCfg *config.ContextEngineConfig,
	gw *capture.CommunicationGateway,
	obsBridge *observability.Bridge,
) contracts.ExecutionFlowHub {
	if ctxCfg == nil {
		return contracts.NoOpExecutionFlowHub{}
	}
	cfg := config.NormalizeExecutionFlowConfig(ctxCfg.ExecutionFlow)
	if !cfg.Enabled {
		return contracts.NoOpExecutionFlowHub{}
	}
	var im flow.IMSink
	if cfg.IMProgress && gw != nil {
		im = imsink.NewGatewaySink(gatewayEngineSink{gw: gw})
	}
	hub := flow.NewHub(flow.HubDeps{
		Config: cfg,
		Queue:  sessionqueue.NewSessionQueue(),
		Tasks:  workmodel.NewTaskManagerFromConfig(ctxCfg.Tasks, obsBridge),
		IM:     im,
	})
	slog.Info("execution flow hub enabled",
		"link_tasks", cfg.LinkTasks,
		"im_progress", cfg.IMProgress,
	)
	return hub
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
