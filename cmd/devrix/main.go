package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/devrix/devrix/internal/layers/communication/adapters"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/shared/config"
)

func main() {
	// Initialize structured logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load configuration
	cfg, err := config.NewConfigLoader().Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize session store
	sessionStore, err := gateway.NewFileSessionStore(cfg.Session.StorageDir)
	if err != nil {
		slog.Error("failed to create session store", "error", err)
		os.Exit(1)
	}

	// Initialize gateway components
	permissionMgr := gateway.NewPermissionManager(&cfg.Permission)

	// Create communication gateway
	gw := gateway.NewCommunicationGateway(
		sessionStore,
		nil, // eventHandler
		nil, // contextEngine (placeholder)
		permissionMgr,
		cfg,
	)

	// Create CLI adapter
	cli := adapters.NewCLIAdapter(gw, cfg)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start CLI
	slog.Info("starting devrix", "version", "v1.0")
	if err := cli.Start(ctx); err != nil {
		slog.Error("cli exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("devrix stopped")
}
