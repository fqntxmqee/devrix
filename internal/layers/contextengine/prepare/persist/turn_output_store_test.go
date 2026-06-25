package persist

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFoldAssistantOutput_BelowLimit_PassesThrough(t *testing.T) {
	store := NewToolResultStore(t.TempDir())
	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", "short", 1000, 800, 200)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}
	if got != "short" {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestFoldAssistantOutput_AboveLimit_FoldsHeadTail(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// 2000 chars; limit 100; head 50; tail 30.
	content := strings.Repeat("abcdefghij", 200)
	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 100, 50, 30)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}

	if !strings.Contains(got, "<prior-output-summary>") {
		t.Errorf("expected summary marker, got %q", got)
	}
	if !strings.Contains(got, "chars truncated; see ") {
		t.Errorf("expected truncation marker, got %q", got)
	}
	if !strings.Contains(got, "/turn-outputs/t1-") {
		t.Errorf("expected turn-outputs path, got %q", got)
	}

	// File must exist on disk.
	sessDir := filepath.Join(dir, "sess1", "turn-outputs")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", sessDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	fullPath := filepath.Join(sessDir, entries[0].Name())
	body, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != content {
		t.Errorf("file body mismatch: got %d bytes want %d", len(body), len(content))
	}
}

func TestFoldAssistantOutput_NilStore_StillTruncates(t *testing.T) {
	content := strings.Repeat("abcdefghij", 100)
	got, err := FoldAssistantOutput(nil, "sess1", 1, "assistant", content, 100, 50, 30)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}
	if !strings.Contains(got, "no store available") {
		t.Errorf("expected no-store marker, got %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncation marker, got %q", got)
	}
}

func TestFoldAssistantOutput_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// No max/head/tail → defaults.
	content := strings.Repeat("x", 20000)
	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 0, 0, 0)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}
	if !strings.Contains(got, "<prior-output-summary>") {
		t.Errorf("expected summary marker with defaults, got %q", got)
	}
}

func TestTruncateTail(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "llo"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"😀😁😂🤣😃", 2, "🤣😃"},
	}
	for _, tt := range tests {
		if got := truncateTail(tt.s, tt.n); got != tt.want {
			t.Errorf("truncateTail(%q, %d): got %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

// _ = context is to silence unused-import warnings if a test is later
// deleted; keeps the file compilable across edits.
var _ = context.Background

// TestFoldAssistantOutput_DedupsLLMLoopInFoldedHead is the regression
// guard for DM-20260625-008 root-cause fix: when the LLM emits a short
// streaming loop in the head of an oversized assistant reply, the
// duplicate MUST be collapsed before the content is folded into the
// next-turn prompt. Without this guard the loop is carried verbatim
// into <prior-output-summary> and biases the next LLM call toward
// replaying the same loop (pattern lock), which is exactly why the
// "fix P0 issues" follow-up was misunderstood in sess_1782381569430_3000.
func TestFoldAssistantOutput_DedupsLLMLoopInFoldedHead(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// Loop matching the minimax M2.7 streaming-loop pattern. Length
	// must exceed the fold dedup threshold (30 runes).
	loop := "优先修复 — Work focus 已经标注。让我先找出 D2 域当前的 P0 问题。"
	if n := utf8.RuneCountInString(loop); n < 30 {
		t.Fatalf("fixture invariant: expected loop length >= 30 runes, got %d", n)
	}

	// Two consecutive copies of the loop at the very start, then a
	// long middle (well past head=800), then a stable tail.
	middle := strings.Repeat("中", 8500)
	tail := "结论: 修复 P0 fold dedup"
	content := loop + "\n\n" + loop + "\n\n" + middle + tail

	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 1000, 800, 200)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}

	// Core assertion: the duplicate loop was collapsed before being
	// placed in the fold summary, so the loop appears exactly once
	// in the result instead of twice.
	if count := strings.Count(got, loop); count != 1 {
		t.Errorf("expected loop to appear exactly once after fold dedup, got %d occurrences in:\n%s", count, got)
	}

	// Sanity: truncation marker and tail survive dedup.
	if !strings.Contains(got, "chars truncated; see ") {
		t.Errorf("expected truncation marker, got %q", got)
	}
	if !strings.Contains(got, tail) {
		t.Errorf("expected tail preserved in fold summary, got %q", got)
	}
}

// TestFoldAssistantOutput_DedupsLLMLoopInFoldedTail verifies the
// symmetric case: when the duplicate sits at the tail end of the
// content, the fold summary's tail segment is also deduped so the
// loop does not propagate forward.
func TestFoldAssistantOutput_DedupsLLMLoopInFoldedTail(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	loop := "优先修复 — Work focus 已经标注。让我先找出 D2 域当前的 P0 问题。"
	if n := utf8.RuneCountInString(loop); n < 30 {
		t.Fatalf("fixture invariant: expected loop length >= 30 runes, got %d", n)
	}

	middle := strings.Repeat("中", 8500)
	// Loop duplicated at the tail.
	content := "starts with normal prose\n\n" + middle + "\n\n" + loop + "\n\n" + loop

	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 1000, 800, 200)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}

	if count := strings.Count(got, loop); count != 1 {
		t.Errorf("expected loop to appear exactly once after fold dedup, got %d occurrences in:\n%s", count, got)
	}
}

// TestFoldAssistantOutput_DoesNotDedupAcrossSegments ensures dedup is
// applied per-segment (head and tail separately) and never matches
// across the fold boundary. If it matched across, the truncation
// marker would lose content on either side.
func TestFoldAssistantOutput_DoesNotDedupAcrossSegments(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// Same phrase at head and at tail. Per-segment dedup should NOT
	// consider these as duplicates of each other; they survive both
	// dedup passes independently.
	phrase := "P0 fold dedup missing across turn boundary"
	if n := utf8.RuneCountInString(phrase); n < DefaultFoldDedupMinDup {
		t.Fatalf("fixture invariant: phrase must exceed dedup threshold (%d runes), got %d", DefaultFoldDedupMinDup, n)
	}

	middle := strings.Repeat("x", 9000)
	content := phrase + "\n\n" + middle + "\n\n" + phrase

	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 1000, 800, 200)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}

	// Both copies should survive: one in head, one in tail. If
	// cross-segment dedup ran, only one would remain.
	if count := strings.Count(got, phrase); count != 2 {
		t.Errorf("expected phrase to appear twice (head + tail), got %d occurrences in:\n%s", count, got)
	}
}