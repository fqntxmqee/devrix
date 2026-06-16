// Package persist — D2-S17 PersistSessionState orchestrator.
//
// PersistOrchestrator coordinates the 4 A-level activities:
//
//	A01 SaveSnapshot     → session context serialization to disk
//	A02 WriteTranscript  → main thread + sidechain transcript append
//	A03 StoreLongTerm    → long-term memory auto-store
//	A04 CommitWindow     → message truncation + active window commit
package persist

import (
	"context"
)

// PersistDeps bundles dependencies for the persist orchestration.
type PersistDeps struct {
	SnapshotPersister SnapshotPersister
	TranscriptWriter  TranscriptWriter
	LongTermStorer    LongTermStorer
}

// PersistInput carries the inputs for a persist orchestration run.
type PersistInput struct {
	SessionID        string
	AgentID          string // empty for main thread
	SnapshotData     []byte
	TranscriptDelta  []byte
	Query            string
	AssistantSummary string
	IsWorker         bool
}

// PersistOrchestrator orchestrates the D2-S17 PersistSessionState scenario.
//
// DSAFT: D2-S17 (PersistSessionState)
type PersistOrchestrator struct {
	deps PersistDeps
}

// NewPersistOrchestrator creates a persist orchestrator.
func NewPersistOrchestrator(deps PersistDeps) *PersistOrchestrator {
	return &PersistOrchestrator{deps: deps}
}

// Persist runs the full PersistSessionState pipeline.
func (o *PersistOrchestrator) Persist(ctx context.Context, input PersistInput) error {
	if len(input.SnapshotData) > 0 && o.deps.SnapshotPersister != nil {
		if err := o.deps.SnapshotPersister.Persist(input.SnapshotData, input.SessionID); err != nil {
			return err
		}
	}

	if len(input.TranscriptDelta) > 0 && !input.IsWorker && o.deps.TranscriptWriter != nil {
		if err := o.deps.TranscriptWriter.AppendMessages(input.SessionID, input.TranscriptDelta); err != nil {
			return err
		}
	}

	if input.Query != "" && input.AssistantSummary != "" && o.deps.LongTermStorer != nil {
		_ = o.deps.LongTermStorer.AutoStore(ctx, input.SessionID, input.Query, input.AssistantSummary)
	}

	return nil
}
