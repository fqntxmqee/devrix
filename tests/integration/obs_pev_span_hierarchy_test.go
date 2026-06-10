//go:build integration && cross

package integration

import (
	"context"
	"testing"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	"github.com/devrix/devrix/internal/layers/llmgateway/breaker"
	llmgw "github.com/devrix/devrix/internal/layers/llmgateway/gateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/settings"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// Covers: L5-OBS-TRACE-04 — Canonical Trace Tree parent-child (R1-R2)
func TestIntegration_PEVSpanHierarchy_should_match_canonical_tree(t *testing.T) {
	obs, err := observability.New(&observability.Config{
		Enabled: true,
		Tracing: settings.TracingConfig{
			Enabled:     true,
			ServiceName: "test-pev-hierarchy",
			Exporter:    "memory",
			Sampling:    settings.SamplingConfig{Type: "always_on", Rate: 1.0},
		},
		Metrics: observability.MetricsConfig{Enabled: false, Exporter: "null"},
		Logging: observability.LoggingConfig{Level: "info", Format: "text"},
	})
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	defer func() { _ = obs.Shutdown(context.Background()) }()
	obsBridge := observability.NewBridge(obs)

	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := config.DefaultLLMGatewayConfig()
	cfg.DefaultProvider = "deepseek"
	reg := adapter.NewRegistry()
	_ = reg.Register(stubAdapter{response: "hierarchy test reply"})
	llmGW := llmgw.New(llmgw.Deps{
		Config: cfg, Registry: reg, Breaker: breaker.New(cfg.CircuitBreaker),
		Retry: retry.NewExecutor(), Counter: counter, Obs: obsBridge,
	})

	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM: llmbridge.New(llmGW), TokenCounter: counter,
		Tools: &NoOpToolRunner{}, ToolsReg: registry.NewBuiltinRegistry(),
		Permission: &AllowAllPermission{}, Config: ctxCfg, ObsBridge: obsBridge,
	})

	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	gw := gateway.NewCommunicationGateway(store, handler, engine, gateway.NewPermissionManager(&config.DefaultConfig().Permission), config.DefaultConfig())
	gw.SetObservability(obs)

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID, Content: "span hierarchy test", MessageID: "msg-hier-1", ChatID: "c1",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}
	if !handler.WaitForMessages(1, 5*time.Second) {
		t.Fatal("expected outbound messages")
	}
	_ = obs.Shutdown(context.Background())

	spans := obs.MemoryExporter().Spans()
	byName := indexSpansByName(spans)

	assertSpanParent(t, byName, telemetry.OpContextPEVIteration, telemetry.OpContextPEVRun)
	assertSpanParent(t, byName, telemetry.OpContextPEVLLMCall, telemetry.OpContextPEVIteration)
	assertSpanParent(t, byName, telemetry.OpLLMStream, telemetry.OpContextPEVLLMCall)

	// gen_ai.* dual-write on LLM call span
	for _, s := range byName[telemetry.OpContextPEVLLMCall] {
		attrs := s.Attributes()
		if attrs["gen_ai.request.model"] == nil {
			t.Error("missing gen_ai.request.model on context.pev.llm_call")
		}
		if attrs["gen_ai.conversation.id"] == nil {
			t.Error("missing gen_ai.conversation.id on context.pev.llm_call")
		}
	}
}

func indexSpansByName(spans []tracer.ReadableSpan) map[string][]tracer.ReadableSpan {
	out := make(map[string][]tracer.ReadableSpan)
	for _, s := range spans {
		out[s.Name()] = append(out[s.Name()], s)
	}
	return out
}

func assertSpanParent(t *testing.T, byName map[string][]tracer.ReadableSpan, childName, parentName string) {
	t.Helper()
	children := byName[childName]
	parents := byName[parentName]
	if len(children) == 0 {
		t.Fatalf("no spans named %q", childName)
	}
	if len(parents) == 0 {
		t.Fatalf("no spans named %q", parentName)
	}

	parentIDs := make(map[string]bool)
	for _, p := range parents {
		parentIDs[p.SpanContext().SpanID.String()] = true
	}
	for _, c := range children {
		p := c.Parent()
		if p == nil || !p.IsValid() {
			t.Fatalf("%q span missing parent, expected %q", childName, parentName)
		}
		if !parentIDs[p.SpanID.String()] {
			t.Fatalf("%q parent spanID=%s not in %q spans", childName, p.SpanID.String(), parentName)
		}
	}
}
