package snapshot_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-05
func TestStore_should_roundtrip_snapshot_v1(t *testing.T) {
	store := snapshot.NewStore(nil)
	sc := &types.SessionContext{
		SessionID:    "sess_1",
		WorkDir:      "/tmp",
		Model:        "test",
		SystemPrompt: "hello",
		Messages:     []types.Message{*types.NewMessage("m1", "sess_1", types.MessageRoleUser, "hi")},
		TokenBudget:  types.DefaultTokenBudget(),
		PEVState:     types.DefaultPEVState(3),
		UpdatedAt:    time.Now(),
	}

	data, err := store.Serialize(sc)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}

	restored, err := store.Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if restored.SessionID != sc.SessionID || len(restored.Messages) != 1 {
		t.Errorf("unexpected restore: %+v", restored)
	}
}

// Covers: L5-CTX-32
func TestStore_should_compress_large_snapshots_with_snappy(t *testing.T) {
	cfg := &config.SnapshotConfig{
		Enabled:              true,
		Compression:          true,
		CompressionThreshold: 64,
	}
	store := snapshot.NewStore(cfg)

	var msgs []types.Message
	for i := 0; i < 200; i++ {
		msgs = append(msgs, *types.NewMessage(
			fmt.Sprintf("m%d", i),
			"sess_snappy",
			types.MessageRoleUser,
			strings.Repeat("payload ", 40),
		))
	}
	sc := &types.SessionContext{
		SessionID:    "sess_snappy",
		WorkDir:      "/tmp",
		Model:        "test",
		SystemPrompt: strings.Repeat("prompt ", 200),
		Messages:     msgs,
		TokenBudget:  types.DefaultTokenBudget(),
		PEVState:     types.DefaultPEVState(3),
		UpdatedAt:    time.Now(),
	}

	rawStore := snapshot.NewStore(&config.SnapshotConfig{Enabled: true})
	rawData, err := rawStore.Serialize(sc)
	if err != nil {
		t.Fatalf("raw Serialize: %v", err)
	}

	compressed, err := store.Serialize(sc)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if len(compressed) >= len(rawData) {
		t.Fatalf("expected smaller compressed payload: raw=%d compressed=%d", len(rawData), len(compressed))
	}
	if string(compressed[:2]) != "\xfe\x53" {
		t.Fatal("expected snappy magic header")
	}

	restored, err := store.Deserialize(compressed)
	if err != nil {
		t.Fatalf("Deserialize compressed: %v", err)
	}
	if restored.SessionID != sc.SessionID || len(restored.Messages) != len(sc.Messages) {
		t.Fatalf("unexpected restore: session=%s msgs=%d", restored.SessionID, len(restored.Messages))
	}

	legacy, err := store.Deserialize(rawData)
	if err != nil {
		t.Fatalf("Deserialize legacy: %v", err)
	}
	if legacy.SessionID != sc.SessionID {
		t.Fatalf("legacy restore failed: %s", legacy.SessionID)
	}
}

func TestStore_should_skip_compression_below_threshold(t *testing.T) {
	cfg := &config.SnapshotConfig{
		Enabled:              true,
		Compression:          true,
		CompressionThreshold: 1_000_000,
	}
	store := snapshot.NewStore(cfg)
	sc := &types.SessionContext{
		SessionID: "small",
		Messages:  []types.Message{*types.NewMessage("m1", "small", types.MessageRoleUser, "hi")},
		UpdatedAt: time.Now(),
	}
	data, err := store.Serialize(sc)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if string(data[:2]) == "\xfe\x53" {
		t.Fatal("small snapshot should not be compressed")
	}
}
