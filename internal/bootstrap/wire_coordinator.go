package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// WireD7 initializes the D7 SessionOrchestrator and wires it into the capture.
// D1 ingress requires a non-nil IOrchestrationEntry; returns error when d7.enabled=false.
func WireD7(
	configFile string,
	gw *capture.CommunicationGateway,
	ctxEngine contracts.IEngine,
	obsBridgeArg interface{},
) error {
	coordCfg := config.DefaultCoordinatorConfig()
	if configFile != "" {
		if fileCfg, err := config.LoadConfigFile(configFile); err == nil {
			coordCfg = config.BuildCoordinatorConfig(&fileCfg.Coordinator)
		}
	}

	if !coordCfg.Enabled {
		return fmt.Errorf("d7: disabled (d7.enabled=false); D1 ingress requires orchestration entry")
	}

	slog.Info("d7: initializing SessionOrchestrator",
		"fast_path_threshold", coordCfg.FastPathThreshold,
		"command_first", coordCfg.CommandFirst,
		"plan_mode_approve_gate", coordCfg.PlanModeApproveGate,
	)

	coordinatorFileCfg := coordinator.FileConfig{
		Enabled:             boolPtr(coordCfg.Enabled),
		FastPathThreshold:   intPtr(coordCfg.FastPathThreshold),
		CommandFirst:        boolPtr(coordCfg.CommandFirst),
		PlanModeApproveGate: boolPtr(coordCfg.PlanModeApproveGate),
	}
	coordinatorCfg := coordinator.BuildConfig(&coordinatorFileCfg)

	d2Executor := newD2Executor(gw, ctxEngine)
	sink := newD1EventPublisher(gw)

	var obsBridge *observability.Bridge
	if b, ok := obsBridgeArg.(*observability.Bridge); ok {
		obsBridge = b
	}
	orch := coordinator.NewSessionOrchestrator(
		coordinatorCfg,
		d2Executor,
		coordinator.WithSink(sink),
		coordinator.WithObservability(obsBridge),
	)

	entry := coordinator.NewEntry(orch)
	gw.SetOrchestrationEntry(entry)
	if exp, ok := ctxEngine.(contracts.ISessionSnapshotExporter); ok {
		gw.SetSessionSnapshotExporter(exp)
	}

	slog.Info("d7: SessionOrchestrator wired to gateway, D1→D7.ProcessMessage path active")
	return nil
}

// d2Executor adapts D2 ContextEngine to the D7 QueryLoopExecutor interface.
type d2Executor struct {
	gw     *capture.CommunicationGateway
	engine contracts.IEngine
}

func newD2Executor(gw *capture.CommunicationGateway, engine contracts.IEngine) *d2Executor {
	return &d2Executor{gw: gw, engine: engine}
}

func (e *d2Executor) RunQueryLoop(ctx context.Context, req coordinator.QueryRequest) (<-chan *contracts.EngineEvent, error) {
	if e.engine == nil {
		return nil, fmt.Errorf("d7 executor: context engine is nil")
	}
	if e.gw == nil {
		return nil, fmt.Errorf("d7 executor: gateway is nil")
	}

	session, err := e.gw.GetSession(req.SessionID)
	if err != nil {
		session = types.NewSession(req.SessionID, "d7", "")
	}

	var message string
	if len(req.Messages) > 0 {
		message = req.Messages[0].Content
	}
	if req.SystemPrompt != "" {
		message = req.SystemPrompt + "\n" + message
	}

	return e.engine.Process(ctx, session, message), nil
}

type d1EventPublisher struct {
	gw *capture.CommunicationGateway
}

func newD1EventPublisher(gw *capture.CommunicationGateway) *d1EventPublisher {
	return &d1EventPublisher{gw: gw}
}

func (p *d1EventPublisher) Publish(ctx context.Context, ev *contracts.EngineEvent) {
	if p.gw == nil || ev == nil {
		return
	}
	p.gw.PublishEngineEvent(ev)
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}
