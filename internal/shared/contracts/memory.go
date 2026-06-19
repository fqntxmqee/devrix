package contracts

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// LongTermRecaller is the read-side port of D2 long-term memory.
//
// DSAFT: D2-S15-A02 (RecallMemory)
//
// Lives in shared/contracts so both prepare/memory (consumer) and
// persist/memory (producer) can reference the port without creating a
// cyclic import between the two packages (D2-STRUCT-T04).
type LongTermRecaller interface {
	Recall(ctx context.Context, query string, limit int) ([]types.MemoryEntry, error)
	Close() error
}

// LongTermStore is the write-side port of D2 long-term memory.
//
// DSAFT: D2-S17-A03 (StoreLongTerm)
type LongTermStore interface {
	Store(ctx context.Context, entry types.MemoryEntry) error
	Close() error
}
