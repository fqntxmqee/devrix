package prepare_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubSessionLoader struct {
	sc  *types.SessionContext
	err error
}

func (s *stubSessionLoader) LoadOrInit(_ *types.Session, _ string) (*types.SessionContext, error) {
	return s.sc, s.err
}

type stubMemoryRecaller struct {
	entries []memory.MemoryEntry
	err     error
}

func (s *stubMemoryRecaller) RecallLongTermEntries(_ context.Context, _ string) ([]memory.MemoryEntry, error) {
	return s.entries, s.err
}

type stubCompressor struct {
	shouldCompress bool
	compressed     []types.Message
	err            error
}

func (s *stubCompressor) ShouldCompress(_ []types.Message, _ types.TokenBudget) bool {
	return s.shouldCompress
}

func (s *stubCompressor) Run(_ context.Context, _ []types.Message, _ string, _ types.TokenBudget) ([]types.Message, types.CompressionReport, error) {
	return s.compressed, types.CompressionReport{}, s.err
}

type stubPromptAssembler struct {
	prompt string
}

func (s *stubPromptAssembler) Build(_ prompt.SystemPromptBuildInput) (string, prompt.SystemPromptBuildReport) {
	return s.prompt, prompt.SystemPromptBuildReport{}
}

// T: D2-S15-A01-T61
func TestPrepareOrchestrator_Prepare_loads_session(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader: &stubSessionLoader{sc: sc},
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
		Model:   "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.SessionContext.SessionID != "s1" {
		t.Fatalf("expected session 's1', got %q", output.SessionContext.SessionID)
	}
}

// T: D2-S15-A01-T62
func TestPrepareOrchestrator_Prepare_recalls_memory_for_non_worker(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	entries := []memory.MemoryEntry{{ID: "m1", Topic: "k1", Content: "v1"}}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:  &stubSessionLoader{sc: sc},
		MemoryRecaller: &stubMemoryRecaller{entries: entries},
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:     &types.Session{SessionID: "s1"},
		Message:     "hello",
		WorkerLocal: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.MemoryEntries) != 1 {
		t.Fatalf("expected 1 memory entry, got %d", len(output.MemoryEntries))
	}
}

// T: D2-S15-A01-T63
func TestPrepareOrchestrator_Prepare_skips_memory_for_worker(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:  &stubSessionLoader{sc: sc},
		MemoryRecaller: &stubMemoryRecaller{entries: []memory.MemoryEntry{{ID: "m1"}}},
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session:     &types.Session{SessionID: "s1"},
		WorkerLocal: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.MemoryEntries) != 0 {
		t.Fatalf("expected 0 memory entries for worker, got %d", len(output.MemoryEntries))
	}
}

// T: D2-S15-A01-T64
func TestPrepareOrchestrator_Prepare_compresses_when_over_budget(t *testing.T) {
	original := []types.Message{{Role: types.MessageRoleUser, Content: "long message"}}
	compressed := []types.Message{{Role: types.MessageRoleUser, Content: "short"}}
	sc := &types.SessionContext{SessionID: "s1", Messages: original}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader: &stubSessionLoader{sc: sc},
		Compressor: &stubCompressor{
			shouldCompress: true,
			compressed:     compressed,
		},
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(output.Messages) != 1 || output.Messages[0].Content != "short" {
		t.Fatalf("expected compressed messages, got %v", output.Messages)
	}
}

// T: D2-S15-A01-T65
func TestPrepareOrchestrator_Prepare_assembles_prompt(t *testing.T) {
	sc := &types.SessionContext{SessionID: "s1", Messages: []types.Message{}}
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{
		SessionLoader:  &stubSessionLoader{sc: sc},
		PromptAssembler: &stubPromptAssembler{prompt: "You are helpful."},
	})
	output, err := orch.Prepare(context.Background(), prepare.PrepareInput{
		Session: &types.Session{SessionID: "s1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.SystemPrompt != "You are helpful." {
		t.Fatalf("expected prompt, got %q", output.SystemPrompt)
	}
}

// T: D2-S15-A01-T66
func TestNewPrepareOrchestrator_nil_deps_does_not_panic(t *testing.T) {
	orch := prepare.NewPrepareOrchestrator(prepare.PrepareDeps{})
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
}
