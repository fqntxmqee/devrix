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
// When d7.enabled=true, RouteInbound routes to D7.ProcessMessage instead of D2.Process.
func WireD7(
	configFile string,
	gw *capture.CommunicationGateway,
	ctxEngine contracts.IEngine,
	obsBridgeArg interface{},
) {
	// Load D7 configuration from config file.
	coordCfg := config.DefaultCoordinatorConfig()
	if configFile != "" {
		if fileCfg, err := config.LoadConfigFile(configFile); err == nil {
			coordCfg = config.BuildCoordinatorConfig(&fileCfg.Coordinator)
		}
	}

	if !coordCfg.Enabled {
		slog.Info("d7: disabled via config, D1→D2.Process legacy path active")
		return
	}

	slog.Info("d7: initializing SessionOrchestrator",
		"fast_path_threshold", coordCfg.FastPathThreshold,
		"command_first", coordCfg.CommandFirst,
		"plan_mode_approve_gate", coordCfg.PlanModeApproveGate,
	)

	// Build the coordinator Config from the shared config.
	coordinatorFileCfg := coordinator.FileConfig{
		Enabled:           boolPtr(coordCfg.Enabled),
		FastPathThreshold: intPtr(coordCfg.FastPathThreshold),
		CommandFirst:      boolPtr(coordCfg.CommandFirst),
		PlanModeApproveGate: boolPtr(coordCfg.PlanModeApproveGate),
	}
	coordinatorCfg := coordinator.BuildConfig(&coordinatorFileCfg)

	// Create the D2 executor adapter that bridges D7 QueryLoopExecutor to D2 IEngine.
	// D7 must NOT call contextengine internals; we go through contracts.IEngine interface.
	d2Executor := newD2Executor(gw, ctxEngine)

	// Create the event publisher that forwards events to the capture.
	// This implements coordinator.EventPublisher for D1 event delivery.
	sink := newD1EventPublisher(gw)

	// Build the SessionOrchestrator with the executor and sink.
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

	// Wrap in the Entry adapter that satisfies contracts.IOrchestrationEntry.
	entry := coordinator.NewEntry(orch)

	// Wire the Entry into the gateway — D1 RouteInbound will now route to D7.
	gw.SetOrchestrationEntry(entry, true)

	slog.Info("d7: SessionOrchestrator wired to gateway, D1→D7.ProcessMessage path active")
}

// d2Executor adapts D2 ContextEngine to the D7 QueryLoopExecutor interface.
// D7 calls RunQueryLoop; this adapter translates to D2's Process method.
type d2Executor struct {
	gw     *capture.CommunicationGateway
	engine contracts.IEngine
}

// newD2Executor creates an executor that bridges D7→D2 via contracts.IEngine.
func newD2Executor(gw *capture.CommunicationGateway, engine contracts.IEngine) *d2Executor {
	return &d2Executor{gw: gw, engine: engine}
}

// RunQueryLoop implements coordinator.QueryLoopExecutor.
// It adapts the D2 Process method to the QueryLoopExecutor interface.
func (e *d2Executor) RunQueryLoop(ctx context.Context, req coordinator.QueryRequest) (<-chan *contracts.EngineEvent, error) {
	if e.engine == nil {
		return nil, fmt.Errorf("d7 executor: context engine is nil")
	}
	if e.gw == nil {
		return nil, fmt.Errorf("d7 executor: gateway is nil")
	}

	// Get session from gateway to call D2 Process.
	session, err := e.gw.GetSession(req.SessionID)
	if err != nil {
		// Fall back: create a minimal session for the query.
		// D2's Process method will append this message to the session's history.
		session = types.NewSession(req.SessionID, "d7", "")
	}

	// Build the message from the query request.
	// If Messages is empty, use the SystemPrompt as the message.
	// D2.Process expects a single message string.
	var message string
	if len(req.Messages) > 0 {
		message = req.Messages[0].Content
	} else {
		message = ""
	}
	if req.SystemPrompt != "" {
		message = req.SystemPrompt + "\n" + message
	}

	return e.engine.Process(ctx, session, message), nil
}

// d1EventPublisher implements coordinator.EventPublisher by forwarding events to the capture.
type d1EventPublisher struct {
	gw *capture.CommunicationGateway
}

// newD1EventPublisher creates a publisher that forwards D7 events to the D1 capture.
func newD1EventPublisher(gw *capture.CommunicationGateway) *d1EventPublisher {
	return &d1EventPublisher{gw: gw}
}

// Publish implements coordinator.EventPublisher.
// It publishes the event through the gateway's event channel.
func (p *d1EventPublisher) Publish(ctx context.Context, ev *contracts.EngineEvent) {
	if p.gw == nil || ev == nil {
		return
	}
	p.gw.PublishEngineEvent(ev)
}

// boolPtr returns a pointer to a bool value (helper for config building).
func boolPtr(b bool) *bool {
	return &b
}

// intPtr returns a pointer to an int value (helper for config building).
func intPtr(i int) *int {
	return &i
}
