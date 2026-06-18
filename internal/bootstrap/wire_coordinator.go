package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/orchestration/runregistry"
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
	agentToolReg *external.Registry,
) error {
	coordCfg := config.DefaultCoordinatorConfig()
	tasksCfg := config.DefaultTasksConfig()
	if configFile != "" {
		if fileCfg, err := config.LoadConfigFile(configFile); err == nil {
			coordCfg = config.BuildCoordinatorConfig(&fileCfg.Coordinator)
			if fileCfg.ContextEngine.Tasks.Mode != "" || fileCfg.ContextEngine.Tasks.StoreDir != "" {
				tasksCfg = fileCfg.ContextEngine.Tasks
			}
		}
		if _, _, _, ctxFileCfg, err := config.LoadConfig(configFile); err == nil && ctxFileCfg != nil {
			tasksCfg = ctxFileCfg.Tasks
		}
	}

	if !coordCfg.Enabled {
		return fmt.Errorf("d7: disabled (d7.enabled=false); D1 ingress requires orchestration entry")
	}

	slog.Info("d7: initializing SessionOrchestrator",
		"routing_mode", coordCfg.RoutingMode,
		"fast_path_threshold", coordCfg.FastPathThreshold,
		"command_first", coordCfg.CommandFirst,
	)

	routingMode := coordinator.RoutingModeLoopFirst
	if coordCfg.RoutingMode == "rule_orchestrate" {
		routingMode = coordinator.RoutingModeRuleOrchestrate
	}
	// DM-20260617-001: when the operator opts into rule_orchestrate
	// (legacy routing) the boot log carries a deprecation warning so
	// the choice is visible without scraping metrics. The corresponding
	// per-Run() warning lives in `query.Loop.Run` and is gated by
	// sync.Once.
	if routingMode == coordinator.RoutingModeRuleOrchestrate {
		slog.Warn("d7: routing_mode=rule_orchestrate is DEPRECATED; ingress will reach D2.QueryLoop.Run and bump d2_query_loop_legacy_invocations_total. Set routing_mode=loop_first (default) to silence this warning.",
			"change", "devrix-queryloop-legacy-decommission",
			"dm", "DM-20260617-001",
			"canonical_path", "D7-S2-A06 RunTurnLoop",
		)
	}
	coordinatorFileCfg := coordinator.FileConfig{
		Enabled:           boolPtr(coordCfg.Enabled),
		RoutingMode:       strPtr(string(routingMode)),
		FastPathThreshold: intPtr(coordCfg.FastPathThreshold),
		CommandFirst:      boolPtr(coordCfg.CommandFirst),
	}
	coordinatorCfg := coordinator.BuildConfig(&coordinatorFileCfg)

	maxContextTokens := config.DefaultContextEngineConfig().MaxContextTokens
	if configFile != "" {
		if fileCfg, err := config.LoadConfigFile(configFile); err == nil && fileCfg.ContextEngine.MaxContextTokens > 0 {
			maxContextTokens = fileCfg.ContextEngine.MaxContextTokens
		}
	}

	var obsBridge *observability.Bridge
	if b, ok := obsBridgeArg.(*observability.Bridge); ok {
		obsBridge = b
	}

	// DM-20260617-008 W4: TaskManager constructed locally and shared with
	// NewLocalWorkModel + NewSessionOrchestrator (via WithTaskManager).
	// Replaces workmodel.GlobalTaskManager process-wide singleton.
	tm := workmodel.NewTaskManagerFromConfig(tasksCfg, obsBridge)
	runregistry.SetGlobal(runregistry.NewRegistry("~/.devrix/runs"))
	todoBackend := &workmodel.TodoWriteBackend{Manager: tm}
	toolrunner.SetTodoSync(todoBackend.Sync)

	// DM-020 D-c: wire TurnOrchestrator as the QueryLoopExecutor.
	// This replaces the legacy executor with the orchestration turn loop that
	// calls D3 directly for LLM and D2 via adapters for tools/persist.
	ctxAdapter := newContextEngineAdapter(gw, ctxEngine, llmStack.TokenCounter)
	llmInvoker := WireTurnInvoker(llmStack)
	loopFirst := coordinatorCfg.IsLoopFirst()

	sink := newGatewayEventPublisher(gw)

	wm := coordinator.NewLocalWorkModel(tm)
	if enforce.GlobalBackgroundRegistry == nil {
		enforce.SetGlobalBackgroundRegistry()
	}
	wm.SetBackgroundProvider(func(sessionID string) []coordinator.BackgroundLite {
		tasks := enforce.GlobalBackgroundRegistry.List(sessionID)
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

	llmDecomp := coordinator.NewLLMDecomposer(coordinator.LLMDecomposerDeps{
		LLM:         llmInvoker,
		DefaultTier: llmStack.DefaultModel,
	})

	orchPath := BuildOrchestratePath(sink, llmDecomp, WaveSchedulerDeps{
		GW:         gw,
		Engine:     ctxEngine,
		AgentTools: agentToolReg,
		ObsBridge:  obsBridge,
	})
	orchPath.SetTaskManager(tm)

	planMode := workmodel.NewPlanMode(newPlanLLMCompleter(llmInvoker, llmStack.DefaultModel), obsBridge)
	toolExec := coordinator.NewTurnToolExecutor(ctxAdapter, orchPath, planMode, loopFirst)
	if obsBridge != nil {
		toolExec.SetTurnToolMetrics(coordinator.NewTurnToolMetrics(obsBridge.Meter()))
	}

	ctxPrep := &coordinator.TurnPrepareWrapper{Inner: ctxAdapter, LoopFirst: loopFirst}

	turnOrch := turn.NewOrchestrator(turn.OrchestratorDeps{
		LLM:              llmInvoker,
		Context:          ctxPrep,
		Tools:            toolExec,
		Persist:          ctxAdapter,
		MaxTurns:         8,
		DefaultModel:     llmStack.DefaultModel,
		MaxContextTokens: maxContextTokens,
		ObsBridge:        obsBridge,
		FocusHint:        &workmodel.FocusHintProvider{Manager: tm},
	})
	executor := newTurnOrchExecutor(turnOrch)

	orch := coordinator.NewSessionOrchestrator(
		coordinatorCfg,
		executor,
		coordinator.WithSink(sink),
		coordinator.WithObservability(obsBridge),
		coordinator.WithWorkModel(wm),
		coordinator.WithOrchestratePath(orchPath),
		coordinator.WithTurnToolExecutor(toolExec),
		coordinator.WithTaskManager(tm),
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
		SessionID:    req.SessionID,
		UserMessage:  req.Messages[0],
		SystemPrompt: req.SystemPrompt,
		MaxTurns:     req.MaxTurns,
		Scope:        turn.TurnScopeMain,
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

func strPtr(s string) *string {
	return &s
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
