// T04 + T08 tests for ContentReplacementState.
// DM-20260702-008 / D2-S15-A02-T04.
package persist

import (
	"testing"
)

func TestNewContentReplacementState_Empty(t *testing.T) {
	s := NewContentReplacementState()
	if s == nil {
		t.Fatal("NewContentReplacementState must not return nil")
	}
	if s.Size() != 0 {
		t.Errorf("fresh state Size = %d, want 0", s.Size())
	}
	if _, ok := s.Lookup("any"); ok {
		t.Errorf("fresh state must not contain any replacements")
	}
	if s.IsSeen("any") {
		t.Errorf("fresh state must not mark any IDs as seen")
	}
}

func TestContentReplacementState_MarkSeenLookup(t *testing.T) {
	s := NewContentReplacementState()
	s.MarkSeen("call_1")
	if !s.IsSeen("call_1") {
		t.Errorf("MarkSeen must make IsSeen return true")
	}
	if s.IsSeen("call_2") {
		t.Errorf("MarkSeen(call_1) must not affect call_2")
	}
	if s.Size() != 1 {
		t.Errorf("Size = %d after one MarkSeen, want 1", s.Size())
	}
	// MarkSeen is idempotent
	s.MarkSeen("call_1")
	if s.Size() != 1 {
		t.Errorf("MarkSeen must be idempotent (Size = %d, want 1)", s.Size())
	}
}

func TestContentReplacementState_RecordReplacement(t *testing.T) {
	s := NewContentReplacementState()
	s.RecordReplacement("call_1", "<persisted-output>preview1</persisted-output>")
	if !s.IsSeen("call_1") {
		t.Errorf("RecordReplacement must also mark the ID as seen")
	}
	got, ok := s.Lookup("call_1")
	if !ok {
		t.Fatal("Lookup after RecordReplacement must hit")
	}
	if got != "<persisted-output>preview1</persisted-output>" {
		t.Errorf("Lookup returned %q, want the exact stored string", got)
	}
	// Re-record must overwrite (caller bug path; we don't prevent it
	// but we want to be loud if it happens).
	s.RecordReplacement("call_1", "different")
	if v, _ := s.Lookup("call_1"); v != "different" {
		t.Errorf("RecordReplacement should overwrite (got %q, want %q)", v, "different")
	}
}

func TestContentReplacementState_Apply_NilState(t *testing.T) {
	var s *ContentReplacementState // nil
	out, frozen, fresh := s.Apply("call_1", "content")
	if out != "content" {
		t.Errorf("nil state must return content unchanged, got %q", out)
	}
	if frozen {
		t.Errorf("nil state must not report frozen")
	}
	if !fresh {
		t.Errorf("nil state must report fresh (caller decides)")
	}
}

func TestContentReplacementState_Apply_FreshUnseen(t *testing.T) {
	s := NewContentReplacementState()
	out, frozen, fresh := s.Apply("call_never_seen", "hello")
	if out != "" {
		t.Errorf("fresh unseen must return empty out (caller decides), got %q", out)
	}
	if frozen {
		t.Errorf("fresh unseen must not be frozen")
	}
	if !fresh {
		t.Errorf("fresh unseen must be fresh")
	}
}

func TestContentReplacementState_Apply_FrozenUnreplaced(t *testing.T) {
	// Path: caller saw the result, decided it was small enough not to
	// persist, marked seen. Later, the result size somehow crosses the
	// threshold — Apply must return the original content (frozen), NOT
	// run the persist decision. Persisting now would change a prefix
	// the LLM already cached.
	s := NewContentReplacementState()
	s.MarkSeen("call_small")
	out, frozen, fresh := s.Apply("call_small", "original-content")
	if out != "original-content" {
		t.Errorf("frozen-unreplaced must return content as-is, got %q", out)
	}
	if !frozen {
		t.Errorf("frozen-unreplaced must be flagged frozen=true")
	}
	if fresh {
		t.Errorf("frozen-unreplaced must NOT be fresh (would re-trigger decision)")
	}
}

func TestContentReplacementState_Apply_ReapplyCached(t *testing.T) {
	// Path: result was persisted on turn 1, replacement cached. On
	// turn 2, Apply must return the EXACT same replacement string —
	// no I/O, no recomputation. This is the prompt-cache stability
	// invariant.
	s := NewContentReplacementState()
	cached := "<persisted-output>\nOutput too large (5.0 KB). Full output saved to: /tmp/x\n\nPreview (first 2.0 KB):\nhello\n</persisted-output>"
	s.RecordReplacement("call_big", cached)
	out, frozen, fresh := s.Apply("call_big", "this-is-ignored-when-cached")
	if out != cached {
		t.Errorf("Apply must return cached replacement byte-identical, got %q", out)
	}
	if !frozen {
		t.Errorf("Apply with cached replacement must be frozen")
	}
	if fresh {
		t.Errorf("Apply with cached replacement must NOT be fresh")
	}
}

func TestNewContentReplacementStateFrom_Reconstructs(t *testing.T) {
	// Resume path: load records from transcript, reconstruct state.
	candidates := []string{"call_a", "call_b", "call_c"}
	records := []ContentReplacementRecord{
		{Kind: "tool-result", ToolUseID: "call_a", Replacement: "<persisted-output>a</persisted-output>"},
		{Kind: "tool-result", ToolUseID: "call_b", Replacement: "<persisted-output>b</persisted-output>"},
		// call_c has no record → seen but not replaced
		{Kind: "tool-result", ToolUseID: "call_X", Replacement: "<persisted-output>X</persisted-output>"}, // unknown id, skipped
	}
	s := NewContentReplacementStateFrom(candidates, records)
	// All candidates marked seen
	for _, id := range candidates {
		if !s.IsSeen(id) {
			t.Errorf("candidate %q should be marked seen after reconstruct", id)
		}
	}
	// call_a, call_b have replacements
	for id, want := range map[string]string{
		"call_a": "<persisted-output>a</persisted-output>",
		"call_b": "<persisted-output>b</persisted-output>",
	} {
		got, ok := s.Lookup(id)
		if !ok || got != want {
			t.Errorf("Lookup(%q) = (%q, %v), want (%q, true)", id, got, ok, want)
		}
	}
	// call_c is seen but no replacement
	if _, ok := s.Lookup("call_c"); ok {
		t.Errorf("call_c should have no replacement record")
	}
	if !s.IsSeen("call_c") {
		t.Errorf("call_c should still be marked seen (frozen unreplaced)")
	}
	// call_X was not a candidate → not added
	if s.IsSeen("call_X") {
		t.Errorf("call_X is not a candidate, must not be added to state")
	}
}

func TestContentReplacementStateFrom_IgnoresFutureKinds(t *testing.T) {
	// Defensive: future ContentReplacementRecord kinds (e.g. "user-text"
	// for redacted PII) must be skipped by this tool-result-only path.
	candidates := []string{"call_a"}
	records := []ContentReplacementRecord{
		{Kind: "tool-result", ToolUseID: "call_a", Replacement: "tr_ok"},
		{Kind: "user-text", ToolUseID: "call_a", Replacement: "user_text_ignored"},
	}
	s := NewContentReplacementStateFrom(candidates, records)
	got, _ := s.Lookup("call_a")
	if got != "tr_ok" {
		t.Errorf("non-tool-result kinds must be ignored, got %q", got)
	}
}
