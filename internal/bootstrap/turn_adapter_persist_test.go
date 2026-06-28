package bootstrap

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/kernel"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/registry"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func mustBuiltinRegistryForAdapter(t *testing.T) *registry.BuiltinRegistry {
	t.Helper()
	reg, err := registry.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

// T: D7-S5-A04-T01 (DM-20260617-003 devrix-d7-turn-history-persist)
// TestPersistTurn_WritesMessagesToD2Memory: with a real ContextEngine,
// PersistTurn should append req.Messages to the in-memory SessionContext
// so a subsequent Prepare returns the full history.
func TestPersistTurn_WritesMessagesToD2Memory(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{Output: "ok"},
		ToolsReg:           mustBuiltinRegistryForAdapter(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             cfg,
	})

	adapter := newContextEngineAdapter(gw, engine, nil)
	sid := "sess-persist-1"

	// Turn 1
	if err := adapter.PersistTurn(context.Background(), sessionorchestrator.PersistRequest{
		SessionID: sid,
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "记住数字 42", SessionID: sid},
			{Role: types.MessageRoleAssistant, Content: "好的，已记住 42", SessionID: sid},
		},
		TurnCount: 1,
	}); err != nil {
		t.Fatalf("PersistTurn turn1: %v", err)
	}

	// Prepare should now return the 2 messages as history.
	prepared, err := adapter.Prepare(context.Background(), sessionorchestrator.PrepareRequest{
		SessionID: sid,
		Message:   types.Message{Role: types.MessageRoleUser, Content: "我刚才让你记的数字是几？"},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(prepared.Messages) != 2 {
		t.Fatalf("Prepare messages len = %d, want 2", len(prepared.Messages))
	}
	if prepared.Messages[0].Content != "记住数字 42" {
		t.Errorf("history[0]: got %q, want %q", prepared.Messages[0].Content, "记住数字 42")
	}
	if prepared.Messages[1].Content != "好的，已记住 42" {
		t.Errorf("history[1]: got %q, want %q", prepared.Messages[1].Content, "好的，已记住 42")
	}
}

// T: D7-S5-A04-T01 — multi-turn round-trip end-to-end.
func TestPersistTurn_FullRound_ThreeTurns(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{Output: "ok"},
		ToolsReg:           mustBuiltinRegistryForAdapter(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             cfg,
	})
	adapter := newContextEngineAdapter(gw, engine, nil)
	sid := "sess-3turns"

	turns := []struct {
		user, asst string
	}{
		{"记住数字 42", "已记 42"},
		{"再记颜色蓝色", "已记 42 + 蓝色"},
		{"我两个秘密分别是什么？", "42 和蓝色"},
	}
	for i, tn := range turns {
		if err := adapter.PersistTurn(context.Background(), sessionorchestrator.PersistRequest{
			SessionID: sid,
			Messages: []types.Message{
				{Role: types.MessageRoleUser, Content: tn.user, SessionID: sid},
				{Role: types.MessageRoleAssistant, Content: tn.asst, SessionID: sid},
			},
			TurnCount: i + 1,
		}); err != nil {
			t.Fatalf("PersistTurn turn%d: %v", i+1, err)
		}
	}

	// After 3 turns, history should have 6 messages.
	prepared, err := adapter.Prepare(context.Background(), sessionorchestrator.PrepareRequest{
		SessionID: sid,
		Message:   types.Message{Role: types.MessageRoleUser, Content: "follow up"},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(prepared.Messages) != 6 {
		t.Fatalf("Prepare messages len = %d, want 6 (3 turns × 2)", len(prepared.Messages))
	}
}

// T: D7-S5-A04-T01 — nil engine must not panic (adapter used in tests/mocks).
func TestPersistTurn_NilEngine(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	// pass a stub that is NOT *kernel.ContextEngine — adapter must no-op.
	adapter := newContextEngineAdapter(gw, &stubSessionEngine{}, nil)
	if err := adapter.PersistTurn(context.Background(), sessionorchestrator.PersistRequest{
		SessionID: "sess-nil-engine",
		Messages:  []types.Message{{Role: types.MessageRoleUser, Content: "x"}},
	}); err != nil {
		t.Errorf("PersistTurn with non-ContextEngine: expected nil, got %v", err)
	}
}

// T: D7-S5-A04-T01 — race: per-session concurrency within the engine should
// not race. (Same-session concurrent writes are serialized by the engine's
// memory manager.)
func TestPersistTurn_NoPanic_Sequential(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{Output: "ok"},
		ToolsReg:           mustBuiltinRegistryForAdapter(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             cfg,
	})
	adapter := newContextEngineAdapter(gw, engine, nil)

	var wg sync.WaitGroup
	const goroutines = 20
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			sid := "sess-conc-" + string(rune('A'+i))
			_ = adapter.PersistTurn(context.Background(), sessionorchestrator.PersistRequest{
				SessionID: sid,
				Messages: []types.Message{
					{Role: types.MessageRoleUser, Content: "u", SessionID: sid},
				},
			})
		}()
	}
	wg.Wait()
}

// T: D7-S2-A06-T02 (DM-20260617-004 devrix-d7-tool-ctx-inject)
// ExecuteRound must inject the live SessionContext (WorkDir + SessionID) into
// the per-tool ctx, mirroring D2 queryloop's WrapToolContext. Without this,
// permission-aware tools (delegate_status, task_output, task_list_background)
// report "session context unavailable" / "session_id unavailable" under the
// LoopFirst routing path.
func TestExecuteRound_AttachesSessionContext(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	cfg.Snapshot.Enabled = false
	realReg, err := tools.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("real reg: %v", err)
	}
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              realReg,
		ToolsReg:           mustBuiltinRegistryForAdapter(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             cfg,
	})
	adapter := newContextEngineAdapter(gw, engine, nil)
	sid := "sess-ctx-inject"

	// Seed sc with a real workdir so bash `pwd` returns it.
	workDir := t.TempDir()
	session := types.NewSession(sid, "cli", workDir)
	ch := engine.Process(context.Background(), session, "warmup")
	for range ch {
	}
	sc, ok := engine.SessionContext(sid)
	if !ok {
		t.Fatalf("precondition: sc not created for %s", sid)
	}
	// sc.WorkDir is what bash will chdir into.
	wantWorkDir := sc.WorkDir
	if _, err := os.Stat(wantWorkDir); err != nil {
		t.Fatalf("sc.WorkDir does not exist: %v", err)
	}

	// Call bash via ExecuteRound; bash reads workdir from ctx. If sc is not
	// injected, bash falls back to os.Getwd() (the test process cwd) and the
	// output will NOT contain workDir.
	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: sid,
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "call-bash-1",
			Name:  "bash",
			Input: `{"command":"pwd"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("ExecuteRound results len = %d, want 1", len(res.Results))
	}
	if res.Results[0].Error != "" {
		t.Fatalf("bash error: %s", res.Results[0].Error)
	}
	if !strings.Contains(res.Results[0].Output, wantWorkDir) {
		t.Errorf("bash output %q does not contain workDir %q — sc not injected into ctx",
			res.Results[0].Output, wantWorkDir)
	}
}

// T: D7-S2-A06-T02 — ExecuteRound with no matching sc must not panic and
// must still execute the tool (falling back to os.Getwd()).

// T: D7-S2-A06-T03 (DM-20260621-005 devrix-d7-d2-prepare-wire)
// When the engine is *kernel.ContextEngine, Prepare must route through
// ContextEngine.PrepareForTurn → D2 PrepareOrchestrator. The orchestrator
// runs A04 AssemblePrompt, which produces a non-empty SystemPrompt. Before
// this wire, Prepare copied sc.Messages directly without invoking the
// orchestrator, so SystemPrompt was always empty.
func TestPrepareForTurn_RoutesThroughPrepareOrchestrator(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              &enforce.ToolRunner{Output: "ok"},
		ToolsReg:           mustBuiltinRegistryForAdapter(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             cfg,
	})
	adapter := newContextEngineAdapter(gw, engine, nil)
	sid := "sess-prepare-wire"

	if err := adapter.PersistTurn(context.Background(), sessionorchestrator.PersistRequest{
		SessionID: sid,
		Messages: []types.Message{
			{Role: types.MessageRoleUser, Content: "ping", SessionID: sid},
			{Role: types.MessageRoleAssistant, Content: "pong", SessionID: sid},
		},
		TurnCount: 1,
	}); err != nil {
		t.Fatalf("PersistTurn: %v", err)
	}

	prepared, err := adapter.Prepare(context.Background(), sessionorchestrator.PrepareRequest{
		SessionID: sid,
		Message:   types.Message{Role: types.MessageRoleUser, Content: "follow"},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.SystemPrompt == "" {
		t.Fatal("expected non-empty SystemPrompt when Prepare routes through PrepareOrchestrator (A04 AssemblePrompt)")
	}
}
func TestExecuteRound_NoSessionContext_StillExecutes(t *testing.T) {
	store, err := capture.NewFileSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("session store: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	cfg.Snapshot.Enabled = false
	realReg, err := tools.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("real reg: %v", err)
	}
	engine := kernel.NewContextEngine(kernel.EngineDeps{
		PreparedTurnRunner: &contextengine.StaticPreparedTurnRunner{Response: "ok"},
		Summarizer:         &contextengine.StaticSummarizer{},
		Tools:              realReg,
		ToolsReg:           mustBuiltinRegistryForAdapter(t),
		Permission:         enforce.AllowAllPermission{},
		Config:             cfg,
	})
	adapter := newContextEngineAdapter(gw, engine, nil)

	// No Process() call → no sc in memory.
	res, err := adapter.ExecuteRound(context.Background(), sessionorchestrator.ToolRoundRequest{
		SessionID: "sess-no-sc",
		ToolCalls: []llmgateway.ToolCall{{
			ID:    "call-bash-noop",
			Name:  "bash",
			Input: `{"command":"echo hi"}`,
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteRound: %v", err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("ExecuteRound results len = %d, want 1", len(res.Results))
	}
	// bash should still run (falls back to os.Getwd) — output contains "hi".
	if !strings.Contains(res.Results[0].Output, "hi") {
		t.Errorf("bash output without sc: %q (expected to contain 'hi')", res.Results[0].Output)
	}
}
