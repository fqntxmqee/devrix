package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/devrix/devrix/internal/shared/types"
)

// MainThreadStore appends the full main-session message history as JSONL.
type MainThreadStore struct {
	mu      sync.Mutex
	baseDir string
}

// NewMainThreadStore creates a store under baseDir (e.g. ~/.devrix/sessions).
func NewMainThreadStore(baseDir string) (*MainThreadStore, error) {
	dir := expandPath(baseDir)
	if dir == "" {
		return nil, fmt.Errorf("main transcript base dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &MainThreadStore{baseDir: dir}, nil
}

func (s *MainThreadStore) path(sessionID string) string {
	return filepath.Join(s.baseDir, sessionID, "transcript.jsonl")
}

// Append writes one message line to the session transcript file.
func (s *MainThreadStore) Append(sessionID string, msg types.Message) error {
	if s == nil || sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.path(sessionID)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// AppendBatch appends multiple messages in order.
func (s *MainThreadStore) AppendBatch(sessionID string, msgs []types.Message) error {
	for _, m := range msgs {
		if err := s.Append(sessionID, m); err != nil {
			return err
		}
	}
	return nil
}

// Load reads all messages from a session transcript file.
func (s *MainThreadStore) Load(sessionID string) ([]types.Message, error) {
	if s == nil || sessionID == "" {
		return nil, nil
	}
	target := s.path(sessionID)
	f, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []types.Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg types.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, scanner.Err()
}
