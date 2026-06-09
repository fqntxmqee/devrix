package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devrix/devrix/internal/bootstrap"
	"github.com/devrix/devrix/internal/layers/communication/adapters"
	"github.com/devrix/devrix/internal/layers/communication/auth"
	"github.com/devrix/devrix/internal/layers/communication/connection"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/instance"
	"github.com/devrix/devrix/internal/layers/communication/metrics"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/communication/ratelimit"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/multiagent/tool"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func main() {
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
	contextengine.ConfigureLLMLogging(contextengine.LLMLogSettings{
		LogContent: obsCfg.LLM.LogContent,
		LogDir:     obsCfg.LLM.LogDir,
	})

	var obs *observability.Observability
	var err error

	obs, err = observability.New(obsCfg)
	if err != nil {
		slog.Warn("failed to initialize observability, continuing without", "error", err)
		obs = observability.NewNoOp()
	} else {
		slog.Info("observability initialized",
			"tracing", obsCfg.IsTracingEnabled(),
			"exporter", obsCfg.Tracing.Exporter,
			"otlp_endpoint", obsCfg.Tracing.OTLP.Endpoint,
			"metrics", obsCfg.IsMetricsEnabled(),
			"logging", obsCfg.IsLoggingEnabled(),
		)
	}

	// Load user config
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		slog.Warn("failed to load user config, using defaults", "error", err)
		userCfg = config.DefaultUserConfig()
	}

	if userCfg.IsYOLOMode() {
		slog.Info("YOLO mode enabled - all permissions auto-approved")
	}

	// Find and load project config
	commCfg, authCfg, _, ctxCfg, err := config.LoadConfig(configFile)
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

	// Load agent tools configuration
	agentToolsCfg, err := config.LoadAgentToolsConfig(configFile)
	if err != nil {
		slog.Error("failed to load agent tools config", "error", err)
		os.Exit(1)
	}
	var agentToolReg *tool.Registry
	if agentToolsCfg.Enabled {
		reg := tool.NewRegistry()
		for _, tCfg := range agentToolsCfg.Tools {
			var agt tool.AgentTool
			switch tCfg.Type {
			case "cursor":
				agt = tool.NewCursorAgentTool(tool.CursorConfig{
					Name:         tCfg.Name,
					DisplayName:  tCfg.DisplayName,
					Description:  tCfg.Description,
					Capabilities: tCfg.Capabilities,
					Role:         tCfg.Role,
					Command:      tCfg.Command,
					Model:        tCfg.Model,
					Mode:         tCfg.Mode,
					WorkDir:      tCfg.WorkDir,
					Timeout:      tCfg.Timeout,
				})
			default:
				agt = tool.NewCLIAgentTool(tool.CLIConfig{
					Name:         tCfg.Name,
					DisplayName:  tCfg.DisplayName,
					Description:  tCfg.Description,
					Capabilities: tCfg.Capabilities,
					Role:         tCfg.Role,
					Command:      tCfg.Command,
					Args:         tCfg.Args,
					WorkDir:      tCfg.WorkDir,
					Timeout:      tCfg.Timeout,
					IdleTimeout:  tCfg.IdleTimeout,
				})
			}
			if err := reg.Register(agt); err != nil {
				slog.Error("register agent tool", "name", tCfg.Name, "error", err)
				os.Exit(1)
			}
			slog.Info("agent tool registered", "name", tCfg.Name)
		}
		agentToolReg = reg
	}

	// Initialize session store
	sessionStore, err := gateway.NewFileSessionStore(commCfg.Session.StorageDir)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	// Initialize components
	authService := auth.NewAuthService(authCfg)
	connManager := connection.NewConnectionManager(60*time.Second, 10*time.Second)
	rateLimiter := ratelimit.NewRateLimiter(ratelimit.DefaultRateLimitConfig())
	metricsCollector := metrics.NewMetricsCollector()
	instanceRegistry := instance.NewInstanceRegistry(60 * time.Second)
	milestoneService := milestone.NewMilestoneService(nil)
	_ = authService
	_ = rateLimiter

	// Initialize permission manager
	permissionMgr := gateway.NewPermissionManager(&commCfg.Permission)
	permissionMgr.SetUserConfig(userCfg)

	// Wire LLM gateway (Layer 3) → Context Engine (Layer 2)
	obsBridge := observability.NewBridge(obs)
	llmStack := llmbridge.WireContextLLM(configFile, obsBridge)
	llmbridge.LogLLMReadiness(configFile)
	contextEngine := bootstrap.NewContextEngine(llmStack, permissionMgr, ctxCfg, toolCfg, obsBridge, milestoneService, agentToolReg)

	// Create event handler
	defaultEventHandler := &DefaultEventHandler{
		connManager: connManager,
		metrics:    metricsCollector,
		obs:        obs,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize IM adapter
	var feishuAdapter *adapters.FeishuAdapter
	var eventHandler gateway.EventHandler = defaultEventHandler

	if userCfg.IM.Enabled {
		switch userCfg.IM.Platform.Provider {
		case "feishu":
			if userCfg.IM.Feishu.AppID != "" && userCfg.IM.Feishu.AppSecret != "" {
				feishuCfg := &adapters.FeishuConfig{
					AppID:         userCfg.IM.Feishu.AppID,
					AppSecret:     userCfg.IM.Feishu.AppSecret,
					BotName:       userCfg.IM.Feishu.BotName,
					Domain:        userCfg.IM.Feishu.Domain,
					EncryptKey:    userCfg.IM.Feishu.EncryptKey,
					CallbackPath:  "/feishu/webhook",
					Port:          "8080",
					UseWebhook:    userCfg.IM.Feishu.UseWebhook,
					ReactionEmoji: userCfg.IM.Feishu.ReactionEmoji,
					DoneEmoji:     userCfg.IM.Feishu.DoneEmoji,
					ReplyInThread: userCfg.IM.Feishu.IsReplyInThread(),
					ProgressStyle: userCfg.IM.Feishu.ProgressStyle,
				}
				feishuAdapter = adapters.NewFeishuAdapter(nil, feishuCfg, commCfg)
				eventHandler = feishuAdapter
			}
		}
	}

	// Create gateway
	gw := gateway.NewCommunicationGateway(
		sessionStore,
		eventHandler,
		contextEngine,
		permissionMgr,
		commCfg,
	)
	gw.SetObservability(obs)

	if multiAgentCfg.Enabled {
		engineBuilder := bootstrap.NewContextEngineBuilder(llmStack, ctxCfg, toolCfg, obsBridge, milestoneService, agentToolReg)
		agentFactory := bootstrap.WireMultiAgent(engineBuilder, multiAgentCfg, obsBridge)
		gw.SetAgentFactory(agentFactory)
		slog.Info("multi-agent layer enabled",
			"max_children", multiAgentCfg.MaxChildren,
			"max_total_agents", multiAgentCfg.MaxTotalAgents,
			"default_mode", multiAgentCfg.DefaultMode,
		)
	}

	gw.StartCleanupRoutine(ctx, 30*time.Second)

	// Start IM adapter
	if feishuAdapter != nil {
		feishuAdapter.SetGateway(gw)
		if err := feishuAdapter.Start(ctx); err != nil {
			slog.Warn("failed to start feishu adapter", "error", err)
		} else {
			slog.Info("feishu adapter started", "app_id", userCfg.IM.Feishu.AppID)
		}
	}

	// Create CLI adapter
	cli := adapters.NewCLIAdapter(gw, commCfg)

	// Register instance
	instanceID := os.Getenv("DEVRIX_INSTANCE_ID")
	if instanceID == "" {
		instanceID = "devrix-cli"
	}
	instanceName := os.Getenv("DEVRIX_INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "Devrix CLI"
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

	// Handle shutdown: first Ctrl+C cancels ctx; second (or timeout) force-exits.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

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
		"observability", obsCfg.Enabled,
		"tracing", obsCfg.IsTracingEnabled(),
		"metrics", obsCfg.IsMetricsEnabled(),
		"logging", obsCfg.IsLoggingEnabled(),
		"yolo_mode", userCfg.IsYOLOMode(),
	)

	if err := cli.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("cli exited with error", "error", err)
		os.Exit(1)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := obs.Shutdown(shutdownCtx); err != nil {
		slog.Warn("observability shutdown error", "error", err)
	}
	connManager.Stop()
	instanceRegistry.Unregister(shutdownCtx, instanceInfo.ID)
	if feishuAdapter != nil {
		_ = feishuAdapter.Stop()
	}

	slog.Info("devrix stopped")
}

// DefaultEventHandler handles gateway events
type DefaultEventHandler struct {
	connManager *connection.ConnectionManager
	metrics    *metrics.MetricsCollector
	obs        *observability.Observability
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

