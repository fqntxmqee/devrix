package textutil

import "testing"

// TestDedupRepeatedText_catches_30rune_loop verifies the cross-domain
// dedup threshold is tight enough to catch the minimax M2.7 short-loop
// pattern that drove the DM-20260625-008 fix. The repeated phrase must
// exceed the new 30-rune minimum. With the old 60-rune threshold this
// scenario would silently pass through unchanged; with 30 the
// duplicate is collapsed and the loop is gone from the buffer.
func TestDedupRepeatedText_catches_30rune_loop(t *testing.T) {
	loop := "优先修复 — Work focus 已经标注。让我先找出 D2 域当前的 P0 问题。"
	//nolint:gosec // G101 false positive — fixture, not a credential.
	if n := len([]rune(loop)); n < 30 {
		t.Fatalf("fixture invariant: expected loop length >= 30 runes, got %d", n)
	}

	input := loop + "\n\n" + loop + "\n\nvisible tail"

	got := DedupRepeatedText(input, 30, 2)
	want := loop + "\n\nvisible tail"
	if got != want {
		t.Fatalf("DedupRepeatedText() = %q, want %q", got, want)
	}
}

// TestDedupRepeatedText_passes_through_short_input ensures dedup is a
// no-op when the buffer is too small to contain a qualifying duplicate.
// Without this guard the O(n^2) scan would still run but find nothing;
// the guard short-circuits it.
func TestDedupRepeatedText_passes_through_short_input(t *testing.T) {
	input := "short answer"
	got := DedupRepeatedText(input, 30, 2)
	if got != input {
		t.Fatalf("DedupRepeatedText() short-circuit failed: got %q, want %q", got, input)
	}
}

// TestDedupRepeatedText_keeps_legitimate_tail ensures dedup only
// removes the duplicate block and preserves any legitimate content
// after the second occurrence.
func TestDedupRepeatedText_keeps_legitimate_tail(t *testing.T) {
	const dup = "I will check the code for any issues" // 35 runes
	//nolint:gosec // G101 false positive — fixture, not a credential.
	if n := len([]rune(dup)); n < 30 {
		t.Fatalf("fixture invariant: expected dup length >= 30 runes, got %d", n)
	}

	input := dup + "\n\n" + dup + "\n\n# Findings\n\nP0: fold dedup missing"
	got := DedupRepeatedText(input, 30, 2)
	want := dup + "\n\n# Findings\n\nP0: fold dedup missing"
	if got != want {
		t.Fatalf("DedupRepeatedText() = %q, want %q", got, want)
	}
}