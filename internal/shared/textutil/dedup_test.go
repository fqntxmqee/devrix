package textutil

import (
	"strings"
	"testing"
)

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

// TestDedupRepeatedTextIterative_collapses_3_to_5_copies (2026-06-26
// hotfix): single-pass DedupRepeatedText only removes ONE longest-LCP
// pair, so a buffer where the same phrase appears 3+ times keeps the
// leftovers. Iterating collapses all copies. Real example from a
// 2026-06-26 minimax M2.7 streaming run on the devrix review-d2 path:
// the LLM emitted "我来帮你review" 4-5 times across consecutive chunks
// and the user saw the duplicate paragraph on the feishu card.
//
// Threshold lowered to 8 (the algorithm's floor) to catch the short
// 8-15 rune echoes that drove this fix. The hasMinUniqueRunes guard
// inside DedupRepeatedText prevents false positives on common
// particles like 的/是 at this threshold.
func TestDedupRepeatedTextIterative_collapses_3_to_5_copies(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		inputRuns int // rune count of input, asserted to shrink
		maxOne    []string
		wantKeep  string
	}{
		{
			name: "5 copies of the same opening",
			input: "我来帮你review d2领域的代码。首先让我了解项目结构。" +
				"我来帮你review d2领域的代码。让我先找到d2相关的代码。" +
				"我来帮你review 我来帮你review 我来帮你review 先找到d2相关的代码。",
			maxOne:   []string{"我来帮你review d2领域的代码", "我来帮你review 我来帮你review 我来帮你review"},
			wantKeep: "首先让我了解项目结构",
		},
		{
			name: "3 copies with a tail that must survive",
			input: "看起来没有直接叫d2的目录。让我换一种方式搜索。" +
				"看起来没有直接叫d2的目录。让我先了解d2的真正含义。" +
				"看起来没有直接叫d2的目录。我来领域相关的代码。" +
				"我先从核心架构开始。",
			maxOne:   []string{"看起来没有直接叫d2的目录"},
			wantKeep: "我先从核心架构开始",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputRuns := len([]rune(tc.input))
			iter := DedupRepeatedTextIterative(tc.input, 8, 2, 8)
			if len([]rune(iter)) >= inputRuns {
				t.Fatalf("iterative dedup did not shrink buffer: in=%d runes, out=%d runes",
					inputRuns, len([]rune(iter)))
			}
			for _, sub := range tc.maxOne {
				if c := strings.Count(iter, sub); c > 1 {
					t.Errorf("substring %q appears %d times after dedup, want ≤ 1; result=%q",
						sub, c, iter)
				}
			}
			if tc.wantKeep != "" && !strings.Contains(iter, tc.wantKeep) {
				t.Errorf("surviving tail %q missing from result=%q", tc.wantKeep, iter)
			}
		})
	}
}

// TestDedupRepeatedTextIterative_stops_at_maxPasses ensures the cap
// prevents runaway cost on adversarial inputs. A buffer of "我要开始工作了"×20
// is fully dedup-able in a few passes; the cap must still collapse the
// bulk of repeats without over-collapsing (the last occurrence must
// remain — that is the unique survivor).
func TestDedupRepeatedTextIterative_stops_at_maxPasses(t *testing.T) {
	phrase := "我要开始工作了" // 7 runes — below floor 8, so dedup is a no-op
	// Use a phrase that crosses the floor.
	phrase = "我要开始工作了请准备好" // 11 runes
	input := strings.Repeat(phrase, 20)
	got := DedupRepeatedTextIterative(input, 8, 2, 4)
	if !strings.Contains(got, phrase) {
		t.Fatalf("iterative dedup over-collapsed; got %q", got)
	}
	if c := strings.Count(got, phrase); c >= 20 {
		t.Fatalf("iterative dedup did not converge under maxPasses=4; phrase count=%d (input had 20)",
			c)
	}
}