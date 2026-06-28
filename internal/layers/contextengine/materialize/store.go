package materialize

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
)

// PartitionStore persists WorkItemPrivate jsonl chains under baseDir.
type PartitionStore struct {
	mu      sync.Mutex
	baseDir string
}

// NewPartitionStore creates a store under baseDir (e.g. ~/.devrix/sessions).
func NewPartitionStore(baseDir string) (*PartitionStore, error) {
	dir := textutil.ExpandPath(baseDir)
	if dir == "" {
		return nil, fmt.Errorf("partition store base dir required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &PartitionStore{baseDir: dir}, nil
}

func (s *PartitionStore) wiPath(sessionID, workItemID string) string {
	return filepath.Join(s.baseDir, sessionID, "wi", workItemID+".jsonl")
}

// Append appends messages to a work item private chain.
func (s *PartitionStore) Append(sessionID, workItemID string, msgs []types.Message) error {
	if s == nil || sessionID == "" || workItemID == "" || len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.wiPath(sessionID, workItemID)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, msg := range msgs {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// Load reads all messages from a work item private chain.
func (s *PartitionStore) Load(sessionID, workItemID string) ([]types.Message, error) {
	if s == nil || sessionID == "" || workItemID == "" {
		return nil, nil
	}
	target := s.wiPath(sessionID, workItemID)
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
			continue
		}
		out = append(out, msg)
	}
	return out, scanner.Err()
}
