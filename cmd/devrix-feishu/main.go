package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/devrix/devrix/internal/bootstrap"
	"github.com/devrix/devrix/internal/layers/communication/adapters"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/instance"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/contextengine"
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
)

func main() {
	logBinaryInfo()

	userCfg, err := config.LoadUserConfig()
	if err != nil {
		slog.Error("failed to load user config", "error", err)
		os.Exit(1)
	}
	if !userCfg.IM.Enabled || userCfg.IM.Platform.Provider != "feishu" {
		slog.Error("im.feishu must be enabled in ~/.devrix/config.yaml")
		os.Exit(1)
	}
	if userCfg.IM.Feishu.AppID == "" || userCfg.IM.Feishu.AppSecret == "" {
		slog.Error("feishu app_id and app_secret are required")
		os.Exit(1)
	}

	configFile := config.FindConfigFile()
	obsCfg := observability.DefaultConfig()
	if configFile != "" {
		if loaded, err := observability.LoadConfigFromFile(configFile); err != nil {
			slog.Warn("failed to load observability config, using defaults", "error", err)
		} else {
			obsCfg = loaded
		}
	}
	obs, err := observability.New(obsCfg)
	if err != nil {
		slog.Warn("failed to initialize observability, continuing without", "error", err)
		obs = observability.NewNoOp()
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

	sessionStore, err := gateway.NewFileSessionStore(commCfg.Session.StorageDir)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	permissionMgr := gateway.NewPermissionManager(&commCfg.Permission)
	permissionMgr.SetUserConfig(userCfg)
	obsBridge := observability.NewBridge(obs)
	llmStack := llmbridge.WireContextLLM(configFile, obsBridge)
	llmbridge.LogLLMReadiness(configFile)
	if llmbridge.IsMockGateway(llmStack) {
		slog.Warn("llm gateway using mock — set MINIMAX_API_KEY and check devrix.yaml")
	}
	milestoneService := milestone.NewMilestoneService(nil)
	engineMode := config.ResolveContextEngine(userCfg.IM)
	contextEngine := selectContextEngine(engineMode, permissionMgr, ctxCfg, toolCfg, obsBridge, llmStack, milestoneService)

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
	feishuAdapter := adapters.NewFeishuAdapter(nil, feishuCfg, commCfg)

	gw := gateway.NewCommunicationGateway(
		sessionStore,
		feishuAdapter,
		contextEngine,
		permissionMgr,
		commCfg,
	)
	gw.SetObservability(obs)
	feishuAdapter.SetGateway(gw)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gw.StartCleanupRoutine(ctx, 30*time.Second)

	instanceRegistry := instance.NewInstanceRegistry(60 * time.Second)
	instanceID := os.Getenv("DEVRIX_INSTANCE_ID")
	if instanceID == "" {
		instanceID = "devrix-feishu"
	}
	instanceName := os.Getenv("DEVRIX_INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "Devrix Feishu"
	}
	instanceInfo := &instance.InstanceInfo{
		ID:      instanceID,
		Name:    instanceName,
		Address: "localhost",
		Port:    8080,
	}
	if err := instanceRegistry.Register(ctx, instanceInfo); err != nil {
		slog.Warn("failed to register instance", "error", err)
	}

	if err := feishuAdapter.Start(ctx); err != nil {
		slog.Error("failed to start feishu adapter", "error", err)
		os.Exit(1)
	}

	slog.Info("devrix feishu server started",
		"app_id", userCfg.IM.Feishu.AppID,
		"engine", engineName(contextEngine),
		"reaction_emoji", feishuCfg.ReactionEmoji,
		"done_emoji", feishuCfg.DoneEmoji,
		"reply_in_thread", feishuCfg.ReplyInThread,
		"progress_style", feishuCfg.ProgressStyle,
	)

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

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = instanceRegistry.Unregister(shutdownCtx, instanceInfo.ID)
	_ = feishuAdapter.Stop()
	_ = obs.Shutdown(context.Background())
}

func selectContextEngine(
	name string,
	permMgr *gateway.PermissionManager,
	ctxCfg *config.ContextEngineConfig,
	toolCfg *config.ToolConfig,
	obsBridge *observability.Bridge,
	llmStack llmbridge.ContextLLMStack,
	milestoneSvc milestone.IMilestoneService,
) gateway.IContextEngine {
	engine := strings.ToLower(strings.TrimSpace(name))
	switch engine {
	case "stub", "echo":
		return gateway.NewStubContextEngine()
	case "four_flow", "fourflow", "four-flow":
		slog.Warn("four_flow engine was removed; using context engine with real LLM")
		fallthrough
	case "", "context", "ctx":
		return bootstrap.NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, milestoneSvc)
	default:
		slog.Warn("unknown context engine; using context engine", "requested", engine)
		return bootstrap.NewContextEngine(llmStack, permMgr, ctxCfg, toolCfg, obsBridge, milestoneSvc)
	}
}

func logBinaryInfo() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	info, err := os.Stat(exe)
	if err != nil {
		slog.Info("devrix-feishu binary", "path", exe)
		return
	}
	slog.Info("devrix-feishu binary",
		"path", exe,
		"mod_time", info.ModTime().Format(time.RFC3339),
		"size_bytes", info.Size(),
	)
}

func engineName(engine gateway.IContextEngine) string {
	switch engine.(type) {
	case *gateway.StubContextEngine:
		return "stub"
	case *contextengine.ContextEngine:
		return "context"
	default:
		return "unknown"
	}
}
