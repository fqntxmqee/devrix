//go:build integration && cross

package integration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/registry"
	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/multiagent/run"
	multiagentprovision "github.com/devrix/devrix/internal/layers/multiagent/provision"
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

// bashOnceThenDoneLLM requests bash once, then completes (avoids infinite loop).
type bashOnceThenDoneLLM struct {
	calls atomic.Int32
}

func (m *bashOnceThenDoneLLM) Call(ctx context.Context, _ contracts.LLMRequest) (<-chan contracts.LLMChunk, error) {
	ch := make(chan contracts.LLMChunk, 2)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if m.calls.Add(1) > 1 {
			ch <- contracts.LLMChunk{Content: "done", Done: true}
			return
		}
		ch <- contracts.LLMChunk{
			ToolCalls: []contracts.ToolCall{{ID: "tc1", Name: "bash", Input: "ls"}},
			Done:      true,
		}
	}()
	return ch, nil
}

// T: D4-S0-A01-T03 — critical tool permission blocks until PermissionManager.Resolve on D7 ingress.
func TestIntegration_GatewayResolveAgentPermission(t *testing.T) {
	handler := testutil.NewMockEventHandler()
	cfg := config.DefaultConfig()
	cfg.Permission.DefaultTimeout = 5 * time.Second
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	permMgr := capture.NewPermissionManager(&cfg.Permission)
	gw := capture.NewCommunicationGateway(store, handler, permMgr, cfg, nil)

	ctxCfg := config.DefaultContextEngineConfig()
	reg := &criticalBashRegistry{BuiltinRegistry: mustBuiltinRegistry(t)}
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: &bashOnceThenDoneLLM{},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:      &mockctx.ToolRunner{},
		ToolsReg:   reg,
		Permission: permMgr,
		Config:     ctxCfg,
	})
	testutil.WireGatewayOrchestration(gw, engine)

	factory := multiagentprovision.NewAgentFactory(
		multiagent.AgentDeps{Engine: engine},
		config.DefaultMultiAgentConfig(),
	)
	gw.SetAgentFactory(factory)

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	resolved := make(chan struct{})
	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			for _, req := range permMgr.ListPending() {
				if err := permMgr.Resolve(req.ID, true); err != nil {
					t.Errorf("Resolve: %v", err)
					return
				}
				close(resolved)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "run bash",
		MessageID: "msg-agent-1",
		ChatID:    "chat-1",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	select {
	case <-resolved:
	case <-time.After(3 * time.Second):
		t.Fatal("expected pending permission request on D7 ingress path")
	}
	gw.WaitForProcesses()
	if !handler.WaitForMessages(1, 2*time.Second) {
		t.Fatal("expected outbound messages from D7 entry → engine path")
	}
}

// T: D4-S0-A01-T03 (direct resolve path)
func TestIntegration_AgentPermissionGateGatewayBridge(t *testing.T) {
	handler := testutil.NewMockEventHandler()
	cfg := config.DefaultConfig()
	gw := capture.NewCommunicationGateway(nil, handler, nil, cfg, nil)

	session := types.NewSession("sess_bridge", "cli", t.TempDir())
	factory := multiagentprovision.NewAgentFactory(multiagent.AgentDeps{
		Engine: &run.StubEngine{Events: []*contracts.EngineEvent{{Type: "complete"}}},
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

	impl, ok := ag.(*run.Impl)
	if !ok {
		t.Fatal("expected *run.Impl")
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
