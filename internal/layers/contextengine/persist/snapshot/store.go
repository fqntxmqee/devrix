package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/textutil"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/golang/snappy"
)

const snappyMagic = "\xfe\x53"

// Store handles ContextSnapshotV1 serialization.
type Store struct {
	cfg *config.SnapshotConfig
}

// NewStore creates a snapshot store.
func NewStore(cfg *config.SnapshotConfig) *Store {
	if cfg == nil {
		cfg = &config.SnapshotConfig{Enabled: true}
	}
	return &Store{cfg: cfg}
}

// Serialize converts SessionContext to JSON bytes.
func (s *Store) Serialize(sc *types.SessionContext) ([]byte, error) {
	snap := types.ContextSnapshotV1{
		Version:      types.ContextSnapshotVersion,
		SessionID:    sc.SessionID,
		Model:        sc.Model,
		WorkDir:      sc.WorkDir,
		Messages:     messagesToSnapshots(sc.Messages),
		TokenBudget:  budgetToSnapshot(sc.TokenBudget),
		SystemPrompt: sc.SystemPrompt,
		UpdatedAt:    sc.UpdatedAt.UTC().Format(time.RFC3339),
		Todos:        sc.Todos,
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return s.encodeSnapshot(raw), nil
}

func (s *Store) encodeSnapshot(raw []byte) []byte {
	if s.cfg == nil || !s.cfg.Compression {
		return raw
	}
	threshold := s.cfg.CompressionThreshold
	if threshold <= 0 {
		threshold = 4096
	}
	if len(raw) < threshold {
		return raw
	}
	compressed := snappy.Encode(nil, raw)
	out := make([]byte, len(snappyMagic)+len(compressed))
	copy(out, snappyMagic)
	copy(out[len(snappyMagic):], compressed)
	return out
}

func decodeSnapshotPayload(data []byte) ([]byte, error) {
	if len(data) >= len(snappyMagic) && string(data[:len(snappyMagic)]) == snappyMagic {
		raw, err := snappy.Decode(nil, data[len(snappyMagic):])
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	return data, nil
}

// Deserialize parses JSON into SessionContext.
func (s *Store) Deserialize(data []byte) (*types.SessionContext, error) {
	if len(data) == 0 {
		return nil, errors.NewSnapshotCorruptError(fmt.Errorf("empty snapshot"))
	}
	payload, err := decodeSnapshotPayload(data)
	if err != nil {
		return nil, errors.NewSnapshotCorruptError(err)
	}
	var snap types.ContextSnapshotV1
	if err := json.Unmarshal(payload, &snap); err != nil {
		return nil, errors.NewSnapshotCorruptError(err)
	}
	if snap.Version != types.ContextSnapshotVersion {
		return nil, errors.NewSnapshotCorruptError(fmt.Errorf("unsupported version %q", snap.Version))
	}
	updatedAt, _ := time.Parse(time.RFC3339, snap.UpdatedAt)
	return &types.SessionContext{
		SessionID:    snap.SessionID,
		WorkDir:      snap.WorkDir,
		Model:        snap.Model,
		Messages:     snapshotsToMessages(snap.Messages, snap.SessionID),
		TokenBudget:  snapshotToBudget(snap.TokenBudget),
		SystemPrompt: snap.SystemPrompt,
		UpdatedAt:    updatedAt,
		Todos:        snap.Todos,
	}, nil
}

// WriteBackup optionally writes backup file.
func (s *Store) WriteBackup(sessionID string, data []byte) error {
	if !s.cfg.Enabled || s.cfg.BackupDir == "" {
		return nil
	}
	path, err := s.backupPath(sessionID)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ReadBackup loads a session snapshot from the backup directory.
func (s *Store) ReadBackup(sessionID string) ([]byte, error) {
	if !s.cfg.Enabled || s.cfg.BackupDir == "" {
		return nil, nil
	}
	path, err := s.backupPath(sessionID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup: %w", err)
	}
	return data, nil
}

func (s *Store) backupPath(sessionID string) (string, error) {
	dir := textutil.ExpandPath(s.cfg.BackupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	return filepath.Join(dir, sessionID+".json"), nil
}

func messagesToSnapshots(msgs []types.Message) []types.MessageSnapshot {
	out := make([]types.MessageSnapshot, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, types.MessageSnapshot{
			ID:        m.ID,
			Role:      string(m.Role),
			Content:   m.Content,
			Metadata:  m.Metadata,
			Timestamp: m.Timestamp.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func snapshotsToMessages(snaps []types.MessageSnapshot, sessionID string) []types.Message {
	out := make([]types.Message, 0, len(snaps))
	for _, s := range snaps {
		ts, _ := time.Parse(time.RFC3339, s.Timestamp)
		out = append(out, types.Message{
			ID:        s.ID,
			SessionID: sessionID,
			Role:      types.MessageRole(s.Role),
			Content:   s.Content,
			Metadata:  s.Metadata,
			Timestamp: ts,
		})
	}
	return out
}

func budgetToSnapshot(b types.TokenBudget) types.TokenBudgetSnapshot {
	return types.TokenBudgetSnapshot{
		MaxContextTokens:  b.MaxContextTokens,
		ReservedOutput:    b.ReservedOutput,
		ToolResultBudget:  b.ToolResultBudget,
		CompressionTarget: b.CompressionTarget,
	}
}

func snapshotToBudget(s types.TokenBudgetSnapshot) types.TokenBudget {
	if s.MaxContextTokens == 0 {
		return types.DefaultTokenBudget()
	}
	return types.TokenBudget{
		MaxContextTokens:  s.MaxContextTokens,
		ReservedOutput:    s.ReservedOutput,
		ToolResultBudget:  s.ToolResultBudget,
		CompressionTarget: s.CompressionTarget,
	}
}
