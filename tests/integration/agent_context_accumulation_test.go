//go:build integration && cross

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	llmgw "github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/multiagent"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D2-S3-A01-T02, D4-S0-A01-T03 — shared engine context across D7 ingress turns.
func TestIntegration_AgentRouteSessionContextAccumulation(t *testing.T) {
	handler := testutil.NewMockEventHandler()
	commCfg := config.DefaultConfig()

	counter, err := budget.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	llmCfg := configure.DefaultLLMGatewayConfig()
	llmCfg.DefaultProvider = "deepseek"
	if p, ok := llmCfg.Providers["deepseek"]; ok && p.DefaultModel != "" {
		llmCfg.DefaultModel = p.DefaultModel
	}
	reg := adapter.NewRegistry()
	ech := &echoContextStub{}
	_ = reg.Register(ech)
	llmGW := llmgw.New(llmgw.Deps{
		Config:   llmCfg,
		Registry: reg,
		Breaker:  protect.New(llmCfg.CircuitBreaker),
		Retry:    protect.NewExecutor(),
		Counter:  counter,
	})
	llmBridge := llmbridge.New(llmGW)
	llmStack := llmbridge.ContextLLMStack{
		Gateway:      llmBridge,
		RawGateway:   llmGW,
		TokenCounter: counter,
		DefaultModel: llmCfg.DefaultModel,
		TierResolver: llmBridge,
	}

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false
	engine := kernel.NewContextEngine(testutil.MergeEngineDeps(
		testutil.ContextEngineDepsFromStack(llmStack, ctxCfg),
		kernel.EngineDeps{
			PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "Echo: agent route"},
			Tools:              &enforce.ToolRunner{},
			ToolsReg:           mustBuiltinRegistry(t),
			Permission:         enforce.AllowAllPermission{},
		},
	))

	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	permMgr := capture.NewPermissionManager(&commCfg.Permission)
	gw := capture.NewCommunicationGateway(store, handler, permMgr, commCfg, nil)
	testutil.WireGatewayOrchestration(gw, engine)

	factory := multiagentprovision.NewAgentFactory(
		multiagent.AgentDeps{Engine: engine},
		config.DefaultMultiAgentConfig(),
	)
	testutil.WireGatewaySessionAgents(gw, factory)

	session, err := gw.CreateSession("feishu_chat", dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	ctx := context.Background()
	instructions := []string{"first via d7", "second via d7"}
	for i, inst := range instructions {
		msg := &types.InboundMessage{
			SessionID: session.SessionID,
			Content:   inst,
			MessageID: fmt.Sprintf("msg-agent-acc-%d", i+1),
			ChatID:    "feishu_chat",
		}
		if err := gw.RouteInbound(ctx, msg); err != nil {
			t.Fatalf("RouteInbound round %d: %v", i+1, err)
		}
		gw.WaitForProcesses()
		expectedEvents := (i + 1) * 2
		if !handler.WaitForMessages(expectedEvents, 10*time.Second) {
			t.Fatalf("round %d: timeout, got %d events", i+1, handler.MessageCount())
		}
		time.Sleep(200 * time.Millisecond)

		sc, ok := engine.SessionContext(session.SessionID)
		if !ok {
			t.Fatalf("round %d: session context missing", i+1)
		}
		wantMsgs := (i + 1) * 2
		if len(sc.Messages) != wantMsgs {
			t.Fatalf("round %d: expected %d messages, got %d", i+1, wantMsgs, len(sc.Messages))
		}

		if d3req := ech.LastRequest(); d3req != nil && i > 0 {
			if len(d3req.Messages) < 2 {
				t.Fatalf("round %d: LLM saw %d messages, want >= 2", i+1, len(d3req.Messages))
			}
		}
	}

	updated, err := store.Get(session.SessionID)
	if err != nil {
		t.Fatalf("Get session: %v", err)
	}
	if len(updated.ContextSnapshot) == 0 {
		t.Fatal("expected ContextSnapshot persisted on disk after D7 ingress")
	}
}
