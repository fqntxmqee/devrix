package bootstrap

import (
	"fmt"
	"log/slog"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
	"github.com/devrix/devrix/internal/layers/multiagent/external"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
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
	coordCfg := loadOrchestratorConfigs(configFile)

	if !coordCfg.coordCfg.Enabled {
		return fmt.Errorf("d7: disabled (d7.enabled=false); D1 ingress requires orchestration entry")
	}

	slog.Info("d7: initializing SessionOrchestrator",
		"routing_mode", coordCfg.coordCfg.RoutingMode,
		"fast_path_threshold", coordCfg.coordCfg.FastPathThreshold,
		"command_first", coordCfg.coordCfg.CommandFirst,
	)

	routingMode := orchtypes.RoutingModeLoopFirst
	if coordCfg.coordCfg.RoutingMode == "rule_orchestrate" {
		routingMode = orchtypes.RoutingModeRuleOrchestrate
	}
	if routingMode == orchtypes.RoutingModeRuleOrchestrate {
		slog.Info("d7: routing_mode=rule_orchestrate enables FastPath confidence threshold gating to OrchestratePath",
			"change", "devrix-d2-queryloop-dismantle",
			"dm", "DM-20260618-010",
		)
	}
	coordinatorFileCfg := orchtypes.FileConfig{
		Enabled:           boolPtr(coordCfg.coordCfg.Enabled),
		RoutingMode:       strPtr(string(routingMode)),
		FastPathThreshold: intPtr(coordCfg.coordCfg.FastPathThreshold),
		CommandFirst:      boolPtr(coordCfg.coordCfg.CommandFirst),
	}
	coordinatorCfg := orchtypes.BuildConfig(&coordinatorFileCfg)

	obsBridge := resolveObsBridge(obsBridgeArg)

	// TaskManager constructed locally and DI'd to NewLocalWorkModel +
	// NewSessionOrchestrator via WithTaskManager.
	tm := workmodel.NewTaskManagerFromConfig(coordCfg.tasksCfg, obsBridge)
	// Registry created by bootstrap and DI'd to TaskManager.
	tm.SetRegistry(workmodel.NewRegistry("~/.devrix/runs"))
	tm.SetAdaptiveThreshold(&workmodel.AdaptiveThreshold{
		GlobalDefault: workmodel.DefaultUncertaintyDecomposeThreshold,
	})
	todoBackend := &workmodel.TodoWriteBackend{Manager: tm}
	tools.SetTodoSync(todoBackend.Sync)

	// DM-020 D-c: wire TurnOrchestrator as the TurnExecutor.
	// This replaces the legacy executor with the orchestration turn loop that
	// calls D3 directly for LLM and D2 via adapters for tools/persist.
	ctxAdapter := newContextEngineAdapter(gw, ctxEngine, llmStack.TokenCounter)
	llmInvoker := WireTurnInvoker(llmStack)
	loopFirst := coordinatorCfg.IsLoopFirst()

	sink := newGatewayEventPublisher(gw)

	wm := sessionorchestrator.NewLocalWorkModel(tm)
	if enforce.GlobalBackgroundRegistry == nil {
		enforce.SetGlobalBackgroundRegistry()
	}
	wm.SetBackgroundProvider(func(sessionID string) []sessionorchestrator.BackgroundLite {
		tasks := enforce.GlobalBackgroundRegistry.List(sessionID)
		if len(tasks) == 0 {
			return nil
		}
		out := make([]sessionorchestrator.BackgroundLite, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, sessionorchestrator.BackgroundLite{
				RunID:  t.ID,
				Status: mapBackgroundStatus(t.Status),
				Output: t.Result,
			})
		}
		return out
	})

	llmDecomp := WireDecisionPlanning(llmInvoker, llmStack.DefaultModel)

	orchPath := BuildOrchestratePath(sink, llmDecomp, WaveSchedulerDeps{
		GW:         gw,
		Engine:     ctxEngine,
		AgentTools: agentToolReg,
		ObsBridge:  obsBridge,
	})
	orchPath.SetTaskManager(tm)

	planMode := workmodel.NewPlanMode(newPlanLLMCompleter(llmInvoker, llmStack.DefaultModel), obsBridge)

	toolExec, turnOrch, subTurn := WireMUPSPipeline(MUPSPipelinesDeps{
		CtxAdapter:       ctxAdapter,
		OrchPath:         orchPath,
		LLMInvoker:       llmInvoker,
		DefaultModel:     llmStack.DefaultModel,
		LoopFirst:        loopFirst,
		ObsBridge:        obsBridge,
		PlanMode:         planMode,
		SubagentCfg:      coordCfg.subagentCfg,
		MaxContextTokens: coordCfg.maxContextTokens,
		FocusHint:        &workmodel.FocusHintProvider{Manager: tm},
		ResolveAwait:     &workmodel.ResolveAwaiter{Manager: tm},
		// DM-20260620-001 / AC1: oversized tool results (read_file / grep /
		// cat / etc.) are persisted to disk and replaced with a preview
		// marker so they do not blow up the LLM context budget.
		ToolResultStore: persist.NewToolResultStore(""),
	})
	setWiredSubTurn(subTurn)
	if ce := contextEngineFrom(ctxEngine); ce != nil {
		ce.SetPreparedTurnRunner(sessionorchestrator.NewPreparedTurnAdapter(turnOrch))
	}
	executor := newTurnOrchExecutor(turnOrch)

	itemRunner, pipelineLearner, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec: toolExec,
		Tasks:    tm,
	})
	if err != nil {
		return fmt.Errorf("d7: wire item pipeline: %w", err)
	}

	orch := sessionorchestrator.NewSessionOrchestrator(
		coordinatorCfg,
		executor,
		sessionorchestrator.WithSink(sink),
		sessionorchestrator.WithObservability(obsBridge),
		sessionorchestrator.WithWorkModel(wm),
		sessionorchestrator.WithOrchestratePath(orchPath),
		sessionorchestrator.WithTurnToolExecutor(toolExec),
		sessionorchestrator.WithTaskManager(tm),
		sessionorchestrator.WithItemPipelineRunner(itemRunner),
		sessionorchestrator.WithLearner(pipelineLearner),
	)

	entry := sessionorchestrator.NewEntry(orch)
	gw.SetOrchestrationEntry(entry)
	if exp, ok := ctxEngine.(contracts.ISessionSnapshotExporter); ok {
		gw.SetSessionSnapshotExporter(exp)
	}

	slog.Info("d7: SessionOrchestrator wired to gateway, D1→D7.ProcessMessage path active")
	slog.Info("d7: TurnOrchestrator wired (D7-S2-A06+A07)", "max_turns", turnOrch.MaxTurns())

	// v6.0.0 6 S 精简 (DM-20260626-001): wire hardening package-level bridge so
	// the 5 new P0/P1 Span ops (channel.route / memory.persist / system.anomaly_detect /
	// taskgraph.synthesize / executor.select) emit even though their call sites
	// live in sub-packages that don't hold a SessionOrchestrator reference.
	hardening.SetBridge(obsBridge)

	return nil
}

// orchestratorConfigs bundles the 4 orchestrator-level config values loaded
// from configFile (or their defaults). Extracted in DM-20260626-007 / PR-4
// to keep InitOrchestration readable.
type orchestratorConfigs struct {
	coordCfg         config.CoordinatorConfig
	tasksCfg         config.TasksConfig
	subagentCfg      config.SubagentConfig
	maxContextTokens int
}

// loadOrchestratorConfigs loads the 4 orchestrator configs from configFile.
// Silently falls back to defaults when configFile is empty or any load
// returns an error (preserves the pre-PR-4 behavior).
func loadOrchestratorConfigs(configFile string) *orchestratorConfigs {
	cfg := &orchestratorConfigs{
		coordCfg:         config.DefaultCoordinatorConfig(),
		tasksCfg:         config.DefaultTasksConfig(),
		subagentCfg:      config.DefaultSubagentConfig(),
		maxContextTokens: config.DefaultContextEngineConfig().MaxContextTokens,
	}
	if configFile == "" {
		return cfg
	}
	if fileCfg, err := config.LoadConfigFile(configFile); err == nil {
		cfg.coordCfg = config.BuildCoordinatorConfig(&fileCfg.Coordinator)
		if fileCfg.ContextEngine.Tasks.Mode != "" || fileCfg.ContextEngine.Tasks.StoreDir != "" {
			cfg.tasksCfg = fileCfg.ContextEngine.Tasks
		}
		if fileCfg.ContextEngine.MaxContextTokens > 0 {
			cfg.maxContextTokens = fileCfg.ContextEngine.MaxContextTokens
		}
	}
	if _, _, _, ctxFileCfg, err := config.LoadConfig(configFile); err == nil && ctxFileCfg != nil {
		cfg.tasksCfg = ctxFileCfg.Tasks
		cfg.subagentCfg = ctxFileCfg.Subagent.Normalized()
	}
	return cfg
}

// resolveObsBridge type-asserts arg to *observability.Bridge. Returns nil when
// the assertion fails (preserves the pre-PR-4 behavior).
func resolveObsBridge(arg interface{}) *observability.Bridge {
	if b, ok := arg.(*observability.Bridge); ok {
		return b
	}
	return nil
}
