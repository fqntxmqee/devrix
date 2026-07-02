// T: D2-S15-A02-T15 — PerMessageBudget unit tests.
package compression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestPerMessageBudget_UnderThreshold_NoPersist(t *testing.T) {
	dir := t.TempDir()
	b := &PerMessageBudget{Threshold: 1000, ProjectDir: dir, SessionID: "s1"}
	// 500 bytes < 1000 → no persist
	got := b.Enforce("call_under", strings.Repeat("x", 500))
	if got != strings.Repeat("x", 500) {
		t.Errorf("under-threshold must round-trip, got len %d (want 500)", len(got))
	}
	if b.State != nil && b.State.IsSeen("call_under") {
		t.Errorf("under-threshold should still MarkSeen (decision freeze), got not seen")
	}
}

func TestPerMessageBudget_OverThreshold_Persists(t *testing.T) {
	dir := t.TempDir()
	b := &PerMessageBudget{Threshold: 1000, ProjectDir: dir, SessionID: "s2"}
	content := strings.Repeat("y", 5000) // 5K > 1K
	got := b.Enforce("call_over", content)
	if !strings.Contains(got, PersistedOutputTag) {
		t.Errorf("over-threshold must produce <persisted-output> wrapper, got: %s", firstLine2(got))
	}
	// File on disk
	wantPath := filepath.Join(dir, "s2", "tool-results", "call_over.txt")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("persisted file missing at %s: %v", wantPath, err)
	}
	// On disk must hold full content
	onDisk, _ := os.ReadFile(wantPath)
	if string(onDisk) != content {
		t.Errorf("on-disk content mismatch (len=%d, want=%d)", len(onDisk), len(content))
	}
}

func TestPerMessageBudget_CriticalBoundary_ExactlyThreshold_NoPersist(t *testing.T) {
	dir := t.TempDir()
	b := &PerMessageBudget{Threshold: 1000, ProjectDir: dir, SessionID: "s3"}
	// Exactly 1000 bytes = threshold (not over) → no persist
	content := strings.Repeat("z", 1000)
	got := b.Enforce("call_at", content)
	if got != content {
		t.Errorf("at-threshold should NOT persist, got len %d (want 1000)", len(got))
	}
}

func TestPerMessageBudget_FrozenDecision_ReApply(t *testing.T) {
	dir := t.TempDir()
	state := persist.NewContentReplacementState()
	b := &PerMessageBudget{Threshold: 100, ProjectDir: dir, SessionID: "s4", State: state}

	content := strings.Repeat("a", 500)
	// First call: persists + records decision
	first := b.Enforce("call_frozen", content)
	if !strings.Contains(first, PersistedOutputTag) {
		t.Fatalf("first call should persist, got: %s", firstLine2(first))
	}
	cached, ok := state.Lookup("call_frozen")
	if !ok {
		t.Fatal("first call should record replacement")
	}

	// Second call with the SAME toolUseID + SAME content: must return
	// the EXACT same byte-identical string (no I/O).
	second := b.Enforce("call_frozen", "this-original-content-is-ignored-when-cached")
	if second != cached {
		t.Errorf("second call must re-apply cached replacement byte-identical\nfirst:  %q\nsecond: %q", firstLine2(cached), firstLine2(second))
	}
}

func TestPerMessageBudget_FrozenUnreplaced_NeverPersists(t *testing.T) {
	dir := t.TempDir()
	state := persist.NewContentReplacementState()
	state.MarkSeen("call_already_seen") // simulate "saw it small last turn"
	b := &PerMessageBudget{Threshold: 100, ProjectDir: dir, SessionID: "s5", State: state}

	// Now content is large, but the decision is FROZEN (seen but no
	// replacement). Persisting now would change a prefix the LLM
	// already cached on a prior turn.
	content := strings.Repeat("b", 500)
	got := b.Enforce("call_already_seen", content)
	if got != content {
		t.Errorf("frozen-unreplaced must return content as-is, got len %d", len(got))
	}
	if strings.Contains(got, PersistedOutputTag) {
		t.Errorf("frozen-unreplaced must NOT contain <persisted-output>")
	}
}

func TestPerMessageBudget_NilState_WorksWithoutDecisionFreeze(t *testing.T) {
	dir := t.TempDir()
	b := &PerMessageBudget{Threshold: 100, ProjectDir: dir, SessionID: "s6", State: nil}
	content := strings.Repeat("c", 500)
	got := b.Enforce("call_nostate", content)
	if !strings.Contains(got, PersistedOutputTag) {
		t.Errorf("nil state should still persist, got: %s", firstLine2(got))
	}
}

func TestPerMessageBudget_PersistFailure_FallsBackToTruncate(t *testing.T) {
	// Unwritable projectDir → PersistToFile returns error → caller
	// falls back to TruncateWithMarker.
	b := &PerMessageBudget{Threshold: 100, ProjectDir: "/nonexistent-readonly-xyz", SessionID: "s7"}
	content := strings.Repeat("d", 500)
	got := b.Enforce("call_fail", content)
	if !strings.Contains(got, "complete=false") {
		t.Errorf("unwritable dir must fall back to truncate-with-marker, got: %s", firstLine2(got))
	}
}

func TestPerMessageBudget_ShouldEnforce_Defaults(t *testing.T) {
	b := &PerMessageBudget{Threshold: 0}
	if !b.ShouldEnforce() {
		t.Errorf("ShouldEnforce should be true with default constant")
	}
	b = &PerMessageBudget{Threshold: 200_000}
	if !b.ShouldEnforce() {
		t.Errorf("ShouldEnforce should be true with 200K threshold")
	}
}

// PerMessageBudget_EndToEnd_Pipeline: drives applyPerMessageBudget
// through the pipeline step in a realistic shape — N tool messages
// where some are over the per-message threshold and need persist.
func TestPerMessageBudget_EndToEnd_Pipeline(t *testing.T) {
	dir := t.TempDir()
	state := persist.NewContentReplacementState()
	budget := &PerMessageBudget{Threshold: 1000, ProjectDir: dir, SessionID: "s8", State: state}

	// Mix of under + over threshold tool messages
	msgs := []types.Message{
		{ID: "small", Role: types.MessageRoleTool, Content: strings.Repeat("s", 500), Timestamp: time.Now()},
		{ID: "big", Role: types.MessageRoleTool, Content: strings.Repeat("B", 5000), Timestamp: time.Now()},
		{ID: "user1", Role: types.MessageRoleUser, Content: "user prompt", Timestamp: time.Now()},
	}
	out := applyPerMessageBudget(budget, msgs)
	if out[0].Content != strings.Repeat("s", 500) {
		t.Errorf("under-threshold should not change, got len %d", len(out[0].Content))
	}
	if !strings.Contains(out[1].Content, PersistedOutputTag) {
		t.Errorf("over-threshold should produce <persisted-output>, got: %s", firstLine2(out[1].Content))
	}
	if out[2].Content != "user prompt" {
		t.Errorf("user message should not change, got %q", out[2].Content)
	}
	// File on disk for the big one
	wantPath := filepath.Join(dir, "s8", "tool-results", "big.txt")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected persisted file at %s: %v", wantPath, err)
	}
}

func firstLine2(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
