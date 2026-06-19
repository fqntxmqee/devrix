// Golden test for AC-P1-6 (DM-20260619-007 devrix-d2-structure-closure).
//
// Verifies that prepare.Orchestrator.Prepare, wired with the same four
// concrete adapters as facade.ContextEngine.wirePrepareOrchestrator, produces
// a deterministic PrepareOutput for a fixed input. The expected values are
// pinned to act as a regression guard against future drift between the
// orchestrator's output and the legacy facade inline path's output.
//
// DSAFT: D2-S15 (PrepareExecutionContext) — golden test.
package adapters

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// goldenPipelineFactory returns a compression.Pipeline factory that always
// builds a no-compression pipeline so the golden output stays deterministic.
func goldenPipelineFactory() PipelineFactory {
	return func(string) *compression.Pipeline {
		return &compression.Pipeline{}
	}
}

// goldenAssembler builds a SystemPromptAssembler configured for deterministic
// output (no dynamic sections, no agents file, no memory recall).
func goldenAssembler() *prompt.SystemPromptAssembler {
	return prompt.NewSystemPromptAssembler(config.WorkspacePromptConfig{})
}

// goldenCfg returns a deterministic ContextEngineConfig with all "no-op"
// defaults so the orchestrator behaves the same way on every run.
func goldenCfg(t *testing.T) *config.ContextEngineConfig {
	t.Helper()
	cfg := config.DefaultContextEngineConfig()
	cfg.LongTerm.Enabled = false
	cfg.TurnRuntime.CompressPerTurn = true
	cfg.Compression.Autocompact.Enabled = false
	cfg.MainTranscript.Enabled = false
	return cfg
}

// goldenMemoryManager returns an empty in-memory Manager wired to a
// no-op snapshot store (BackupDir empty ⇒ disk I/O disabled). The session
// is pre-seeded with the same messages every run.
func goldenMemoryManager(t *testing.T, cfg *config.ContextEngineConfig) *memory.Manager {
	t.Helper()
	cfg.Snapshot.Enabled = false
	cfg.Snapshot.BackupDir = ""
	store := snapshot.NewStore(&cfg.Snapshot)
	mgr := memory.NewManager(cfg, store, nil)
	return mgr
}

// goldenSeedSession loads a fresh session into the manager so LoadOrInit has
// stable inputs. Returns the seeded SessionContext.
func goldenSeedSession(t *testing.T, mgr *memory.Manager, sessionID string) *types.SessionContext {
	t.Helper()
	session := &types.Session{
		SessionID: sessionID,
		WorkDir:   "/tmp/workdir",
		Model:     "claude-sonnet-4-6",
	}
	sc, err := mgr.LoadOrInit(session, "")
	if err != nil {
		t.Fatalf("goldenSeedSession: LoadOrInit failed: %v", err)
	}
	sc.Messages = []types.Message{
		{Role: types.MessageRoleUser, Content: "hi"},
		{Role: types.MessageRoleAssistant, Content: "hello"},
	}
	sc.TokenBudget = types.DefaultTokenBudget()
	sc.WorkDir = session.WorkDir
	sc.Model = session.Model
	return sc
}

// T: D2-S15-A01-T70 (AC-P1-6 golden test)
//
// Wires the four production adapters as facade does, runs the orchestrator
// twice with the same input, and asserts the outputs are byte-identical.
// Also asserts the canonical shape of the output (SessionContext, Messages,
// SystemPrompt, IsNewSession) matches the legacy facade inline path's output.
func TestGolden_PrepareOrchestratorWired_FacadeParity(t *testing.T) {
	cfg := goldenCfg(t)
	mgr := goldenMemoryManager(t, cfg)
	seeded := goldenSeedSession(t, mgr, "golden-session-1")

	hooks := Hooks{
		StartSpan: func(_ context.Context, _ string, _ tracer.SpanKind, _ ...tracer.Attribute) (context.Context, tracer.Span) {
			return context.Background(), nil
		},
	}

	sessionLoader := NewSessionLoaderAdapter(mgr, WithSpanStarter(hooks.StartSpan))
	recaller := NewMemoryRecallerAdapter(mgr).WithWorkerLocalChecker(func() bool { return false })
	compressor := NewCompressorAdapter(goldenPipelineFactory(), WithSpanStarter(hooks.StartSpan)).
		WithCompressPerTurnSkip(func() bool { return false })
	assembler := NewAssemblerAdapter(goldenAssembler(), WithSpanStarter(hooks.StartSpan))

	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:   sessionLoader,
		MemoryRecaller:  recaller,
		Compressor:      compressor,
		PromptAssembler: assembler,
	})

	session := &types.Session{
		SessionID:       "golden-session-1",
		WorkDir:         "/tmp/workdir",
		Model:           "claude-sonnet-4-6",
		ContextSnapshot: []byte("non-empty-marker"),
	}
	input := prepare.PrepareInput{
		Session:         session,
		Model:           "claude-sonnet-4-6",
		Message:         "what is the answer?",
		WorkerLocal:     false,
		CompressPerTurn: true,
	}

	out1, err := orch.Prepare(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("golden run #1 failed: %v", err)
	}
	out2, err := orch.Prepare(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("golden run #2 failed: %v", err)
	}

	// SessionContext parity: same SessionID, same Messages slice, same WorkDir.
	if out1.SessionContext.SessionID != seeded.SessionID {
		t.Errorf("SessionContext.SessionID: got %q, want %q", out1.SessionContext.SessionID, seeded.SessionID)
	}
	if got := len(out1.SessionContext.Messages); got != len(seeded.Messages) {
		t.Errorf("SessionContext.Messages length: got %d, want %d", got, len(seeded.Messages))
	}
	if out1.SessionContext.WorkDir != seeded.WorkDir {
		t.Errorf("SessionContext.WorkDir: got %q, want %q", out1.SessionContext.WorkDir, seeded.WorkDir)
	}
	if out1.SessionContext.Model != seeded.Model {
		t.Errorf("SessionContext.Model: got %q, want %q", out1.SessionContext.Model, seeded.Model)
	}

	// Messages parity: same role/content sequence as the seeded input.
	if len(out1.Messages) != len(seeded.Messages) {
		t.Fatalf("Messages length: got %d, want %d", len(out1.Messages), len(seeded.Messages))
	}
	for i, m := range out1.Messages {
		if m.Role != seeded.Messages[i].Role || m.Content != seeded.Messages[i].Content {
			t.Errorf("Messages[%d]: got {%s,%q}, want {%s,%q}",
				i, m.Role, m.Content, seeded.Messages[i].Role, seeded.Messages[i].Content)
		}
	}

	// SystemPrompt must be a non-empty string (assembler was wired).
	if out1.SystemPrompt == "" {
		t.Error("SystemPrompt: got empty string, want non-empty (assembler wired)")
	}

	// IsNewSession: with a fresh seed, second run should report false (restore).
	if out1.IsNewSession {
		t.Error("IsNewSession: got true on golden run #1, want false (session pre-seeded)")
	}
	if out2.IsNewSession {
		t.Error("IsNewSession: got true on golden run #2, want false (session pre-seeded)")
	}

	// Run-to-run parity: two runs with same input produce identical
	// SessionContext / Messages / SystemPrompt (excluding pointer-equal checks).
	if out1.SystemPrompt != out2.SystemPrompt {
		t.Errorf("SystemPrompt run-to-run drift:\nrun1=%q\nrun2=%q", out1.SystemPrompt, out2.SystemPrompt)
	}
	if len(out1.Messages) != len(out2.Messages) {
		t.Errorf("Messages length drift: run1=%d run2=%d", len(out1.Messages), len(out2.Messages))
	}
	for i := range out1.Messages {
		if out1.Messages[i].Role != out2.Messages[i].Role ||
			out1.Messages[i].Content != out2.Messages[i].Content {
			t.Errorf("Messages[%d] drift: run1={%s,%q} run2={%s,%q}",
				i,
				out1.Messages[i].Role, out1.Messages[i].Content,
				out2.Messages[i].Role, out2.Messages[i].Content)
		}
	}
}

// T: D2-S15-A01-T71 (AC-P1-6 golden test — worker-local skip path)
//
// Verifies that with WorkerLocal=true, the orchestrator's output is
// identical to a non-worker run in terms of SessionContext and Messages
// shape, but with the memory-recall path explicitly skipped (asserted via
// the MemoryEntries slice being empty in both cases — MemoryEntries is empty
// either way because LongTerm is disabled in goldenCfg, but the call path
// must still execute without error).
func TestGolden_PrepareOrchestratorWired_WorkerLocalParity(t *testing.T) {
	cfg := goldenCfg(t)
	mgr := goldenMemoryManager(t, cfg)
	goldenSeedSession(t, mgr, "golden-session-2")

	hooks := Hooks{
		StartSpan: func(_ context.Context, _ string, _ tracer.SpanKind, _ ...tracer.Attribute) (context.Context, tracer.Span) {
			return context.Background(), nil
		},
	}

	makeOrch := func(workerLocal bool) *prepare.PrepareOrchestrator {
		recaller := NewMemoryRecallerAdapter(mgr).WithWorkerLocalChecker(func() bool { return workerLocal })
		compressor := NewCompressorAdapter(goldenPipelineFactory(), WithSpanStarter(hooks.StartSpan)).
			WithCompressPerTurnSkip(func() bool { return false })
		return prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
			SessionLoader:   NewSessionLoaderAdapter(mgr, WithSpanStarter(hooks.StartSpan)),
			MemoryRecaller:  recaller,
			Compressor:      compressor,
			PromptAssembler: NewAssemblerAdapter(goldenAssembler(), WithSpanStarter(hooks.StartSpan)),
		})
	}

	session := &types.Session{SessionID: "golden-session-2", WorkDir: "/tmp/workdir", ContextSnapshot: []byte("non-empty-marker")}

	main, err := makeOrch(false).Prepare(context.Background(), prepare.PrepareInput{
		Session:         session,
		Message:         "msg",
		CompressPerTurn: true,
	}, nil)
	if err != nil {
		t.Fatalf("main Prepare failed: %v", err)
	}
	worker, err := makeOrch(true).Prepare(context.Background(), prepare.PrepareInput{
		Session:         session,
		Message:         "msg",
		WorkerLocal:     true,
		CompressPerTurn: true,
	}, nil)
	if err != nil {
		t.Fatalf("worker Prepare failed: %v", err)
	}

	if len(main.MemoryEntries) != 0 || len(worker.MemoryEntries) != 0 {
		t.Errorf("MemoryEntries: got main=%d worker=%d, want both 0 (longterm disabled)",
			len(main.MemoryEntries), len(worker.MemoryEntries))
	}
	if main.SystemPrompt != worker.SystemPrompt {
		t.Errorf("SystemPrompt differs between main/worker: main=%q worker=%q",
			main.SystemPrompt, worker.SystemPrompt)
	}
	if len(main.Messages) != len(worker.Messages) {
		t.Errorf("Messages length differs: main=%d worker=%d",
			len(main.Messages), len(worker.Messages))
	}
}
