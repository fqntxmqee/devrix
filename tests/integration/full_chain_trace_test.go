//go:build integration && cross

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/adapter"
	llmgw "github.com/devrix/devrix/internal/layers/llmgateway/gateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/breaker"
	"github.com/devrix/devrix/internal/layers/llmgateway/retry"
	"github.com/devrix/devrix/internal/layers/llmgateway/token"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/coverage"
	"github.com/devrix/devrix/internal/layers/observability/settings"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// stubAdapter returns a fixed LLM response for testing.
type stubAdapter struct {
	response string
}

func (s stubAdapter) Stream(_ context.Context, _ *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	ch := make(chan *llmgateway.AdapterChunk, 2)
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{Content: s.response},
	}
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{Done: true},
	}
	close(ch)
	return ch, nil
}

func (s stubAdapter) Provider() string { return "deepseek" }

// TestIntegration_FullChainD1toD3 verifies the full trace from D1 gateway
// through D2 context engine to D3 LLM gateway, capturing all Jaeger spans.
//
// Covers: L5-CTX-05, L5-CTX-06, L5-CTX-09, L5-CTX-11, L5-LLM-14
func TestIntegration_FullChainD1toD3(t *testing.T) {
	t.Cleanup(testutil.WaitForGatewayAsync)

	// --- Setup observability with memory exporter ---
	obs, err := observability.New(&observability.Config{
		Enabled: true,
		Tracing: settings.TracingConfig{
			Enabled:     true,
			ServiceName: "test-devrix",
			Exporter:    "memory",
			Sampling:    settings.SamplingConfig{Type: "always_on", Rate: 1.0},
		},
		Metrics: settings.MetricsConfig{
			Enabled:  false,
			Exporter: "null",
		},
		Logging: observability.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	})
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	defer func() {
		_ = obs.Shutdown(context.Background())
	}()
	obsBridge := observability.NewBridge(obs)

	// --- D3: Real LLM Gateway with mock adapter ---
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := config.DefaultLLMGatewayConfig()
	cfg.DefaultProvider = "deepseek" // match our stub adapter's provider
	reg := adapter.NewRegistry()
	_ = reg.Register(stubAdapter{response: "Hello from D3"})
	llmGW := llmgw.New(llmgw.Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  breaker.New(cfg.CircuitBreaker),
		Retry:    retry.NewExecutor(),
		Counter:  counter,
		Obs:      obsBridge,
	})
	llmBridge := llmbridge.New(llmGW)

	// --- D2: Context Engine with real components ---
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false // disable long-term to keep test fast
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:          llmBridge,
		TokenCounter: counter,
		Tools:        &NoOpToolRunner{},
		ToolsReg:     mustBuiltinRegistry(t),
		Permission:   &AllowAllPermission{},
		Config:       ctxCfg,
		ObsBridge:    obsBridge,
	})

	// --- D1: Communication Gateway ---
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	permMgr := gateway.NewPermissionManager(&config.DefaultConfig().Permission)
	gw := gateway.NewCommunicationGateway(store, handler, engine, permMgr, config.DefaultConfig())
	gw.SetObservability(obs)

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// --- Route message through the full chain ---
	ctx := context.Background()
	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "test message for full D1-D3 chain",
		MessageID: "msg-001",
		ChatID:    "chat-1",
	}
	if err := gw.RouteInbound(ctx, msg); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	if !handler.WaitForMessages(1, 5*time.Second) {
		t.Fatal("expected outbound messages")
	}

	// --- Shutdown to flush all spans ---
	_ = obs.Shutdown(context.Background())

	// --- Collect and verify spans ---
	memExporter := obs.MemoryExporter()
	if memExporter == nil {
		t.Fatal("memory exporter not found")
	}
	spans := memExporter.Spans()
	if len(spans) == 0 {
		t.Fatal("no spans collected")
	}

	// Print span hierarchy for visual inspection
	t.Logf("=== Collected %d spans ===", len(spans))
	spanByName := make(map[string]int)
	for _, s := range spans {
		name := s.Name()
		spanByName[name]++

		parentInfo := "root"
		if p := s.Parent(); p != nil {
			parentInfo = fmt.Sprintf("parent=%s", p.SpanID.String()[:12])
		}
		attrs := s.Attributes()
		attrStrs := make([]string, 0, len(attrs))
		for k, v := range attrs {
			attrStrs = append(attrStrs, fmt.Sprintf("%s=%v", k, v))
		}
		t.Logf("  [%s] %s (%s) attrs: {%s}",
			parentInfo,
			name,
			s.Kind().String(),
			strings.Join(attrStrs, ", "),
		)
	}

	// --- Verify REQUIRED spans are present ---
	expectedSpans := []struct {
		name   string
		layer  string
		strict bool // true = must be present
	}{
		// D1 Gateway
		{name: "gateway.message.receive", layer: "communication", strict: true},
		{name: "gateway.session.create", layer: "communication", strict: true},
		{name: "gateway.engine_event.handle", layer: "communication", strict: true},
		// D2 Context Engine
		{name: "context.process", layer: "context", strict: true},
		{name: "context.snapshot.load", layer: "context", strict: true},
		{name: "context.system_prompt.load", layer: "context", strict: true},
		{name: "context.pev.run", layer: "context", strict: true},
		{name: "context.pev.iteration", layer: "context", strict: true},
		{name: "context.pev.llm_call", layer: "context", strict: true},
		{name: "context.memory.snapshot.save", layer: "context", strict: true},
		// D3 LLM Gateway
		{name: "llm.stream", layer: "llm", strict: true},
		{name: "llm.provider.route", layer: "llm", strict: true},
		{name: "llm.circuit_breaker", layer: "llm", strict: true},
		{name: "llm.retry", layer: "llm", strict: true},
		{name: "llm.adapter.stream", layer: "llm", strict: true},
	}

	spanSet := make(map[string]bool)
	for _, s := range spans {
		spanSet[s.Name()] = true
	}

	for _, exp := range expectedSpans {
		if spanSet[exp.name] {
			t.Logf("  ✓ %s", exp.name)
		} else if exp.strict {
			t.Errorf("  ✗ MISSING (strict): %s", exp.name)
		} else {
			t.Logf("  - %s (optional, not found)", exp.name)
		}
	}

	t.Logf("=== Span coverage report ===")
	if c := coverage.Global(); c != nil {
		report := c.Report(coverage.AllOperations(), true)
		t.Logf("Total operations: %d, Hit: %d, Missed: %d",
			report.OperationsTotal, report.OperationsHit, len(report.OperationsZeroHit))
		if len(report.OperationsZeroHit) > 0 {
			t.Logf("Zero-hit operations: %v", report.OperationsZeroHit)
		}
	}
}

// TestIntegration_D2MultiRoundPEV verifies the D2 context engine assembly across 3 PEV
// iterations, capturing input/output span data at each round of the execute-verify loop.
//
// Covers: L5-CTX-05, L5-CTX-06, L5-CTX-09, L5-CTX-11, L5-LLM-14
func TestIntegration_D2MultiRoundPEV(t *testing.T) {
	t.Cleanup(testutil.WaitForGatewayAsync)

	obs, err := observability.New(&observability.Config{
		Enabled: true,
		Tracing: settings.TracingConfig{
			Enabled:     true,
			ServiceName: "test-devrix-multiround",
			Exporter:    "memory",
			Sampling:    settings.SamplingConfig{Type: "always_on", Rate: 1.0},
		},
		Metrics: settings.MetricsConfig{
			Enabled:  false,
			Exporter: "null",
		},
		Logging: observability.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	})
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	defer func() { _ = obs.Shutdown(context.Background()) }()
	obsBridge := observability.NewBridge(obs)

	// --- D3: LLM Gateway with tool-call stub adapter ---
	counter, err := token.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := config.DefaultLLMGatewayConfig()
	cfg.DefaultProvider = "deepseek"
	reg := adapter.NewRegistry()
	_ = reg.Register(toolCallStub{})
	llmGW := llmgw.New(llmgw.Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  breaker.New(cfg.CircuitBreaker),
		Retry:    retry.NewExecutor(),
		Counter:  counter,
		Obs:      obsBridge,
	})
	llmBridge := llmbridge.New(llmGW)

	// --- D2: Context Engine with multi-round PEV config ---
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false
	ctxCfg.PEV.MaxIterations = 3
	ctxCfg.PEV.VerifyMode = config.VerifyModeCommands // not single-round → allows multi-iteration

	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		LLM:          llmBridge,
		TokenCounter: counter,
		Tools:        &emptyToolRunner{},
		ToolsReg:     mustBuiltinRegistry(t),
		Permission:   &AllowAllPermission{},
		Config:       ctxCfg,
		ObsBridge:    obsBridge,
	})

	// --- D1: Communication Gateway ---
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	permMgr := gateway.NewPermissionManager(&config.DefaultConfig().Permission)
	gw := gateway.NewCommunicationGateway(store, handler, engine, permMgr, config.DefaultConfig())
	gw.SetObservability(obs)

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// --- Route message through the full chain ---
	ctx := context.Background()
	msg := &types.InboundMessage{
		SessionID: session.SessionID,
		Content:   "deploy the application to production",
		MessageID: "msg-multi-001",
		ChatID:    "chat-1",
	}
	if err := gw.RouteInbound(ctx, msg); err != nil {
		t.Logf("RouteInbound completed (expected after 3 PEV rounds): %v", err)
	}

	if !handler.WaitForMessages(1, 10*time.Second) {
		t.Fatal("expected outbound messages")
	}

	// --- Shutdown to flush all spans ---
	_ = obs.Shutdown(context.Background())

	// --- Collect and display spans ---
	memExporter := obs.MemoryExporter()
	if memExporter == nil {
		t.Fatal("memory exporter not found")
	}
	spans := memExporter.Spans()
	t.Logf("\n=== D2 多轮 PEV 测试: 共采集 %d 个 spans ===", len(spans))

	// Group spans by trace for tree view
	spanByName := make(map[string]int)
	for _, s := range spans {
		spanByName[s.Name()]++
	}

	// Print all spans in order with full attributes
	for _, s := range spans {
		name := s.Name()
		parentInfo := "root"
		if p := s.Parent(); p != nil {
			parentInfo = fmt.Sprintf("parent=%-36s", p.SpanID.String()[:12])
		}
		attrs := s.Attributes()
		attrStrs := make([]string, 0, len(attrs))
		for k, v := range attrs {
			attrStrs = append(attrStrs, fmt.Sprintf("%s=%v", k, v))
		}
		t.Logf("  [%s] %s\n    attrs: {%s}",
			parentInfo,
			name,
			strings.Join(attrStrs, ", "),
		)
	}

	t.Logf("\n=== Span 命中统计 ===")
	t.Logf("PEV iteration spans: %d", spanByName["context.pev.iteration"])
	t.Logf("PEV LLM call spans:  %d", spanByName["context.pev.llm_call"])
	t.Logf("PEV verify spans:    %d", spanByName["context.pev.verify"])
	t.Logf("Tool execute spans:  %d", spanByName["context.pev.tool_execute"])
	t.Logf("LLM stream spans:    %d", spanByName["llm.stream"])

	// Verify at least 3 iteration spans were created (multi-round behavior)
	if spanByName["context.pev.iteration"] < 3 {
		t.Errorf("expected >= 3 PEV iterations, got %d", spanByName["context.pev.iteration"])
	}

	t.Logf("\n=== Coverage report ===")
	if c := coverage.Global(); c != nil {
		report := c.Report(coverage.AllOperations(), true)
		t.Logf("Total operations: %d, Hit: %d, Missed: %d",
			report.OperationsTotal, report.OperationsHit, len(report.OperationsZeroHit))
	}
}

// toolCallStub returns a fixed LLM response containing bash tool calls.
type toolCallStub struct{}

func (s toolCallStub) Stream(_ context.Context, _ *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	ch := make(chan *llmgateway.AdapterChunk, 2)
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{
			Content: "I'll run the deployment script for you",
			ToolCalls: []llmgateway.ToolCall{
				{ID: "tc-1", Name: "bash", Input: "echo deploying"},
			},
		},
	}
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{
			Done:  true,
			Usage: llmgateway.TokenUsage{PromptTokens: 20, CompletionTokens: 10},
		},
	}
	close(ch)
	return ch, nil
}

func (s toolCallStub) Provider() string { return "deepseek" }

// emptyToolRunner returns empty tool output, causing PEV verifyBasic to fail
// and forcing the PEV loop to continue to the next iteration.
type emptyToolRunner struct{}

func (e *emptyToolRunner) Execute(_ context.Context, _ contextengine.ToolCall) (*contextengine.ToolResult, error) {
	return &contextengine.ToolResult{Output: ""}, nil
}

// NoOpToolRunner implements contextengine.IToolRunner with no-op execution.
type NoOpToolRunner struct{}

func (n *NoOpToolRunner) Execute(ctx context.Context, call contextengine.ToolCall) (*contextengine.ToolResult, error) {
	return &contextengine.ToolResult{Output: "no-op"}, nil
}

// AllowAllPermission allows all tool requests.
type AllowAllPermission struct{}

func (a *AllowAllPermission) Request(ctx context.Context, sessionID, toolName, input string, risk types.RiskLevel) bool {
	return true
}
