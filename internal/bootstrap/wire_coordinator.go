package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/communication/conclusion"
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
	"github.com/devrix/devrix/internal/shared/types"
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
		"command_first", coordCfg.coordCfg.CommandFirst,
	)

	// v2.6.0 (DM-20260629-001): RoutingModeRuleOrchestrate retired; legacy
	// "rule_orchestrate" YAML values are normalized to RoutingModeLoopFirst
	// (orchtypes.normalizeRoutingMode in config.go).
	coordinatorFileCfg := orchtypes.FileConfig{
		Enabled:            boolPtr(coordCfg.coordCfg.Enabled),
		RoutingMode:        strPtr(coordCfg.coordCfg.RoutingMode),
		CommandFirst:       boolPtr(coordCfg.coordCfg.CommandFirst),
		PriorContextRounds: intPtr(coordCfg.coordCfg.PriorContextRounds),
		SemanticConvergence: buildSemanticConvergenceFileConfig(coordCfg.coordCfg.SemanticConvergence),
		// DM-20260707-001 PR-D (T29): propagate dag_executor sub-config
		// from the YAML file so ops can flip the multi-intent DAG fork
		// without code changes. Default in cfg defaults is OFF; nil here
		// lets BuildDAGExecutorConfig preserve defaults.
		DAGExecutor: buildDAGExecutorFileConfig(coordCfg.coordCfg.DAGExecutor),
	}
	coordinatorCfg := orchtypes.BuildConfig(&coordinatorFileCfg)

	obsBridge := resolveObsBridge(obsBridgeArg)

	// TaskManager constructed locally and DI'd to NewLocalWorkModel +
	// NewSessionOrchestrator via WithTaskManager.
	tm := workmodel.NewTaskManagerFromConfig(coordCfg.tasksCfg, obsBridge)
	prevBeforeDispatch := gw.BeforeDispatch()
	gw.SetBeforeDispatch(func(ctx context.Context, session *types.Session) error {
		if session != nil {
			if wd := strings.TrimSpace(session.WorkDir); wd != "" {
				tm.SetSessionWorkDir(session.SessionID, wd)
			}
		}
		if prevBeforeDispatch != nil {
			return prevBeforeDispatch(ctx, session)
		}
		return nil
	})
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

	planMode := workmodel.NewPlanMode(newPlanLLMCompleter(llmInvoker, llmStack.DefaultModel), obsBridge)

	toolExec, turnOrch, subTurn := WireMUPSPipeline(MUPSPipelinesDeps{
		CtxAdapter:       ctxAdapter,
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
		PromptLanguage:  coordCfg.promptLanguage,
	})
	setWiredSubTurn(subTurn)
	if ce := contextEngineFrom(ctxEngine); ce != nil {
		ce.SetPreparedTurnRunner(sessionorchestrator.NewPreparedTurnAdapter(turnOrch))
	}

	itemRunner, pipelineLearner, err := WireItemPipeline(ItemPipelineWireDeps{
		ToolExec:         toolExec,
		Tasks:            tm,
		LLMInvoker:       llmInvoker,
		CtxPreparer:      ctxAdapter,
		PromptLanguage:   coordCfg.promptLanguage,
		// DM-20260706-006: wire the LLM-driven semantic verifier into
		// the per-WorkItem MUPS pipeline. Production default Enabled=true
		// (set in shared/config.DefaultCoordinatorConfig). The Jaccard
		// pre-check (MinSimilarity) gates the LLM call so the cost is
		// bounded to stagnation-suspect rounds only.
		SemanticConvergence: coordinatorCfg.SemanticConvergence,
		// DM-20260707-001 PR-D (T29): flip the multi-intent DAG fork gate
		// when ops has set dag_executor.enabled=true. Default false keeps
		// the legacy single-WorkItem path active.
		DAGEnabled:       coordinatorCfg.DAGExecutor.Enabled,
	})
	if err != nil {
		return fmt.Errorf("d7: wire item pipeline: %w", err)
	}

	orchOpts := []sessionorchestrator.OrchestratorOption{
		sessionorchestrator.WithSink(sink),
		sessionorchestrator.WithObservability(obsBridge),
		sessionorchestrator.WithWorkModel(wm),
		sessionorchestrator.WithTurnToolExecutor(toolExec),
		sessionorchestrator.WithTaskManager(tm),
		sessionorchestrator.WithItemPipelineRunner(itemRunner),
		sessionorchestrator.WithLearner(pipelineLearner),
	}
	// DM-20260628-003 (D7-S15): wire turn-state + transcript reader when
	// the deployment sets d7.prior_context_rounds > 0. The transcript
	// dir is resolved from coordCfg.tasksCfg.Diagnostics (matching the
	// writer's resolution in NewTranscriptWriter) so the reader and
	// writer point at the same jsonl files. WithPriorContextRounds
	// builds a fresh TurnState + TranscriptReader; WithTranscriptDir
	// overrides the default reader dir (applied AFTER
	// WithPriorContextRounds in this slice so it wins).
	if coordinatorCfg.PriorContextRounds > 0 {
		orchOpts = append(orchOpts,
			sessionorchestrator.WithPriorContextRounds(coordinatorCfg.PriorContextRounds),
		)
		transcriptDir := resolveTranscriptDir(coordCfg.transcriptDir)
		if transcriptDir != "" {
			orchOpts = append(orchOpts,
				sessionorchestrator.WithTranscriptDir(transcriptDir),
			)
		}
	}
	orch := sessionorchestrator.NewSessionOrchestrator(coordinatorCfg, nil, orchOpts...)

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
	hardening.SetLocatorAttrsProvider(workmodel.LocatorSpanAttrsFromContext)

	// DM-20260630-011 (devrix-session-conclusion-completeness): wire the
	// D1 conclusion package-level tracer so EmitComplete can emit the
	// D1_EmitComplete_Fallback span. Same package-level pattern as hardening
	// to avoid threading an obsBridge through every Emitter interface.
	if obsBridge != nil {
		conclusion.SetBridge(obsBridge.Tracer())
	}

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
	promptLanguage   string
	// transcriptDir (DM-20260628-003, D7-S15) is the resolved
	// context_engine.diagnostics.transcript_dir from configFile. Empty
	// when not configured — caller falls back to env / ~/.devrix.
	transcriptDir string
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
		cfg.transcriptDir = fileCfg.ContextEngine.Diagnostics.TranscriptDir
	}
	if _, _, _, ctxFileCfg, err := config.LoadConfig(configFile); err == nil && ctxFileCfg != nil {
		cfg.tasksCfg = ctxFileCfg.Tasks
		cfg.subagentCfg = ctxFileCfg.Subagent.Normalized()
		cfg.promptLanguage = ctxFileCfg.Workspace.Language
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

// resolveTranscriptDir (DM-20260628-003, D7-S15) mirrors the resolution
// in NewTranscriptWriter so the orchestrator's TranscriptReader points
// at the same dir the gateway's writer uses. Order:
//  1. fileCfg.ContextEngine.Diagnostics.TranscriptDir (already loaded
//     into coordCfg.transcriptDir by loadOrchestratorConfigs)
//  2. $DEVRIX_TRANSCRIPT_DIR
//  3. ~/.devrix/transcripts (or "" if home lookup fails)
//
// Returns "" when no source resolves — caller should treat that as
// "use default" and not pass WithTranscriptDir (so TranscriptReader
// falls back to its own default in sync with the writer).
func resolveTranscriptDir(fileTranscriptDir string) string {
	tdir := fileTranscriptDir
	if tdir == "" {
		tdir = os.Getenv("DEVRIX_TRANSCRIPT_DIR")
	}
	if tdir == "" {
		if home, herr := os.UserHomeDir(); herr == nil {
			tdir = filepath.Join(home, ".devrix", "transcripts")
		}
	}
	return tdir
}
