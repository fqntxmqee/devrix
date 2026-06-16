package transcript

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

// SidechainStore appends sub-agent messages as JSONL for resume.
type SidechainStore struct {
	mu      sync.Mutex
	baseDir string
}

// NewSidechainStore creates a store under baseDir (e.g. ~/.devrix/sessions).
func NewSidechainStore(baseDir string) (*SidechainStore, error) {
	dir := textutil.ExpandPath(baseDir)
	if dir == "" {
		return nil, fmt.Errorf("sidechain base dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &SidechainStore{baseDir: dir}, nil
}

func (s *SidechainStore) path(sessionID, agentID string) string {
	return filepath.Join(s.baseDir, sessionID, "subagents", agentID+".jsonl")
}

// Append writes one message line to the sidechain file.
func (s *SidechainStore) Append(sessionID, agentID string, msg types.Message) error {
	if s == nil || sessionID == "" || agentID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.path(sessionID, agentID)
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

// Load reads all messages from a sidechain file.
func (s *SidechainStore) Load(sessionID, agentID string) ([]types.Message, error) {
	if s == nil || sessionID == "" || agentID == "" {
		return nil, nil
	}
	target := s.path(sessionID, agentID)
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
