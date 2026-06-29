package prepare_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubSessionLoader struct {
	sc      *types.SessionContext
	isNew   bool
	err     error
	called  bool
	gotCtx  context.Context
}

func (s *stubSessionLoader) LoadOrInit(ctx context.Context, _ *types.Session, _ string) (*types.SessionContext, bool, error) {
	s.called = true
	s.gotCtx = ctx
	return s.sc, s.isNew, s.err
}

type stubMemoryRecaller struct {
	entries []memory.MemoryEntry
	err     error
	called  bool
}

func (s *stubMemoryRecaller) RecallLongTermEntries(_ context.Context, _ string) ([]memory.MemoryEntry, error) {
	s.called = true
	return s.entries, s.err
}

type stubCompressor struct {
	shouldCompress bool
	compressed     []types.Message
	err            error
	called         bool
}

func (s *stubCompressor) ShouldCompress(_ []types.Message, _ types.TokenBudget) bool {
	return s.shouldCompress
}

func (s *stubCompressor) Run(_ context.Context, _ []types.Message, _ string, _ types.TokenBudget) ([]types.Message, types.CompressionReport, error) {
	s.called = true
	return s.compressed, types.CompressionReport{}, s.err
}

type stubPromptAssembler struct {
	prompt string
	called bool
	gotCtx context.Context
}

func (s *stubPromptAssembler) Build(ctx context.Context, _ prompt.SystemPromptBuildInput) (string, prompt.SystemPromptBuildReport) {
	s.called = true
	s.gotCtx = ctx
	return s.prompt, prompt.SystemPromptBuildReport{}
}

// noopSpanStarter returns ctx unchanged with no span. Used for tests where
// observability is not exercised.
func noopSpanStarter(ctx context.Context, _ string, _ any, _ ...any) (context.Context, any) {
	return ctx, nil
}

// T: D2-S15-A01-T61
func TestPrepareOrchestrator_Prepare_loads_session(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	loader := &stubSessionLoader{sc: sc, isNew: true}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader: loader,
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
		Model:   "claude-sonnet-4-6",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !loader.called {
		t.Error("expected SessionLoader to be called")
	}
	if output.SessionContext.SessionID != "s1" {
		t.Fatalf("expected session 's1', got %q", output.SessionContext.SessionID)
	}
	if !output.IsNewSession {
		t.Error("expected IsNewSession=true when loader reports isNew=true")
	}
}

// T: D2-S15-A01-T62
func TestPrepareOrchestrator_Prepare_recalls_memory_for_non_worker(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	entries := []memory.MemoryEntry{{ID: "m1", Topic: "k1", Content: "v1"}}
	loader := &stubSessionLoader{sc: sc}
	recaller := &stubMemoryRecaller{entries: entries}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:  loader,
		MemoryRecaller: recaller,
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:     &types.Session{SessionID: "s1"},
		Message:     "hello",
		WorkerLocal: false,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recaller.called {
		t.Error("expected MemoryRecaller to be called for non-worker")
	}
	if len(output.MemoryEntries) != 1 {
		t.Fatalf("expected 1 memory entry, got %d", len(output.MemoryEntries))
	}
}

// T: D2-S15-A01-T63
func TestPrepareOrchestrator_Prepare_skips_memory_for_worker(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	loader := &stubSessionLoader{sc: sc}
	recaller := &stubMemoryRecaller{entries: []memory.MemoryEntry{{ID: "m1"}}}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:  loader,
		MemoryRecaller: recaller,
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:     &types.Session{SessionID: "s1"},
		WorkerLocal: true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recaller.called {
		t.Error("expected MemoryRecaller NOT to be called for worker-local")
	}
	if len(output.MemoryEntries) != 0 {
		t.Fatalf("expected 0 memory entries for worker, got %d", len(output.MemoryEntries))
	}
}

// T: D2-S15-A01-T64
func TestPrepareOrchestrator_Prepare_compresses_when_over_budget_and_allowed(t *testing.T) {
	original := []types.Message{{Role: types.MessageRoleUser, Content: "long message"}}
	compressed := []types.Message{{Role: types.MessageRoleUser, Content: "short"}}
	sc := &types.SessionContext{SessionID: "s1", Messages: original}
	loader := &stubSessionLoader{sc: sc}
	comp := &stubCompressor{
		shouldCompress: true,
		compressed:     compressed,
	}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader: loader,
		Compressor:    comp,
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:         &types.Session{SessionID: "s1"},
		CompressPerTurn: true,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !comp.called {
		t.Error("expected Compressor.Run to be called when CompressPerTurn=true")
	}
	if len(output.Messages) != 1 || output.Messages[0].Content != "short" {
		t.Fatalf("expected compressed messages, got %v", output.Messages)
	}
}

// T: D2-S15-A01-T65
func TestPrepareOrchestrator_Prepare_skips_compression_when_disallowed(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{{Role: types.MessageRoleUser, Content: "x"}}}
	comp := &stubCompressor{shouldCompress: true}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader: &stubSessionLoader{sc: sc},
		Compressor:    comp,
	})
	_, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:         &types.Session{SessionID: "s1"},
		CompressPerTurn: false,
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.called {
		t.Error("expected Compressor.Run NOT to be called when CompressPerTurn=false")
	}
}

// T: D2-S15-A01-T66
func TestPrepareOrchestrator_Prepare_assembles_prompt(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	assembler := &stubPromptAssembler{prompt: "You are helpful."}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:   &stubSessionLoader{sc: sc},
		PromptAssembler: assembler,
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !assembler.called {
		t.Error("expected PromptAssembler to be called")
	}
	if output.SystemPrompt != "You are helpful." {
		t.Fatalf("expected prompt, got %q", output.SystemPrompt)
	}
}

// T: D2-S15-A01-T67
func TestNewPrepareOrchestrator_nil_deps_does_not_panic(t *testing.T) {
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{})
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
}

// T: D2-S15-A01-T68
func TestPrepareOrchestrator_Prepare_load_error_returns_error(t *testing.T) {
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader: &stubSessionLoader{err: errSentinel},
	})
	_, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
	}, nil)
	if err == nil {
		t.Fatal("expected error when SessionLoader fails")
	}
}

// T: D2-S15-A01-T69
func TestPrepareOrchestrator_Hooks_BeforeLoad_AfterLoad_AfterPrepare(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	var beforeLoad, afterLoad, afterPrepare bool
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:   &stubSessionLoader{sc: sc},
		PromptAssembler: &stubPromptAssembler{prompt: "hi"},
	}).WithHooks(prepare.PrepareHooks{
		BeforeLoad: func(_ context.Context, _ *prepare.PrepareInput, _ *types.SessionContext) {
			beforeLoad = true
		},
		AfterLoad: func(_ context.Context, _ *prepare.PrepareInput, _ *types.SessionContext) {
			afterLoad = true
		},
		AfterPrepare: func(_ context.Context, _ *prepare.PrepareInput, _ *prepare.PrepareOutput) {
			afterPrepare = true
		},
	})
	_, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !beforeLoad || !afterLoad || !afterPrepare {
		t.Errorf("hooks not all fired: BeforeLoad=%v AfterLoad=%v AfterPrepare=%v",
			beforeLoad, afterLoad, afterPrepare)
	}
}

// errSentinel is a non-nil dummy error for stub tests.
var errSentinel = sentinelErr("session load failed")

type sentinelErr string

func (e sentinelErr) Error() string { return string(e) }

// T: D2-S15-A01-T70 (DM-20260621-005) — orchestrator forwards its incoming
// ctx to SessionLoader so the A01 D2_Context_Snapshot_Load span attaches
// under the parent's span context (was previously context.Background()).
func TestPrepareOrchestrator_Prepare_forwards_ctx_to_session_loader(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	loader := &stubSessionLoader{sc: sc}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{SessionLoader: loader})

	type ctxKey struct{}
	parent := context.WithValue(context.Background(), ctxKey{}, "marker")
	_, err := orch.Prepare(parent, prepare.PrepareInput{Session: &types.Session{SessionID: "s1"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loader.gotCtx == nil {
		t.Fatal("expected SessionLoader to receive non-nil ctx")
	}
	if v, _ := loader.gotCtx.Value(ctxKey{}).(string); v != "marker" {
		t.Fatalf("ctx value lost: got %q, want %q", v, "marker")
	}
}

// T: D2-S15-A01-T71 (DM-20260621-005) — orchestrator forwards its incoming
// ctx to PromptAssembler so the A04 D2_Context_Harness_SystemPrompt_Build
// span attaches under the parent's span context (was previously
// context.Background()).
func TestPrepareOrchestrator_Prepare_forwards_ctx_to_prompt_assembler(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	assembler := &stubPromptAssembler{prompt: "hi"}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:   &stubSessionLoader{sc: sc},
		PromptAssembler: assembler,
	})

	type ctxKey struct{}
	parent := context.WithValue(context.Background(), ctxKey{}, "marker")
	_, err := orch.Prepare(parent, prepare.PrepareInput{Session: &types.Session{SessionID: "s1"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assembler.gotCtx == nil {
		t.Fatal("expected PromptAssembler to receive non-nil ctx")
	}
	if v, _ := assembler.gotCtx.Value(ctxKey{}).(string); v != "marker" {
		t.Fatalf("ctx value lost: got %q, want %q", v, "marker")
	}
}

type capturingPromptAssembler struct {
	lastIn prompt.SystemPromptBuildInput
}

func (s *capturingPromptAssembler) Build(_ context.Context, in prompt.SystemPromptBuildInput) (string, prompt.SystemPromptBuildReport) {
	s.lastIn = in
	a := prompt.NewSystemPromptAssembler(config.DefaultWorkspacePromptConfig())
	return a.Build(in)
}

func TestPrepareOrchestrator_Prepare_passesAgentsRawToAssembler(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", WorkDir: "/tmp/ws", Messages: []types.Message{}}
	cap := &capturingPromptAssembler{}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:   &stubSessionLoader{sc: sc},
		PromptAssembler: cap,
	})
	agents := "D2 → internal/layers/contextengine/"
	out, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:         &types.Session{SessionID: "s1"},
		Message:         "review d2",
		AgentsRaw:       agents,
		UserContextMode: "system",
	}, nil)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if cap.lastIn.AgentsRaw != agents {
		t.Fatalf("AgentsRaw = %q, want %q", cap.lastIn.AgentsRaw, agents)
	}
	if cap.lastIn.OmitAgentsFromSystem {
		t.Fatal("expected agents in system for mode=system")
	}
	if !strings.Contains(out.SystemPrompt, "contextengine") {
		t.Fatalf("system prompt missing agents: %q", out.SystemPrompt)
	}
}