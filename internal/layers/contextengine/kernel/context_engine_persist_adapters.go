// Persist orchestrator adapters — bridge facade fields to persist.* port
// interfaces so the persist.Orchestrator can be wired up.
//
// DSAFT: D2-S17 (PersistSessionState) — facade→orchestrator port adapters.
package kernel

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
)

// snapshotAdapter implements persist.SnapshotPersister over facade.memory.
//
// persist.Orchestrator passes pre-serialized snapshot bytes; facade.memory
// already produced those bytes (e.memory.PersistSnapshot), so this adapter
// writes them to the snapshot store's backup dir via memory.WriteSnapshotBytes.
type snapshotAdapter struct {
	engine *ContextEngine
}

// Persist writes the snapshot bytes to the snapshot store's backup dir.
func (a *snapshotAdapter) Persist(data []byte, sessionID string) error {
	return a.engine.memory.WriteSnapshotBytes(sessionID, data)
}

func newSnapshotAdapter(e *ContextEngine) *snapshotAdapter {
	return &snapshotAdapter{engine: e}
}

// transcriptAdapter implements persist.TranscriptWriter over facade.mainTranscript.
type transcriptAdapter struct {
	engine *ContextEngine
}

// AppendMessages JSON-encodes the byte slice into a batch of messages and
// forwards to the main-thread transcript store. The byte-slice shape matches
// what persist.Orchestrator receives (encoded JSON of []Message).
func (a *transcriptAdapter) AppendMessages(sessionID string, data []byte) error {
	if a.engine.mainTranscript == nil || len(data) == 0 {
		return nil
	}
	var msgs []types.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return err
	}
	return a.engine.mainTranscript.AppendBatch(sessionID, msgs)
}

func newTranscriptAdapter(e *ContextEngine) *transcriptAdapter {
	return &transcriptAdapter{engine: e}
}

// longTermAdapter implements persist.LongTermStorer over facade.memory.
type longTermAdapter struct {
	engine *ContextEngine
}

// AutoStore runs the long-term auto-store path. Mirrors
// facade/engine_persist.go::finalizeTurn's AutoStoreLongTerm call.
func (a *longTermAdapter) AutoStore(ctx context.Context, sessionID, query, summary string) error {
	sc, ok := a.engine.memory.Get(sessionID)
	if !ok || sc == nil {
		return nil
	}
	return a.engine.memory.AutoStoreLongTerm(ctx, sc, query, summary)
}

func newLongTermAdapter(e *ContextEngine) *longTermAdapter {
	return &longTermAdapter{engine: e}
}

// Compile-time guards.
var (
	_ persist.SnapshotPersister = (*snapshotAdapter)(nil)
	_ persist.TranscriptWriter  = (*transcriptAdapter)(nil)
	_ persist.LongTermStorer    = (*longTermAdapter)(nil)
)
