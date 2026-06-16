package fallback

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// SessionLogEntry is a single append-only session log record.
type SessionLogEntry struct {
	Role      types.MessageRole `json:"role"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	SessionID string            `json:"sessionId"`
}

// SessionLog appends transcript entries to an on-disk JSONL file (V5a optional).
type SessionLog struct {
	dir string
}

// NewSessionLog creates a session log writer under dir.
func NewSessionLog(dir string) *SessionLog {
	return &SessionLog{dir: dir}
}

// Append writes a session log entry when enabled.
func (l *SessionLog) Append(sessionID string, role types.MessageRole, content string) error {
	if l == nil || l.dir == "" || sessionID == "" {
		return nil
	}
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return err
	}
	entry := SessionLogEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		SessionID: sessionID,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := filepath.Join(l.dir, sessionID+".log.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}
