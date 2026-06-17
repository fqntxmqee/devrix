package bootstrap

import (
	"context"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/registry"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
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
	gw := capture.NewCommunicationGateway(store, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: &mockctx.StaticLLMCaller{Response: "ok"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:          &mockctx.ToolRunner{Output: "ok"},
		ToolsReg:       mustBuiltinRegistryForAdapter(t),
		Permission:     mockctx.AllowAllPermission{},
		Config:         cfg,
	})

	adapter := newContextEngineAdapter(gw, engine, nil)
	sid := "sess-persist-1"

	// Turn 1
	if err := adapter.PersistTurn(context.Background(), turn.PersistRequest{
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
	prepared, err := adapter.Prepare(context.Background(), turn.PrepareRequest{
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
	gw := capture.NewCommunicationGateway(store, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: &mockctx.StaticLLMCaller{Response: "ok"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:          &mockctx.ToolRunner{Output: "ok"},
		ToolsReg:       mustBuiltinRegistryForAdapter(t),
		Permission:     mockctx.AllowAllPermission{},
		Config:         cfg,
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
		if err := adapter.PersistTurn(context.Background(), turn.PersistRequest{
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
	prepared, err := adapter.Prepare(context.Background(), turn.PrepareRequest{
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
	gw := capture.NewCommunicationGateway(store, nil, nil, nil)

	// pass a stub that is NOT *contextengine.ContextEngine — adapter must no-op.
	adapter := newContextEngineAdapter(gw, &stubSessionEngine{}, nil)
	if err := adapter.PersistTurn(context.Background(), turn.PersistRequest{
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
	gw := capture.NewCommunicationGateway(store, nil, nil, nil)

	cfg := config.DefaultContextEngineConfig()
	cfg.Compression.Autocompact.Enabled = false
	engine := contextengine.NewContextEngine(contextengine.EngineDeps{
		QueryLLMCaller: &mockctx.StaticLLMCaller{Response: "ok"},
		Summarizer:     &mockctx.StaticSummarizer{},
		Tools:          &mockctx.ToolRunner{Output: "ok"},
		ToolsReg:       mustBuiltinRegistryForAdapter(t),
		Permission:     mockctx.AllowAllPermission{},
		Config:         cfg,
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
			_ = adapter.PersistTurn(context.Background(), turn.PersistRequest{
				SessionID: sid,
				Messages: []types.Message{
					{Role: types.MessageRoleUser, Content: "u", SessionID: sid},
				},
			})
		}()
	}
	wg.Wait()
}
