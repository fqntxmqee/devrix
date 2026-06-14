//go:build integration && cross

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/agent"
	multiagentfactory "github.com/devrix/devrix/internal/layers/multiagent/factory"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

type criticalBashRegistry struct {
	*registry.BuiltinRegistry
}

func (r *criticalBashRegistry) RiskLevel(tool string) types.RiskLevel {
	if tool == "bash" {
		return types.RiskLevelCritical
	}
	return r.BuiltinRegistry.RiskLevel(tool)
}

type testPermGateAdapter struct {
	fn func(context.Context, string, string, string, types.RiskLevel) bool
}

func (a testPermGateAdapter) Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool {
	return a.fn(ctx, sessionID, toolName, input, risk)
}

type integrationEngineBuilder struct {
	llm      llmgateway.ILLMGateway
	tools    contextengine.IToolRunner
	toolsReg contextengine.IToolRegistry
	ctxCfg   *config.ContextEngineConfig
	toolCfg  *config.ToolConfig
}

func (b *integrationEngineBuilder) Build(perm multiagent.PermissionGate) contracts.IEngine {
	var gate contracts.IPermissionGate
	if perm != nil {
		gate = testPermGateAdapter{fn: perm.Request}
	}
	return contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:        b.llm,
		Tools:      b.tools,
		ToolsReg:   b.toolsReg,
		Permission: gate,
		Config:     b.ctxCfg,
	})
}

// T: D4-S0-A01-T03
func TestIntegration_GatewayResolveAgentPermission(t *testing.T) {
	handler := testutil.NewMockEventHandler()
	cfg := config.DefaultConfig()
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, handler, capture.NewPermissionManager(&cfg.Permission), cfg)

	ctxCfg := config.DefaultContextEngineConfig()
	toolCfg := config.DefaultToolConfig()
	reg := &criticalBashRegistry{BuiltinRegistry: mustBuiltinRegistry(t)}
	builder := &integrationEngineBuilder{
		llm:      &mockctx.LLMGatewayWithTools{},
		tools:    &mockctx.ToolRunner{},
		toolsReg: reg,
		ctxCfg:   ctxCfg,
		toolCfg:  toolCfg,
	}
	factory := multiagentfactory.NewAgentFactoryWithBuilder(multiagent.AgentDeps{}, builder, config.DefaultMultiAgentConfig())
	gw.SetAgentFactory(factory)

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "run bash",
		MessageID: "msg-agent-1",
		ChatID:    "chat-1",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	waitUntil(t, 3*time.Second, func() bool {
		return handler.PermissionRequestCount() > 0
	})
	if handler.PermissionRequestCount() == 0 {
		t.Fatal("expected gateway permission request via agent observer")
	}
	if !handler.WaitForMessages(1, 2*time.Second) {
		t.Fatal("expected outbound messages from agent engine sink")
	}
}

// T: D4-S0-A01-T03 (direct resolve path)
func TestIntegration_AgentPermissionGateGatewayBridge(t *testing.T) {
	handler := testutil.NewMockEventHandler()
	cfg := config.DefaultConfig()
	gw := capture.NewCommunicationGateway(nil, handler, nil, cfg)

	session := types.NewSession("sess_bridge", "cli", t.TempDir())
	factory := multiagentfactory.NewAgentFactory(multiagent.AgentDeps{
		Engine: &agent.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}},
	}, config.DefaultMultiAgentConfig())
	ag, err := factory.Create(context.Background(), multiagent.AgentConfig{
		SessionID:         session.SessionID,
		WorkDir:           session.WorkDir,
		PermissionTimeout: 2 * time.Second,
	}, session)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	gw.RegisterSessionAgent(session.SessionID, ag)

	impl, ok := ag.(*agent.Impl)
	if !ok {
		t.Fatal("expected *agent.Impl")
	}
	impl.SetAgentObserver(&gatewayBridgeObserver{gw: gw, session: session})

	done := make(chan bool, 1)
	go func() {
		done <- impl.PermissionGate().Request(
			context.Background(), session.SessionID, "bash", "ls", types.RiskLevelCritical,
		)
	}()

	waitUntil(t, 2*time.Second, func() bool {
		return handler.PermissionRequestCount() > 0
	})
	if handler.PermissionRequestCount() == 0 {
		t.Fatal("expected permission bridge to call handler")
	}

	gw.ResolveAgentPermission(session.SessionID, "bash", true)
	select {
	case granted := <-done:
		if !granted {
			t.Fatal("expected permission granted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for permission resolution")
	}
}

type gatewayBridgeObserver struct {
	gw      *capture.CommunicationGateway
	session *types.Session
}

func (o *gatewayBridgeObserver) EmitAgentEvent(ev multiagent.AgentEvent) {
	if ev.EventType != "permission_required" {
		return
	}
	tool, _ := ev.Metadata["tool"].(string)
	req := types.NewPermissionRequest("req-test", o.session.SessionID, tool, types.RiskLevelCritical, time.Minute)
	approved, _ := o.gw.RoutePermission(req)
	o.gw.ResolveAgentPermission(o.session.SessionID, tool, approved)
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
}
