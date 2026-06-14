package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/errors"
)

// MemoryEntry is a persisted long-term memory record.
type MemoryEntry struct {
	ID        string
	SessionID string
	Topic     string
	Content   string
	CreatedAt time.Time
}

// ILongTermMemory provides cross-session recall and store.
type ILongTermMemory interface {
	Recall(ctx context.Context, query string, limit int) ([]MemoryEntry, error)
	Store(ctx context.Context, entry MemoryEntry) error
	Close() error
}

// SQLiteLongTermMemory stores entries in SQLite.
type SQLiteLongTermMemory struct {
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
func OpenSQLiteLongTerm(dbPath string) (*SQLiteLongTermMemory, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(createMemoryTableSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteLongTermMemory{db: db}, nil
}

// NewLongTermFromConfig creates long-term memory from configuration.
func NewLongTermFromConfig(cfg config.LongTermConfig) (ILongTermMemory, error) {
	if !cfg.Enabled {
		return NewDisabledLongTermMemory(), nil
	}
	path, err := config.ResolvedLongTermDBPath(cfg)
	if err != nil {
		return nil, err
	}
	return OpenSQLiteLongTerm(path)
}

// Recall searches topic and content with LIKE matching.
func (m *SQLiteLongTermMemory) Recall(ctx context.Context, query string, limit int) ([]MemoryEntry, error) {
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
func (m *SQLiteLongTermMemory) Store(ctx context.Context, entry MemoryEntry) error {
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
func (m *SQLiteLongTermMemory) Close() error {
	if m.db == nil {
		return nil
	}
	return m.db.Close()
}
