package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/adapters"
	"github.com/devrix/devrix/internal/layers/communication/auth"
	"github.com/devrix/devrix/internal/layers/communication/connection"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/instance"
	"github.com/devrix/devrix/internal/layers/communication/metrics"
	"github.com/devrix/devrix/internal/layers/communication/milestone"
	"github.com/devrix/devrix/internal/layers/communication/ratelimit"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func main() {
	// Initialize structured logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load user config first (for YOLO mode)
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		slog.Warn("failed to load user config, using defaults", "error", err)
		userCfg = config.DefaultUserConfig()
	}

	// Show YOLO mode status
	if userCfg.IsYOLOMode() {
		slog.Info("YOLO mode enabled - all permissions auto-approved")
	}

	// Find and load project config file
	configFile := config.FindConfigFile()
	if configFile != "" {
		slog.Info("loading config file", "path", configFile)
	}

	commCfg, authCfg, _, err := config.LoadConfig(configFile)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize session store
	sessionStore, err := gateway.NewFileSessionStore(commCfg.Session.StorageDir)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	// Initialize V2 components
	authService := auth.NewAuthService(authCfg)

	connManager := connection.NewConnectionManager(
		60*time.Second, // heartbeat timeout (from connection config)
		10*time.Second, // heartbeat interval
	)

	rateLimiter := ratelimit.NewRateLimiter(ratelimit.DefaultRateLimitConfig())

	// Initialize V3 components
	metricsCollector := metrics.NewMetricsCollector()
	instanceRegistry := instance.NewInstanceRegistry(60 * time.Second)
	milestoneService := milestone.NewMilestoneService(nil)

	// Initialize permission manager
	permissionMgr := gateway.NewPermissionManager(&commCfg.Permission)
	permissionMgr.SetUserConfig(userCfg) // Enable YOLO mode if configured

	// Create context engine (stub for now)
	contextEngine := gateway.NewStubContextEngine()

	// Create default event handler (used when no IM is connected)
	defaultEventHandler := &DefaultEventHandler{
		connManager: connManager,
		metrics:     metricsCollector,
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize IM adapter based on user config (only ONE platform at a time)
	var feishuAdapter *adapters.FeishuAdapter
	var eventHandler gateway.EventHandler = defaultEventHandler
	imConnected := false

	if userCfg.IM.Enabled {
		switch userCfg.IM.Platform.Provider {
		case "feishu":
			if userCfg.IM.Feishu.AppID != "" && userCfg.IM.Feishu.AppSecret != "" {
				feishuCfg := &adapters.FeishuConfig{
					AppID:       userCfg.IM.Feishu.AppID,
					AppSecret:   userCfg.IM.Feishu.AppSecret,
					BotName:     userCfg.IM.Feishu.BotName,
					Domain:      userCfg.IM.Feishu.Domain,
					EncryptKey:  userCfg.IM.Feishu.EncryptKey,
					CallbackPath: "/feishu/webhook",
					Port:        "8080",
					UseWebhook:  userCfg.IM.Feishu.UseWebhook,
				}
				feishuAdapter = adapters.NewFeishuAdapter(nil, feishuCfg, commCfg)
				// Feishu adapter IS the event handler when IM is connected
				eventHandler = feishuAdapter
			} else {
				slog.Warn("feishu enabled but app_id or app_secret is empty")
			}

		case "dingtalk":
			if userCfg.IM.DingTalk.AppKey != "" && userCfg.IM.DingTalk.AppSecret != "" {
				slog.Info("dingtalk adapter not yet implemented")
			} else {
				slog.Warn("dingtalk enabled but app_key or app_secret is empty")
			}

		default:
			slog.Warn("IM enabled but no valid platform configured", "provider", userCfg.IM.Platform.Provider)
		}
	}

	// Create communication gateway with the event handler
	gw := gateway.NewCommunicationGateway(
		sessionStore,
		eventHandler,
		contextEngine,
		permissionMgr,
		commCfg,
	)

	// Start session cleanup routine
	gw.StartCleanupRoutine(ctx, 30*time.Second)

	// Now set the gateway on Feishu adapter
	if feishuAdapter != nil {
		feishuAdapter.SetGateway(gw)
		if err := feishuAdapter.Start(ctx); err != nil {
			slog.Warn("failed to start feishu adapter", "error", err)
		} else {
			slog.Info("feishu adapter started", "app_id", userCfg.IM.Feishu.AppID)
			imConnected = true
		}
	}

	// Create CLI adapter
	cli := adapters.NewCLIAdapter(gw, commCfg)

	// Log IM status
	slog.Info("IM configuration",
		"enabled", userCfg.IM.Enabled,
		"provider", userCfg.IM.Platform.Provider,
		"connected", imConnected,
	)

	// Register this instance
	instanceID := os.Getenv("DEVRIX_INSTANCE_ID")
	if instanceID == "" {
		instanceID = "devrix-cli"
	}
	instanceName := os.Getenv("DEVRIX_INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "Devrix CLI"
	}

	instanceInfo := &instance.InstanceInfo{
		ID:        instanceID,
		Name:      instanceName,
		Address:   "localhost",
		Port:      0,
		Status:    "healthy",
	}
	if err := instanceRegistry.Register(ctx, instanceInfo); err != nil {
		slog.Warn("failed to register instance", "error", err)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		connManager.Stop()
		instanceRegistry.Unregister(ctx, instanceInfo.ID)
		if feishuAdapter != nil {
			feishuAdapter.Stop()
		}
		cancel()
	}()

	// Log startup info
	slog.Info("devrix components initialized",
		"auth", authService != nil,
		"conn_manager", connManager != nil,
		"rate_limiter", rateLimiter != nil,
		"metrics", metricsCollector != nil,
		"instance_registry", instanceRegistry != nil,
		"milestone_service", milestoneService != nil,
		"feishu", feishuAdapter != nil,
		"yolo_mode", userCfg.IsYOLOMode(),
		"im_enabled", userCfg.IM.Enabled,
		"im_provider", userCfg.IM.Platform.Provider,
	)

	// Start CLI
	slog.Info("starting devrix", "version", "v2.0")
	if err := cli.Start(ctx); err != nil {
		slog.Error("cli exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("devrix stopped")
}

// DefaultEventHandler handles gateway events
type DefaultEventHandler struct {
	connManager *connection.ConnectionManager
	metrics     *metrics.MetricsCollector
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
