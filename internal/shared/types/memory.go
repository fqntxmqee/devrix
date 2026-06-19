package types

import "time"

// MemoryEntry is a persisted long-term memory record.
//
// Originally declared in prepare/memory/longterm.go (S15 read-side).
// P4 split moved the storage implementation to persist/memory and
// promoted the row schema to shared/types so the read-side and
// write-side can both reference it without creating a cyclic import
// between prepare/memory and persist/memory (D2-STRUCT-T04).
type MemoryEntry struct {
	ID        string
	SessionID string
	Topic     string
	Content   string
	CreatedAt time.Time
}
