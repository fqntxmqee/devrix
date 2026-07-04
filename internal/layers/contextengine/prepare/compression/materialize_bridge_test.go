package compression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestApplyPersistBudget_persistsOversizedToolResult(t *testing.T) {
	dir := t.TempDir()
	sessionID := "sess-mat-persist"
	large := strings.Repeat("x", 60_000)
	msgs := []types.Message{{
		ID:      "tool-1",
		Role:    types.MessageRoleTool,
		Content: large,
	}}
	out := ApplyPersistBudget(msgs, PersistBudgetConfig{
		ProjectDir: dir,
		SessionID:  sessionID,
		PerMessageBudget: &PerMessageBudget{
			ProjectDir: dir,
			SessionID:  sessionID,
			State:      persist.NewContentReplacementState(),
		},
	})
	if len(out) != 1 {
		t.Fatalf("len=%d, want 1", len(out))
	}
	if !strings.Contains(out[0].Content, "<persisted-output>") {
		t.Fatalf("expected persisted-output wrapper, got %q", out[0].Content[:min(120, len(out[0].Content))])
	}
	path := filepath.Join(dir, sessionID, "tool-results", "tool-1.txt")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persist file missing: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
