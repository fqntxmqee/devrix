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
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/adapters"
	"github.com/devrix/devrix/internal/layers/communication/connection"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/instance"
	"github.com/devrix/devrix/internal/layers/communication/metrics"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/evolution/orchestration"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func main() {
	// Go 1.26: replace default slog handler before any log call to avoid
	// circular log→slog→log deadlock in the runtime default handler.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

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
	observability.ConfigureLLMLogging(observability.LLMLogSettings{
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

	sessionStore, err := gateway.NewFileSessionStore(commCfg.Session.StorageDir)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	connManager := connection.NewConnectionManager(60*time.Second, 10*time.Second)
	metricsCollector := metrics.NewMetricsCollector()
	instanceRegistry := instance.NewInstanceRegistry(60 * time.Second)
	milestoneService := milestone.NewMilestoneService(nil)

	permissionMgr := gateway.NewPermissionManager(&commCfg.Permission)
	permissionMgr.SetUserConfig(userCfg)

	obsBridge := observability.NewBridge(obs)
	llmStack := llmbridge.WireContextLLM(configFile, obsBridge)
	llmbridge.LogLLMReadiness(configFile)
	if llmbridge.IsMockGateway(llmStack) {
		slog.Warn("llm gateway using mock — set MINIMAX_API_KEY and check devrix.yaml")
	}

	agentToolReg := bootstrap.WireAgentToolRegistry(configFile)
	engineMode := "context"
	if userCfg.IM.Enabled {
		engineMode = config.ResolveContextEngine(userCfg.IM)
	}
	contextEngine := bootstrap.SelectContextEngine(
		engineMode,
		permissionMgr,
		ctxCfg,
		toolCfg,
		obsBridge,
		llmStack,
		agentToolReg,
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

	gw := gateway.NewCommunicationGateway(
		sessionStore,
		eventHandler,
		contextEngine,
		permissionMgr,
		commCfg,
	)
	gw.SetObservability(obs)

	var agentFactory multiagent.IAgentFactory
	if multiAgentCfg.Enabled {
		engineBuilder := bootstrap.NewContextEngineBuilder(llmStack, ctxCfg, toolCfg, obsBridge, agentToolReg).
			WithMultiAgentConfig(multiAgentCfg)
		agentFactory = bootstrap.WireMultiAgent(engineBuilder, multiAgentCfg, obsBridge, contextEngine)
		gw.SetAgentFactory(agentFactory)
		slog.Info("multi-agent layer enabled",
			"max_children", multiAgentCfg.MaxChildren,
			"max_total_agents", multiAgentCfg.MaxTotalAgents,
			"default_mode", multiAgentCfg.DefaultMode,
		)
	}

	bootstrap.WireD7(configFile, gw, contextEngine, obsBridge)

	initOrchestration(configFile, multiAgentCfg.Enabled, llmStack.RawGateway, gw, milestoneService, agentFactory, obs)

	bootstrap.WireExecutionFlow(ctxCfg, gw, obsBridge)
	if ce, ok := contextEngine.(*contextengine.ContextEngine); ok {
		bootstrap.WireDelegate(ctxCfg, multiAgentCfg, gw, ce, ce.ToolRegistry())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		cli := adapters.NewCLIAdapter(gw, commCfg)
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
	gw *gateway.CommunicationGateway,
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

	runtimeJudge := orchestration.NewRuntimeJudge(rawGateway, orchCfg)
	executor := orchestration.NewInterventionExecutor(gw, milestoneSvc, agentFactory)
	orchValidator := orchestration.NewRuntimeOrchestrationValidator(orchCfg, runtimeJudge, executor)
	orchValidator.SetObservability(obs)
	gw.SetAgentObserverFactory(func(ctx context.Context, session *types.Session) orchestration.AgentObserver {
		return orchestration.NewOrchestrationObserver(orchValidator, ctx, session)
	})
	slog.Info("orchestration validator enabled",
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
