package snapshot_test

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
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
