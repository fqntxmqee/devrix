package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	clidebug "github.com/devrix/devrix/internal/cli/debug"
	evalcli "github.com/devrix/devrix/internal/cli/eval"
	"github.com/devrix/devrix/internal/bootstrap"
	contextanalyze "github.com/devrix/devrix/internal/cli/context_analyze"
	doctorcli "github.com/devrix/devrix/internal/cli/doctor"
	toolcli "github.com/devrix/devrix/internal/cli/tool"
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/channel/adapters"
	"github.com/devrix/devrix/internal/layers/communication/channel/connection"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/communication/channel/instance"
	"github.com/devrix/devrix/internal/layers/communication/channel/metrics"
	"github.com/devrix/devrix/internal/layers/orchestration/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	asksurface "github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/layers/evolution/guard"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/layers/multiagent/provision/freefork"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/incident"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"

	// Spans self-registration (trigger init() to register domain spans)
	_ "github.com/devrix/devrix/internal/layers/communication"
	_ "github.com/devrix/devrix/internal/layers/orchestration/coordinator"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func main() {
	// DM-20260617-002 W5 (AC12): 在最早时机解析 --debug flag 并安装 filter，
	// 这样后续所有 slog 调用都受 categories 白名单过滤。
	debugCategories := bootstrap.ParseDebugFlag(os.Args[1:])

	// Go 1.26: replace default slog handler before any log call to avoid
	// circular log→slog→log deadlock in the runtime default handler.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if len(debugCategories) > 0 {
		bootstrap.InstallDebugFilter(debugCategories)
	}

	if len(os.Args) >= 2 && os.Args[1] == "debug" {
		if err := clidebug.Run(os.Args[2:]); err != nil {
			slog.Error("debug command failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "eval" {
		if err := evalcli.Run(os.Args[2:]); err != nil {
			slog.Error("eval command failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// DM-20260617-002 W9 (AC1): /doctor 自检 CLI 子命令, 不需要 LLM / obs 栈。
	if len(os.Args) >= 2 && os.Args[1] == "doctor" {
		if err := doctorcli.Run(os.Args[2:]); err != nil {
			slog.Error("doctor command failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// DM-20260617-002 W10 (AC2): /context analyze CLI 子命令, 不需要 LLM / obs 栈。
	if len(os.Args) >= 2 && (os.Args[1] == "context-analyze" || os.Args[1] == "context_analyze") {
		if err := contextanalyze.Run(os.Args[2:]); err != nil {
			slog.Error("context-analyze command failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// DM-20260617-007 W12 (AC12, AC13): /tool list 子命令, dump 当前
	// BuildSurfaces 输出的 tool schema 列表, 不需要 LLM stack / multi-agent。
	if len(os.Args) >= 2 && os.Args[1] == "tool" && len(os.Args) >= 3 && os.Args[2] == "list" {
		if err := toolcli.Run(os.Args[3:]); err != nil {
			slog.Error("tool list command failed", "error", err)
			os.Exit(1)
		}
		return
	}

	logBinaryInfo()

	configFile := config.FindConfigFile()
	if configFile != "" {
		slog.Info("loading config file", "path", configFile)
	}

	obsCfg := observability.DefaultConfig()
	if configFile != "" {
		if loaded, err := observability.LoadConfigFromFile(configFile); err != nil {
			slog.Warn("failed to load observability config, using defaults", "error", err)
		} else {
			obsCfg = loaded
		}
	}
	incident.ConfigureLLMLogging(incident.LLMLogSettings{
		LogContent: obsCfg.LLM.LogContent,
		LogDir:     obsCfg.LLM.LogDir,
	})

	obs, err := observability.New(obsCfg)
	if err != nil {
		slog.Warn("failed to initialize observability, continuing without", "error", err)
		obs = observability.NewNoOp()
	} else {
		observability.InstallSlogBridge()
		slog.Info("observability initialized",
			"tracing", obsCfg.IsTracingEnabled(),
			"exporter", obsCfg.Tracing.Exporter,
			"otlp_endpoint", obsCfg.Tracing.OTLP.Endpoint,
			"metrics", obsCfg.IsMetricsEnabled(),
			"logging", obsCfg.IsLoggingEnabled(),
		)
	}

	userCfg, err := config.LoadUserConfig()
	if err != nil {
		slog.Warn("failed to load user config, using defaults", "error", err)
		userCfg = config.DefaultUserConfig()
	}
	if userCfg.IsYOLOMode() {
		slog.Info("YOLO mode enabled - all permissions auto-approved")
	}

	commCfg, _, _, ctxCfg, err := config.LoadConfig(configFile)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	toolCfg, err := config.LoadToolConfig(configFile)
	if err != nil {
		slog.Error("failed to load tool config", "error", err)
		os.Exit(1)
	}
	multiAgentCfg, err := config.LoadMultiAgentConfig(configFile)
	if err != nil {
		slog.Error("failed to load multi-agent config", "error", err)
		os.Exit(1)
	}

	sessionStore, err := capture.NewFileSessionStore(commCfg.Session.StorageDir)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	connManager := connection.NewConnectionManager(60*time.Second, 10*time.Second)
	metricsCollector := metrics.NewMetricsCollector()
	instanceRegistry := instance.NewInstanceRegistry(60 * time.Second)
	milestoneService := milestone.NewMilestoneService(nil)

	permissionMgr := capture.NewPermissionManager(&commCfg.Permission)
	permissionMgr.SetUserConfig(userCfg)

	obsBridge := observability.NewBridge(obs)
	llmStack, err := llmbridge.WireContextLLM(configFile, userCfg.LLMGateway, obsBridge)
	if err != nil {
		if llmbridge.IsObservabilityRequiredError(err) {
			slog.Error("llm gateway wiring failed: observability bridge is required", "error", err)
		} else {
			slog.Error("llm gateway wiring failed", "error", err)
		}
		os.Exit(1)
	}
	llmbridge.LogLLMReadiness(configFile)
	if llmbridge.IsMockGateway(llmStack) {
		slog.Warn("llm gateway using mock — set MINIMAX_API_KEY and check devrix.yaml")
	}

	agentToolReg := bootstrap.WireAgentToolRegistry(configFile)
	engineMode := "context"
	if userCfg.IM.Enabled {
		engineMode = config.ResolveContextEngine(userCfg.IM)
	}

	// DM-20260617-008 W5: build factory + forker BEFORE the main engine so
	// the main engine's free_fork surface can be wired with the explicit
	// forker (replaces the legacy freefork.SetGlobalForker write inside
	// WireMultiAgent). The factory's shared engine is wired later via
	// factoryImpl.SetSharedEngine(contextEngine) once the main engine is
	// built.
	var (
		engineBuilder *bootstrap.ContextEngineBuilder
		agentFactory  multiagent.IAgentFactory
		forker        freefork.Forker
	)
	if multiAgentCfg.Enabled {
		engineBuilder = bootstrap.NewContextEngineBuilder(llmStack, ctxCfg, toolCfg, obsBridge, agentToolReg).
			WithMultiAgentConfig(multiAgentCfg)
		bootstrapFactory := bootstrap.WireAgentFactory(engineBuilder, multiAgentCfg, obsBridge, nil)
		forker = bootstrap.WireDefaultForker(bootstrapFactory)
		engineBuilder.WithForker(forker)
		agentFactory = bootstrapFactory
	}

	contextEngine := bootstrap.SelectContextEngine(
		engineMode,
		permissionMgr,
		ctxCfg,
		toolCfg,
		multiAgentCfg,
		obsBridge,
		llmStack,
		agentToolReg,
		forker,
	)

	defaultEventHandler := &DefaultEventHandler{
		connManager: connManager,
		metrics:     metricsCollector,
		obs:         obs,
	}

	imHosts, eventHandler := bootstrap.WireIM(userCfg, commCfg, defaultEventHandler)
	imActive := imHosts != nil && imHosts.Active()
	runCLI := !imActive || os.Getenv("DEVRIX_CLI") == "1"
	if runCLI && (imHosts == nil || !imActive) {
		eventHandler = bootstrap.NewCLIProgressHandler(defaultEventHandler, commCfg.CLI.ANSI)
	}

	// DM-20260617-002 S4-Gate H-1 fix: ctx 提前到 engine builder 之前, 让
	// startTrackerTick 等后台 goroutine 能在 shutdown 时干净退出 (cancel 传播)。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// DM-20260617-008 W1: transcript writer injected via ctor (no process-wide global).
	transcriptWriter := bootstrap.NewTranscriptWriter(ctxCfg)
	gw := capture.NewCommunicationGateway(sessionStore, eventHandler, permissionMgr, commCfg, transcriptWriter)
	gw.SetObservability(obs)

	// DM-20260618-006 (devrix-ask-user-question): wire the
	// ask_user_question surface's sender to the gateway's outbound path.
	// The surface itself is mounted by bootstrap.BuildSurfaces (see
	// internal/bootstrap/surfaces.go); this is the bridge that lets the
	// tool push a formatted question to the user's IM chat.
	asksurface.SetAskUserQuestionSender(func(ctx context.Context, sessionID, text string) error {
		return gw.RouteOutbound(&types.OutboundMessage{
			MessageID: "ask_" + sessionID + "_" + time.Now().UTC().Format("20060102T150405.000"),
			SessionID: sessionID,
			Content:   text,
			Role:      types.MessageRoleAssistant,
			Metadata: map[string]string{
				"source":   "ask_user_question",
				"blocking": "false",
			},
			SentAt: time.Now().UTC(),
		})
	})

	// DM-20260617-008 W4: TaskManager constructed once at startup and
	// shared with InitOrchestration (NewLocalWorkModel + WithTaskManager),
	// WireDelegate (delegatetools.SetDeps.Tasks), and NewCLIAdapter.
	// Replaces workmodel.GlobalTaskManager process-wide singleton.
	tm := workmodel.NewTaskManagerFromConfig(ctxCfg.Tasks, obsBridge)

	if multiAgentCfg.Enabled && agentFactory != nil {
		// DM-20260617-008 W5: now that the main engine exists, wire it into
		// the factory's deps.Engine so root session agents share the
		// gateway context engine and accumulate history.
		if factoryImpl, ok := agentFactory.(*multiagentprovision.AgentFactory); ok {
			factoryImpl.SetSharedEngine(contextEngine)
		}
		engineBuilder.WithContext(ctx)
		gw.SetAgentFactory(agentFactory)
		slog.Info("multi-agent layer enabled",
			"max_children", multiAgentCfg.MaxChildren,
			"max_total_agents", multiAgentCfg.MaxTotalAgents,
			"default_mode", multiAgentCfg.DefaultMode,
		)
	}

	if err := bootstrap.InitOrchestration(configFile, gw, contextEngine, obsBridge, llmStack, agentToolReg); err != nil {
		slog.Error("failed to init orchestration", "error", err)
		os.Exit(1)
	}

	// DM-20260618-010 follow-up: mirror the main engine's PreparedTurnRunner
	// onto the per-agent engine builder so forked workers (D4 ParentID != "")
	// can run Process() without "PreparedTurnRunner not wired" errors.
	if engineBuilder != nil {
		if ce, ok := contextEngine.(*contextengine.ContextEngine); ok {
			if runner := ce.PreparedTurnRunner(); runner != nil {
				engineBuilder.WithPreparedTurnRunner(runner)
			}
		}
	}

	initOrchestration(configFile, multiAgentCfg.Enabled, llmStack.RawGateway, gw, milestoneService, agentFactory, obs)

	hub, _ := bootstrap.WireExecutionFlow(ctxCfg, gw, obsBridge, tm)
	if ce, ok := contextEngine.(*contextengine.ContextEngine); ok {
		// DM-20260617-008 W4: shared TaskManager (see tm construction above).
		bootstrap.WireDelegate(ctxCfg, multiAgentCfg, gw, ce, ce.ToolRegistry(), hub, tm)
	}

	gw.StartCleanupRoutine(ctx, 30*time.Second)

	if err := bootstrap.StartIM(ctx, gw, imHosts); err != nil {
		slog.Error("failed to start IM adapter", "error", err)
		os.Exit(1)
	}

	instanceID := os.Getenv("DEVRIX_INSTANCE_ID")
	if instanceID == "" {
		instanceID = "devrix"
	}
	instanceName := os.Getenv("DEVRIX_INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "Devrix"
	}
	instanceInfo := &instance.InstanceInfo{
		ID:      instanceID,
		Name:    instanceName,
		Address: "localhost",
		Port:    0,
		Status:  "healthy",
	}
	if err := instanceRegistry.Register(ctx, instanceInfo); err != nil {
		slog.Warn("failed to register instance", "error", err)
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGHUP)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()

		select {
		case sig2 := <-sigCh:
			slog.Warn("force exit on repeated signal", "signal", sig2)
			os.Exit(130)
		case <-time.After(3 * time.Second):
			slog.Warn("shutdown timeout, force exit")
			os.Exit(130)
		}
	}()

	slog.Info("devrix started",
		"version", "v2.0",
		"engine", bootstrap.ContextEngineKind(contextEngine),
		"im_enabled", userCfg.IM.Enabled,
		"im_provider", userCfg.IM.Platform.Provider,
		"im_active", imActive,
		"cli", runCLI,
		"yolo_mode", userCfg.IsYOLOMode(),
	)

	if runCLI {
		// DM-20260617-008 W4: shared TaskManager (constructed above).
		cli := adapters.NewCLIAdapter(gw, commCfg, tm)
		if err := cli.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("cli exited with error", "error", err)
			os.Exit(1)
		}
	} else {
		<-ctx.Done()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := obs.Shutdown(shutdownCtx); err != nil {
		slog.Warn("observability shutdown error", "error", err)
	}
	connManager.Stop()
	instanceRegistry.Unregister(shutdownCtx, instanceInfo.ID)
	if imHosts != nil {
		imHosts.Stop()
	}

	slog.Info("devrix stopped")
}

// initOrchestration wires cross-model decision validation.
func initOrchestration(
	configFile string,
	multiAgentEnabled bool,
	rawGateway llmgateway.IGateway,
	gw *capture.CommunicationGateway,
	milestoneSvc *milestone.MilestoneService,
	agentFactory multiagent.IAgentFactory,
	obs *observability.Observability,
) {
	if !multiAgentEnabled || rawGateway == nil {
		return
	}

	orchCfg := config.DefaultOrchestrationConfig()
	if configFile != "" {
		if fileCfg, err := config.LoadConfigFile(configFile); err == nil {
			orchCfg = *config.BuildOrchestrationConfig(&fileCfg.Orchestration)
		}
	}
	if !orchCfg.Enabled {
		return
	}

	runtimeJudge := guard.NewRuntimeJudge(rawGateway, orchCfg)
	executor := guard.NewInterventionExecutor(gw, milestoneSvc, agentFactory)
	guardValidator := guard.NewRuntimeGuardValidator(orchCfg, runtimeJudge, executor)
	guardValidator.SetObservability(obs)
	gw.SetAgentObserverFactory(func(ctx context.Context, session *types.Session) guard.AgentObserver {
		return guard.NewGuardObserver(guardValidator, ctx, session)
	})
	slog.Info("guard validator enabled",
		"judge_provider", orchCfg.JudgeProvider,
		"auto_intervene", orchCfg.AutoIntervene,
	)
}

func logBinaryInfo() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	info, err := os.Stat(exe)
	if err != nil {
		slog.Info("devrix binary", "path", exe)
		return
	}
	slog.Info("devrix binary",
		"path", exe,
		"mod_time", info.ModTime().Format(time.RFC3339),
		"size_bytes", info.Size(),
	)
}

// DefaultEventHandler handles gateway events when no IM adapter is active.
type DefaultEventHandler struct {
	connManager *connection.ConnectionManager
	metrics     *metrics.MetricsCollector
	obs         *observability.Observability
}

func (h *DefaultEventHandler) OnMessage(msg *types.OutboundMessage) {
	slog.Debug("message", "sessionID", msg.SessionID, "content", msg.Content)
}

func (h *DefaultEventHandler) OnPermissionRequest(req *types.PermissionRequest) bool {
	slog.Info("permission request", "tool", req.ToolName, "risk", req.RiskLevel)
	return true
}

func (h *DefaultEventHandler) OnError(err error, sessionID string) {
	slog.Error("session error", "sessionID", sessionID, "error", err)
}

func (h *DefaultEventHandler) OnStatus(sessionID string, state types.SessionState) {
	slog.Debug("session status", "sessionID", sessionID, "state", state)
}
