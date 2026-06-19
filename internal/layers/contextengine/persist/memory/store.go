// Package memory hosts the S17 long-term memory store: the SQLite-backed
// implementation that satisfies both the read-side (LongTermRecaller) and
// write-side (LongTermStore) interfaces consumed by prepare/memory.Manager.
//
// DSAFT: D2-S17-A03 (StoreLongTerm), D2-S15-A02 (RecallMemory) impl.
//
// Interface split rationale (AC-P4-3): the consumer in prepare/memory only
// depends on a narrow role-specific interface (Recaller or Store). A single
// *SQLiteLongTerm value can be passed to both because the underlying
// connection is shared — but the Manager struct holds them as two
// independent fields, not a single combined ILongTermMemory, so the
// read-side and write-side are independently mockable / swappable.
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// MemoryEntry is re-exported as a type alias for callers that
// historically imported the row schema from the storage package.
type MemoryEntry = types.MemoryEntry

// LongTermRecaller is re-exported from shared/contracts for convenience.
type LongTermRecaller = contracts.LongTermRecaller

// LongTermStore is re-exported from shared/contracts for convenience.
type LongTermStore = contracts.LongTermStore

// SQLiteLongTerm is the production implementation backed by a single
// SQLite file. A value satisfies both LongTermRecaller and LongTermStore
// because read and write share the same connection.
type SQLiteLongTerm struct {
	db *sql.DB
}

const createMemoryTableSQL = `
CREATE TABLE IF NOT EXISTS memory_entries (
	id TEXT PRIMARY KEY,
	session_id TEXT NOT NULL,
	topic TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_memory_topic ON memory_entries(topic);
CREATE INDEX IF NOT EXISTS idx_memory_session ON memory_entries(session_id);
`

// OpenSQLiteLongTerm opens or creates a SQLite long-term memory database.
func OpenSQLiteLongTerm(dbPath string) (*SQLiteLongTerm, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(createMemoryTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteLongTerm{db: db}, nil
}

// NewLongTermFromConfig returns a (recaller, store) pair from configuration.
// When cfg.Enabled is false, both are wired to a no-op disabled stub.
func NewLongTermFromConfig(cfg config.LongTermConfig) (LongTermRecaller, LongTermStore, error) {
	if !cfg.Enabled {
		disabled := &DisabledLongTerm{}
		return disabled, disabled, nil
	}
	path, err := config.ResolvedLongTermDBPath(cfg)
	if err != nil {
		return nil, nil, err
	}
	lt, err := OpenSQLiteLongTerm(path)
	if err != nil {
		return nil, nil, err
	}
	return lt, lt, nil
}

// Recall searches topic and content with LIKE matching.
func (m *SQLiteLongTerm) Recall(ctx context.Context, query string, limit int) ([]MemoryEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	pattern := "%" + strings.TrimSpace(query) + "%"
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, session_id, topic, content, created_at
		FROM memory_entries
		WHERE topic LIKE ? OR content LIKE ?
		ORDER BY created_at DESC
		LIMIT ?`, pattern, pattern, limit)
	if err != nil {
		return nil, errors.NewLongTermDBError(err)
	}
	defer rows.Close()

	var out []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var created int64
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Topic, &e.Content, &created); err != nil {
			return nil, errors.NewLongTermDBError(err)
		}
		e.CreatedAt = time.Unix(created, 0)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.NewLongTermDBError(err)
	}
	return out, nil
}

// Store persists a memory entry.
func (m *SQLiteLongTerm) Store(ctx context.Context, entry MemoryEntry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("mem_%d", time.Now().UnixNano())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO memory_entries (id, session_id, topic, content, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		entry.ID, entry.SessionID, entry.Topic, entry.Content, entry.CreatedAt.Unix(),
	)
	if err != nil {
		return errors.NewLongTermDBError(err)
	}
	return nil
}

// Close closes the database.
func (m *SQLiteLongTerm) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}

// DisabledLongTerm is a no-op recaller + store used when
// longterm.enabled=false. Store is silently dropped; Recall returns
// FeatureNotImplemented to surface misuse in tests/dev.
type DisabledLongTerm struct{}

// NewDisabledLongTerm returns a value usable as both interfaces.
func NewDisabledLongTerm() *DisabledLongTerm {
	return &DisabledLongTerm{}
}

// Recall returns FeatureNotImplemented when long-term memory is disabled.
func (d *DisabledLongTerm) Recall(_ context.Context, _ string, _ int) ([]MemoryEntry, error) {
	return nil, errors.NewFeatureNotImplementedError("long-term memory", "v3")
}

// Store is a no-op for disabled memory.
func (d *DisabledLongTerm) Store(_ context.Context, _ MemoryEntry) error {
	return nil
}

// Close is a no-op.
func (d *DisabledLongTerm) Close() error {
	return nil
}
