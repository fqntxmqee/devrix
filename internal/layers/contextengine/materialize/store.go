package materialize

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
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
	// strict controls behavior on JSONL parse errors. When false
	// (default), bad lines are skipped with a slog.Warn so a noisy
	// file doesn't fail the whole load — preserves backward compat.
	// When true, the load returns an error on the first bad line so
	// callers (orchestrators / audits) can refuse to surface a
	// half-loaded message chain. RH-D2-CC-06 (DM-20260630-013
	// T-P2-12.1): strict mode is opt-in for now; default lenient
	// matches pre-change behavior. Future D2-S18 may flip the default.
	strict bool
}

// NewPartitionStore creates a store under baseDir (e.g. ~/.devrix/sessions).
func NewPartitionStore(baseDir string) (*PartitionStore, error) {
	dir := textutil.ExpandPath(baseDir)
	if dir == "" {
		return nil, fmt.Errorf("partition store base dir required")
	}
	return &PartitionStore{baseDir: dir}, nil
}

// NewPartitionStoreStrict is like NewPartitionStore but enables strict
// JSONL parse mode. The first unparseable line causes LoadWorkItem /
// LoadAgent to return an error instead of silently skipping. Use this
// for production deployments where data integrity matters more than
// backward compat (RH-D2-CC-06, DM-20260630-013 T-P2-12.1).
func NewPartitionStoreStrict(baseDir string) (*PartitionStore, error) {
	s, err := NewPartitionStore(baseDir)
	if err != nil {
		return nil, err
	}
	s.strict = true
	return s, nil
}

// SetStrict toggles strict mode at runtime. Useful for tests that want
// to exercise the strict path without constructing a separate store.
func (s *PartitionStore) SetStrict(strict bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.strict = strict
	s.mu.Unlock()
}

func (s *PartitionStore) wiPath(sessionID, workItemID string) string {
	return filepath.Join(s.baseDir, sessionID, "wi", workItemID+".jsonl")
}

func (s *PartitionStore) agentPath(sessionID, agentID string) string {
	return filepath.Join(s.baseDir, sessionID, "subagents", agentID+".jsonl")
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

// AppendAgent appends messages to a sub-agent sidechain partition.
func (s *PartitionStore) AppendAgent(sessionID, agentID string, msgs []types.Message) error {
	if s == nil || sessionID == "" || agentID == "" || len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target := s.agentPath(sessionID, agentID)
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
	var badLines int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg types.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			badLines++
			if s.strict {
				// RH-D2-CC-06 (DM-20260630-013 T-P2-12.1): strict mode
				// refuses to load a half-parsed chain so callers can
				// surface a clear "jsonl_corrupt" error rather than
				// silently dropping messages. Operators see the line
				// number + first 80 bytes of the bad line in the
				// returned error for triage.
				return nil, fmt.Errorf("materialize: bad jsonl line %d: %w (line=%q)", badLines, err, truncateForLog(line, 80))
			}
			// Lenient mode: log + skip. The slog.Warn gives operators
			// a Jaeger signal for "we silently dropped N messages from
			// this chain" without breaking the load.
			slog.Warn("materialize: bad jsonl line; skipping (lenient mode)",
				"session_id", sessionID, "work_item_id", workItemID,
				"line_no", badLines, "err", err)
			continue
		}
		out = append(out, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if badLines > 0 {
		slog.Info("materialize: jsonl load completed with skipped lines",
			"session_id", sessionID, "work_item_id", workItemID,
			"loaded", len(out), "skipped", badLines)
	}
	return out, nil
}

// LoadAgent reads persisted sub-agent sidechain messages.
// Mirrors Load's strict/lenient + bad-line-count behavior so a noisy
// sidechain partition fails the same way the work-item chain does
// (RH-D2-CC-06, DM-20260630-013 T-P2-12.1).
func (s *PartitionStore) LoadAgent(sessionID, agentID string) ([]types.Message, error) {
	if s == nil || sessionID == "" || agentID == "" {
		return nil, nil
	}
	target := s.agentPath(sessionID, agentID)
	f, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []types.Message
	var badLines int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg types.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			badLines++
			if s.strict {
				return nil, fmt.Errorf("materialize: bad agent jsonl line %d: %w (line=%q)", badLines, err, truncateForLog(line, 80))
			}
			slog.Warn("materialize: bad agent jsonl line; skipping (lenient mode)",
				"session_id", sessionID, "agent_id", agentID,
				"line_no", badLines, "err", err)
			continue
		}
		out = append(out, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if badLines > 0 {
		slog.Info("materialize: agent jsonl load completed with skipped lines",
			"session_id", sessionID, "agent_id", agentID,
			"loaded", len(out), "skipped", badLines)
	}
	return out, nil
}

// truncateForLog renders b as a string with at most n bytes plus an
// ellipsis, safe for embedding in error messages. Bytes (not runes) is
// fine here because the goal is just to keep the error one line; the
// JSONL charset is ASCII in practice and a partial multi-byte sequence
// at the truncation boundary is acceptable for a triage log.
func truncateForLog(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
