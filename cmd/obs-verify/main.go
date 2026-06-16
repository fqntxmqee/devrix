package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devrix/devrix/internal/bootstrap"
	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/registry"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

func main() {
	obsCfg := observability.DefaultConfig()
	obsCfg.Enabled = true
	obsCfg.Tracing.Enabled = true
	obsCfg.Tracing.Exporter = "console"
	obsCfg.Metrics.Enabled = true
	obsCfg.Logging.Enabled = true

	obs, _ := observability.New(obsCfg)
	obsBridge := observability.NewBridge(obs)

	dir, _ := os.MkdirTemp("", "devrix-obs-verify")
	defer os.RemoveAll(dir)

	store, _ := capture.NewFileSessionStore(dir)
	cfg := config.DefaultConfig()
	ctxCfg := config.DefaultContextEngineConfig()
	handler := testutil.NewMockEventHandler()
	permMgr := capture.NewPermissionManager(&cfg.Permission)

	toolsReg, err := registry.NewBuiltinRegistry()
	if err != nil {
		log.Fatal(err)
	}
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: &mockctx.StaticLLMCaller{Response: "Hello from context engine"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   toolsReg,
		Permission: mockctx.AllowAllPermission{},
		Config:     ctxCfg,
		ObsBridge:  obsBridge,
	})

	gw := capture.NewCommunicationGateway(store, handler, permMgr, cfg)
	gw.SetObservability(obs)
	if err := bootstrap.InitOrchestration("", gw, engine, obsBridge, llmbridge.ContextLLMStack{}, nil); err != nil {
		log.Fatal(err)
	}

	session, _ := gw.CreateSession("cli", "/tmp")
	ctx := context.Background()

	gw.RouteInbound(ctx, &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "test message",
		MessageID: "msg-001",
		ChatID:    "chat-1",
	})

	handler.WaitForMessages(1, 3*time.Second)
	fmt.Fprintf(os.Stderr, "\n=== Messages received: %d ===\n", handler.MessageCount())

	if obs.Meter() != nil {
		fmt.Fprintf(os.Stderr, "\n=== Metrics Output ===\n")
		fmt.Print(obs.Meter().Registry().Output())
	}

	obs.Shutdown(context.Background())
}
