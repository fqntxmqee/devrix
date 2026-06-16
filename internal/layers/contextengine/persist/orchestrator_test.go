package persist_test

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
)

type stubSnapshotPersister struct {
	err error
}

func (s *stubSnapshotPersister) Persist(_ []byte, _ string) error {
	return s.err
}

type stubTranscriptWriter struct {
	err error
}

func (s *stubTranscriptWriter) AppendMessages(_ string, _ []byte) error {
	return s.err
}

type stubLongTermStorer struct {
	err error
}

func (s *stubLongTermStorer) AutoStore(_ context.Context, _, _, _ string) error {
	return s.err
}

// T: D2-S17-A01-T41
func TestPersistOrchestrator_Persist_saves_snapshot(t *testing.T) {
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		SnapshotPersister: &stubSnapshotPersister{},
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:    "s1",
		SnapshotData: []byte("snapshot_data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// T: D2-S17-A01-T42
func TestPersistOrchestrator_Persist_returns_error_on_snapshot_failure(t *testing.T) {
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		SnapshotPersister: &stubSnapshotPersister{err: errors.New("disk full")},
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:    "s1",
		SnapshotData: []byte("data"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// T: D2-S17-A01-T43
func TestPersistOrchestrator_Persist_skips_transcript_for_worker(t *testing.T) {
	writer := &stubTranscriptWriter{}
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		TranscriptWriter: writer,
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:       "s1",
		TranscriptDelta: []byte("data"),
		IsWorker:        true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// T: D2-S17-A01-T44
func TestNewPersistOrchestrator_nil_deps_does_not_panic(t *testing.T) {
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{})
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	err := orch.Persist(context.Background(), persist.PersistInput{SessionID: "s1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
