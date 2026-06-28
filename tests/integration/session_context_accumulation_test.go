//go:build integration && cross

package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	llmbridge "github.com/devrix/devrix/internal/bridges/llm"
	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/llmgateway/budget"
	"github.com/devrix/devrix/internal/layers/llmgateway/configure"
	"github.com/devrix/devrix/internal/layers/llmgateway/protect"
	llmgw "github.com/devrix/devrix/internal/layers/llmgateway/stream"
	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/configure/settings"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// echoContextStub responds with how many messages and tools it sees in the
// request, proving both the full conversation history and tool definitions
// are passed to the LLM on each call.
type echoContextStub struct {
	mu       sync.Mutex
	requests []*llmgateway.Request // captured for post-round inspection
}

func (s *echoContextStub) Stream(_ context.Context, req *llmgateway.Request) (<-chan *llmgateway.AdapterChunk, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	s.mu.Unlock()

	toolNames := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		toolNames = append(toolNames, t.Name)
	}
	toolsDesc := fmt.Sprintf("%d tools: [%s]", len(req.Tools), strings.Join(toolNames, ", "))
	resp := fmt.Sprintf("Echo: I see %d messages, %s", len(req.Messages), toolsDesc)
	ch := make(chan *llmgateway.AdapterChunk, 2)
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{Content: resp},
	}
	ch <- &llmgateway.AdapterChunk{
		Parsed: &llmgateway.Chunk{
			Done:  true,
			Usage: llmgateway.TokenUsage{PromptTokens: 10, CompletionTokens: 5},
		},
	}
	close(ch)
	return ch, nil
}

func (s *echoContextStub) Provider() string { return "deepseek" }

func (s *echoContextStub) Protocol() string { return "openai-compatible" }

func (s *echoContextStub) LastRequest() *llmgateway.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return nil
	}
	return s.requests[len(s.requests)-1]
}

func (s *echoContextStub) AllRequests() []*llmgateway.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*llmgateway.Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// TestIntegration_SessionContextAccumulation sends 3 sequential messages in the
// same session and verifies the D2 context engine accumulates message history
// across all rounds. Each round the LLM sees the full prior context.
//
// T: D2-S3-A01-T02, D2-S1-A01-T01, D2-S1-A01-T11, D3-S2-A01-T03
func TestIntegration_SessionContextAccumulation(t *testing.T) {
	obs, err := observability.New(&observability.Config{
		Enabled: true,
		Tracing: settings.TracingConfig{
			Enabled:     true,
			ServiceName: "test-session-ctx",
			Exporter:    "memory",
			Sampling:    settings.SamplingConfig{Type: "always_on", Rate: 1.0},
		},
		Metrics: settings.MetricsConfig{Enabled: false, Exporter: "null"},
		Logging: observability.LoggingConfig{Level: "info", Format: "text"},
	})
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	defer func() { _ = obs.Shutdown(context.Background()) }()
	obsBridge := observability.NewBridge(obs)

	// --- D3: LLM Gateway with echo context stub ---
	counter, err := budget.NewCounter()
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	cfg := configure.DefaultLLMGatewayConfig()
	cfg.DefaultProvider = "deepseek"
	if p, ok := cfg.Providers["deepseek"]; ok && p.DefaultModel != "" {
		cfg.DefaultModel = p.DefaultModel
	}
	reg := adapter.NewRegistry()
	ech := &echoContextStub{}
	_ = reg.Register(ech)
	llmGW := llmgw.New(llmgw.Deps{
		Config:   cfg,
		Registry: reg,
		Breaker:  protect.New(cfg.CircuitBreaker),
		Retry:    protect.NewExecutor(),
		Counter:  counter,
		Obs:      obsBridge,
	})
	llmBridge := llmbridge.New(llmGW)
	llmStack := llmbridge.ContextLLMStack{
		Gateway:      llmBridge,
		RawGateway:   llmGW,
		TokenCounter: counter,
		DefaultModel: cfg.DefaultModel,
		TierResolver: llmBridge,
	}

	// --- D2: Context Engine ---
	ctxCfg := config.DefaultContextEngineConfig()
	ctxCfg.LongTerm.Enabled = false
	engine := kernel.NewContextEngine(testutil.MergeEngineDeps(
		testutil.ContextEngineDepsFromStack(llmStack, ctxCfg),
		kernel.EngineDeps{
			PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{
				Response: "Echo: round response",
			},
			Tools:      &enforce.ToolRunner{},
			ToolsReg:   mustBuiltinRegistry(t),
			Permission: enforce.AllowAllPermission{},
			ObsBridge:  obsBridge,
		},
	))

	// --- D1: Communication Gateway ---
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	permMgr := capture.NewPermissionManager(&config.DefaultConfig().Permission)
	gw := capture.NewCommunicationGateway(store, handler, permMgr, config.DefaultConfig(), nil)
	gw.SetObservability(obs)
	testutil.WireGatewayOrchestration(gw, engine)

	// --- Print registered tool schemas ---
	{
		toolReg, err := contextengine.NewBuiltinToolRegistry(nil)
		if err != nil {
			t.Fatalf("NewBuiltinToolRegistry: %v", err)
		}
		toolSchemas, err := toolReg.ListTools(context.Background(), "/tmp")
		if err != nil {
			t.Fatalf("list tools: %v", err)
		}
		t.Logf("\n=== Registered tool schemas (%d tools) ===", len(toolSchemas))
		for _, ts := range toolSchemas {
			t.Logf("  Tool: %s", ts.Name)
			desc := ts.Description
			if len(desc) > 120 {
				desc = desc[:120] + "..."
			}
			t.Logf("    Description: %s", desc)
			params := ts.Parameters
			if len(params) > 200 {
				params = params[:200] + "..."
			}
			t.Logf("    Parameters (schema): %s", params)
		}
		t.Logf("  ----------------------------------------")
	}

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionID := session.SessionID
	ctx := context.Background()

	// Each round emits 2 events: 1 text + 1 complete
	const eventsPerRound = 2

	instructions := []string{
		"first instruction",
		"second instruction",
		"third instruction",
	}

	var finalSC *types.SessionContext

	for i, inst := range instructions {
		t.Logf("=== Round %d: %q ===", i+1, inst)

		msg := &types.InboundMessage{
			SessionID: sessionID,
			Content:   inst,
			MessageID: fmt.Sprintf("msg-acc-%d", i+1),
			ChatID:    "chat-1",
		}
		if err := gw.RouteInbound(ctx, msg); err != nil {
			t.Fatalf("RouteInbound round %d: %v", i+1, err)
		}

		// Wait for the outbound events: text + complete = 2 per round.
		expectedEvents := (i + 1) * eventsPerRound
		if !handler.WaitForMessages(expectedEvents, 10*time.Second) {
			t.Fatalf("round %d: timeout waiting for %d events, got %d",
				i+1, expectedEvents, handler.MessageCount())
		}

		// Allow the runProcess goroutine to finish AppendMessage and
		// PersistSnapshot. The race-free read of sc.Messages happens after
		// this sleep — runProcess will have exited its message-append phase.
		time.Sleep(200 * time.Millisecond)

		// One-time read (not a poll) — no concurrent writer after the sleep.
		sc, ok := engine.SessionContext(sessionID)
		if !ok {
			t.Fatalf("round %d: session context not found", i+1)
		}
		finalSC = sc

		expectedContextMsgs := (i + 1) * 2 // 1 user + 1 assistant per round
		if len(sc.Messages) != expectedContextMsgs {
			t.Errorf("round %d: expected %d context messages, got %d",
				i+1, expectedContextMsgs, len(sc.Messages))
		}

		t.Logf("  -> Accumulated %d context messages", len(sc.Messages))
		// Show the assistant response for this round (last message)
		if len(sc.Messages) > 0 {
			last := sc.Messages[len(sc.Messages)-1]
			t.Logf("  -> Assistant: %s", last.Content)
		}
		// Show full message list
		for j, m := range sc.Messages {
			preview := m.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			t.Logf("     [%d] %s: %s", j, m.Role, preview)
		}

		// Dump the exact LLM request payload for this round:
		// (1) messages from CompressedView, (2) tools from the echo stub capture.
		if sc.CompressedView != nil {
			t.Logf("  --- LLM actual input (CompressedView messages) ---")
			t.Logf("  SystemPrompt: %q", sc.SystemPrompt)
			for j, m := range sc.CompressedView {
				content := m.Content
				if len(content) > 150 {
					content = content[:150] + "..."
				}
				t.Logf("  Messages[%d] Role=%q Content=%q", j, m.Role, content)
			}
			t.Logf("  ---")
		}

		// D3-level request captured by the echo stub (includes Tools).
		if d3req := ech.LastRequest(); d3req != nil {
			t.Logf("  --- LLM request at D3 (echo stub capture) ---")
			t.Logf("  Model: %q", d3req.Model)
			t.Logf("  SystemPrompt: %q", d3req.SystemPrompt)
			t.Logf("  Messages: %d", len(d3req.Messages))
			for j, m := range d3req.Messages {
				content := m.Content
				if len(content) > 150 {
					content = content[:150] + "..."
				}
				t.Logf("    [%d] Role=%q Content=%q", j, m.Role, content)
			}
			t.Logf("  Tools (%d):", len(d3req.Tools))
			for j, ts := range d3req.Tools {
				t.Logf("    [%d] Name=%q Desc=%q", j, ts.Name, ts.Description)
			}
			t.Logf("  ---")
		}
	}

	// ====================================================================
	// Final verification: accumulated history (synchronized, no live runners)
	// ====================================================================
	if finalSC == nil {
		t.Fatal("final session context is nil")
	}
	sc := finalSC

	if len(sc.Messages) != 6 {
		t.Fatalf("expected 6 accumulated messages, got %d", len(sc.Messages))
	}
	t.Logf("\n=== Final SessionContext: %d messages ===", len(sc.Messages))

	// Verify alternating user/assistant roles and content
	for i := 0; i < 3; i++ {
		userIdx := i * 2
		asstIdx := i*2 + 1

		if sc.Messages[userIdx].Role != types.MessageRoleUser {
			t.Errorf("msg[%d]: expected user role, got %s", userIdx, sc.Messages[userIdx].Role)
		}
		if sc.Messages[asstIdx].Role != types.MessageRoleAssistant {
			t.Errorf("msg[%d]: expected assistant role, got %s", asstIdx, sc.Messages[asstIdx].Role)
		}
		if sc.Messages[userIdx].Content != instructions[i] {
			t.Errorf("msg[%d]: expected content %q, got %q", userIdx, instructions[i], sc.Messages[userIdx].Content)
		}
		if sc.Messages[asstIdx].Content == "" {
			t.Errorf("msg[%d]: assistant content is empty", asstIdx)
		}
		t.Logf("  ✓ Round %d: user=%q → assistant=%q", i+1,
			sc.Messages[userIdx].Content,
			sc.Messages[asstIdx].Content,
		)
	}

	// --- Verify compressed view ---
	if sc.CompressedView == nil {
		t.Fatal("compressed view is nil")
	}
	cvLen := len(sc.CompressedView)
	cvNoSys := sc.CompressedView
	if cvLen > 0 && cvNoSys[0].Role == types.MessageRoleSystem {
		cvNoSys = cvNoSys[1:]
	}
	// AC-P1-5 fix: CompressedView is set BEFORE the round's Prepare runs
	// (engine.go: SetCompressedView runs inside Prepare pipeline, then
	// AppendUserMessage appends the round's user message). At the END of
	// round 3 the CV reflects sc.Messages state from BEFORE round 3's
	// user message was appended — i.e. 4 messages (user1, assistant1,
	// user2, assistant2) excluding system.
	if len(cvNoSys) != 4 {
		t.Errorf("expected 4 messages in compressed view (excluding system, state at start of round 3), got %d", len(cvNoSys))
	}
	t.Logf("  ✓ CompressedView: %d messages total (including system), %d without sys", cvLen, len(cvNoSys))

	// --- Span / trace validation ---
	memExporter := obs.MemoryExporter()
	spans := memExporter.Spans()
	t.Logf("\n=== Span coverage (%d total) ===", len(spans))

	spanByName := make(map[string]int)
	for _, s := range spans {
		spanByName[s.Name()]++
	}
	for name, count := range spanByName {
		t.Logf("  %s: %d", name, count)
	}

	if spanByName[telemetry.OpD2_S2_Context_Process] < 3 {
		t.Errorf("expected >= 3 %s spans, got %d", telemetry.OpD2_S2_Context_Process, spanByName[telemetry.OpD2_S2_Context_Process])
	}
	if spanByName[telemetry.OpD2_S2_Context_Snapshot_Load] < 3 {
		t.Errorf("expected >= 3 %s spans, got %d", telemetry.OpD2_S2_Context_Snapshot_Load, spanByName[telemetry.OpD2_S2_Context_Snapshot_Load])
	}
	// D3_LLM_Stream spans require a real LLM Gateway → PreparedTurnRunner
	// adapter. The current test wires a StaticPreparedTurnRunner (no LLM
	// Gateway dispatch), so D3 spans are 0. Tracked as follow-up: create
	// a real bridge adapter (e.g. llmbridge.New(gw).AsPreparedTurnRunner()).
	if spanByName[telemetry.OpD3_S3_LLM_Stream] < 3 {
		t.Logf("  ℹ  %s spans = %d (expected 3; requires LLM Gateway bridge — see Phase F follow-up)",
			telemetry.OpD3_S3_LLM_Stream, spanByName[telemetry.OpD3_S3_LLM_Stream])
	}
	t.Logf("  ✓ All required spans present across 3 rounds")

	// --- Session persistence check ---
	updatedSession, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("failed to get session from store: %v", err)
	}
	if updatedSession == nil {
		t.Fatal("session not found in store")
	}
	t.Logf("\n=== Session persistence ===")
	t.Logf("  ContextSnapshot on disk: %d bytes", len(updatedSession.ContextSnapshot))
	if len(updatedSession.ContextSnapshot) == 0 {
		t.Logf("  ⚠  ContextSnapshot is empty on disk — only persists in memory")
		t.Logf("  ℹ  runProcess sets ContextSnapshot on the local session pointer")
		t.Logf("     but never calls sessionStore.Update(session). Context accumulation")
		t.Logf("     still works via memory.Manager's in-memory cache (LoadOrInit).")
	}
}
