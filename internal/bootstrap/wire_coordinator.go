package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/nested"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// InitOrchestration initializes the SessionOrchestrator and wires it into the capture.
// D1 ingress requires a non-nil IOrchestrationEntry; returns error when d7.enabled=false.
//
// DM-020 (D7 Turn 编排上移): llmStack wires the D7→D3 LLMInvoker (A07).
// The TurnOrchestrator (A06) is assembled in slice c with the context engine adapter.
func InitOrchestration(
	configFile string,
	gw *capture.CommunicationGateway,
	ctxEngine contracts.IEngine,
	obsBridgeArg interface{},
	llmStack llmbridge.ContextLLMStack,
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

	var obsBridge *observability.Bridge
	if b, ok := obsBridgeArg.(*observability.Bridge); ok {
		obsBridge = b
	}

	// DM-020 D-c: wire TurnOrchestrator as the QueryLoopExecutor.
	// This replaces the legacy executor with the orchestration turn loop that
	// calls D3 directly for LLM and D2 via adapters for tools/persist.
	ctxAdapter := newContextEngineAdapter(gw, ctxEngine, llmStack.TokenCounter)
	llmInvoker := WireTurnInvoker(llmStack)
	turnOrch := turn.NewOrchestrator(turn.OrchestratorDeps{
		LLM:       llmInvoker,
		Context:   ctxAdapter,
		Tools:     ctxAdapter,
		Persist:   ctxAdapter,
		MaxTurns:  8,
		ObsBridge: obsBridge,
	})
	executor := newTurnOrchExecutor(turnOrch)

	sink := newGatewayEventPublisher(gw)

	wm := coordinator.NewLocalWorkModel(workmodel.GlobalTaskManager)
	if nested.GlobalBackgroundRegistry == nil {
		nested.SetGlobalBackgroundRegistry()
	}
	wm.SetBackgroundProvider(func(sessionID string) []coordinator.BackgroundLite {
		tasks := nested.GlobalBackgroundRegistry.List(sessionID)
		if len(tasks) == 0 {
			return nil
		}
		out := make([]coordinator.BackgroundLite, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, coordinator.BackgroundLite{
				RunID:  t.ID,
				Status: mapBackgroundStatus(t.Status),
				Output: t.Result,
			})
		}
		return out
	})

	// DM-20260615-005 / D7-S5-A03: wire the LLM-augmented task
	// synthesizer into the default OrchestratePath. Uses the same
	// GatewayInvoker as the leader path; on parse/timeout failure the
	// rule-based decomposeGoal fallback runs.
	llmDecomp := coordinator.NewLLMDecomposer(coordinator.LLMDecomposerDeps{
		LLM:         llmInvoker,
		DefaultTier: llmStack.DefaultModel,
	})

	orch := coordinator.NewSessionOrchestrator(
		coordinatorCfg,
		executor,
		coordinator.WithSink(sink),
		coordinator.WithObservability(obsBridge),
		coordinator.WithWorkModel(wm),
		coordinator.WithLLMDecomposer(llmDecomp),
	)

	entry := coordinator.NewEntry(orch)
	gw.SetOrchestrationEntry(entry)
	if exp, ok := ctxEngine.(contracts.ISessionSnapshotExporter); ok {
		gw.SetSessionSnapshotExporter(exp)
	}

	slog.Info("d7: SessionOrchestrator wired to gateway, D1→D7.ProcessMessage path active")
	slog.Info("d7: TurnOrchestrator wired (D7-S2-A06+A07)", "max_turns", 8)
	return nil
}

// turnOrchExecutor adapts turn.TurnOrchestrator to coordinator.QueryLoopExecutor.
// DM-020 D-c: this replaces the legacy executor as the FastPath executor.
type turnOrchExecutor struct {
	orch turn.TurnOrchestrator
}

func newTurnOrchExecutor(orch turn.TurnOrchestrator) *turnOrchExecutor {
	return &turnOrchExecutor{orch: orch}
}

func (e *turnOrchExecutor) RunQueryLoop(ctx context.Context, req coordinator.QueryRequest) (<-chan *contracts.EngineEvent, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("turn executor: at least one message required")
	}
	return e.orch.RunTurn(ctx, turn.TurnRequest{
		SessionID:   req.SessionID,
		UserMessage: req.Messages[0],
		MaxTurns:    req.MaxTurns,
		Scope:       turn.TurnScopeMain,
	})
}

type gatewayEventPublisher struct {
	gw *capture.CommunicationGateway
}

func newGatewayEventPublisher(gw *capture.CommunicationGateway) *gatewayEventPublisher {
	return &gatewayEventPublisher{gw: gw}
}

func (p *gatewayEventPublisher) Publish(ctx context.Context, ev *contracts.EngineEvent) {
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

// mapBackgroundStatus converts a BackgroundRegistry status string to a
// coordinator TaskStatus. BackgroundRegistry uses "running" while the work
// model uses "in_progress"; all other values ("completed", "failed",
// "cancelled") match directly.
func mapBackgroundStatus(s string) coordinator.TaskStatus {
	if s == "running" {
		return coordinator.TaskStatusInProgress
	}
	return coordinator.TaskStatus(s)
}
