package memory

import "github.com/devrix/devrix/internal/shared/types"

// MemoryEntry re-exports types.MemoryEntry for callers that historically
// imported the row schema from the prepare-side (S15) package.
//
// P4 split: the canonical type lives in shared/types (no domain owns the
// row shape); prepare/memory is the read-side port SoT and exposes a
// narrow MemoryRecaller interface for orchestrator use. See
// persist/memory.LongTermRecaller / LongTermStore for the dual ports.
type MemoryEntry = types.MemoryEntry
