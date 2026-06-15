package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// WireD7 initializes the D7 SessionOrchestrator and wires it into the capture.
// D1 ingress requires a non-nil IOrchestrationEntry; returns error when d7.enabled=false.
//
// DM-020 (D7 Turn 编排上移): llmStack wires the D7→D3 LLMInvoker (A07).
// The TurnOrchestrator (A06) is assembled in slice c with the D2 adapter.
func WireD7(
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

	// DM-020 D-c: wire TurnOrchestrator as the QueryLoopExecutor.
	// This replaces the legacy d2Executor with D7's own turn loop that
	// calls D3 directly for LLM and D2 via拆面 adapters for tools/persist.
	d2a := newD2Adapter(gw, ctxEngine, llmStack.TokenCounter)
	llmInvoker := WireTurnInvoker(llmStack)
	turnOrch := turn.NewOrchestrator(turn.OrchestratorDeps{
		LLM:      llmInvoker,
		Context:  d2a,
		Tools:    d2a,
		Persist:  d2a,
		MaxTurns: 8,
	})
	executor := newTurnOrchExecutor(turnOrch)

	sink := newD1EventPublisher(gw)

	var obsBridge *observability.Bridge
	if b, ok := obsBridgeArg.(*observability.Bridge); ok {
		obsBridge = b
	}
	wm := coordinator.NewLocalWorkModel(workmodel.GlobalTaskManager)

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
// DM-020 D-c: this replaces d2Executor as the D7 FastPath executor.
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
