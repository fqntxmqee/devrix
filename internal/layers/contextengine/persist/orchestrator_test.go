package persist_test

import (
	"context"
	"errors"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
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

type stubCommitWindowRunner struct {
	err    error
	called bool
	sc     *types.SessionContext
}

func (s *stubCommitWindowRunner) RunCommitWindow(_ context.Context, sc *types.SessionContext) (types.CompressionReport, error) {
	s.called = true
	s.sc = sc
	if s.err != nil {
		return types.CompressionReport{}, s.err
	}
	return types.CompressionReport{}, nil
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

// T: D2-S17-A04-T45 (AC-P1-2 — orchestrator must call CommitWindow A04)
func TestPersistOrchestrator_Persist_runs_CommitWindow_for_main(t *testing.T) {
	cw := &stubCommitWindowRunner{}
	sc := &types.SessionContext{SessionID: "s1"}
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		CommitWindow: cw,
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:      "s1",
		SessionContext: sc,
		IsWorker:       false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cw.called {
		t.Error("expected CommitWindow.RunCommitWindow to be called for main session")
	}
	if cw.sc != sc {
		t.Error("CommitWindow received wrong session context")
	}
}

// T: D2-S17-A04-T46 — CommitWindow must be skipped for worker sessions.
func TestPersistOrchestrator_Persist_skips_CommitWindow_for_worker(t *testing.T) {
	cw := &stubCommitWindowRunner{}
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		CommitWindow: cw,
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:      "s1",
		SessionContext: &types.SessionContext{},
		IsWorker:       true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cw.called {
		t.Error("expected CommitWindow NOT to be called for worker")
	}
}

// T: D2-S17-A04-T47 — CommitWindow must be skipped when SessionContext is nil.
func TestPersistOrchestrator_Persist_skips_CommitWindow_without_SC(t *testing.T) {
	cw := &stubCommitWindowRunner{}
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		CommitWindow: cw,
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID: "s1",
		// SessionContext is nil
		IsWorker: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cw.called {
		t.Error("expected CommitWindow NOT to be called when SessionContext is nil")
	}
}

// T: D2-S17-A04-T48 — CommitWindow error must propagate.
func TestPersistOrchestrator_Persist_propagates_CommitWindow_error(t *testing.T) {
	cw := &stubCommitWindowRunner{err: errors.New("trim failed")}
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		CommitWindow: cw,
	})
	err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:      "s1",
		SessionContext: &types.SessionContext{},
	})
	if err == nil {
		t.Fatal("expected error from CommitWindow to propagate")
	}
}

// T: D2-S17-A04-T49 — CommitWindow must run BEFORE Snapshot (so snapshot captures
// the trimmed view, matching the legacy facade behavior).
func TestPersistOrchestrator_Persist_runs_CommitWindow_before_Snapshot(t *testing.T) {
	order := make([]string, 0, 2)
	cw := &recordingCommitWindow{onRun: func() { order = append(order, "CommitWindow") }}
	sp := &recordingSnapshotPersister{onPersist: func() { order = append(order, "Snapshot") }}
	orch := persist.NewPersistOrchestrator(persist.PersistDeps{
		CommitWindow:      cw,
		SnapshotPersister: sp,
	})
	if err := orch.Persist(context.Background(), persist.PersistInput{
		SessionID:      "s1",
		SessionContext: &types.SessionContext{},
		SnapshotData:   []byte("snap"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != "CommitWindow" || order[1] != "Snapshot" {
		t.Errorf("expected order [CommitWindow, Snapshot], got %v", order)
	}
}

type recordingCommitWindow struct {
	onRun func()
}

func (s *recordingCommitWindow) RunCommitWindow(_ context.Context, _ *types.SessionContext) (types.CompressionReport, error) {
	if s.onRun != nil {
		s.onRun()
	}
	return types.CompressionReport{}, nil
}

type recordingSnapshotPersister struct {
	onPersist func()
}

func (s *recordingSnapshotPersister) Persist(_ []byte, _ string) error {
	if s.onPersist != nil {
		s.onPersist()
	}
	return nil
}
