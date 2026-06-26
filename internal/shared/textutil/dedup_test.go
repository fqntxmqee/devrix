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

// TestDedupAdjacentRepeats_collapses_short_consecutive_echoes (2026-06-26
// hotfix): the 8-rune floor in DedupRepeatedText + the hasMinUniqueRunes
// guard rejects 3-4 rune phrases as too unvaried, so short echoes like
// "让我先让我先" or "我来帮你我来帮你" slip through. This is the
// companion pass for those: find any 3-7 rune phrase that appears 2+
// times consecutively and collapse to one. Real example from a
// 2026-06-26 minimax M2.7 streaming run on the devrix review-d2 path:
//
//	"让我先让我先制定一个清晰的 plan" → "让我先制定一个清晰的 plan"
func TestDedupAdjacentRepeats_collapses_short_consecutive_echoes(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		want     string
	}{
		{
			name:  "3-rune phrase × 2 then unique tail",
			input: "让我先让我先制定一个清晰的 plan",
			want:  "让我先制定一个清晰的 plan",
		},
		{
			name:  "3-rune phrase × 3",
			input: "让我先让我先让我先",
			want:  "让我先",
		},
		{
			name:  "5-rune phrase × 2 between prose",
			input: "好的，我来帮你我来帮你看一下代码。",
			want:  "好的，我来帮你看一下代码。",
		},
		{
			name:  "no duplicate — passes through",
			input: "让我先看看项目结构",
			want:  "让我先看看项目结构",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DedupAdjacentRepeats(tc.input, 3, 7)
			if got != tc.want {
				t.Errorf("DedupAdjacentRepeats(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestDedupAdjacentRepeats_preserves_single_particle asserts that the
// function does NOT collapse a single common particle like "的" or
// "是" when it appears in prose. minRunes is clamped to 2 so the
// function ignores 1-rune repeats, but even 2-rune repeats like "我的"
// should pass through when they're not consecutive duplicates.
func TestDedupAdjacentRepeats_preserves_single_particle(t *testing.T) {
	input := "我的项目是 Go 项目，我的目标是实现 d2 编排。"
	got := DedupAdjacentRepeats(input, 3, 7)
	if got != input {
		t.Errorf("DedupAdjacentRepeats should pass through prose unchanged; got %q want %q", got, input)
	}
}

// TestStripToolCallXML_removes_function_calls_block (2026-06-26
// hotfix): minimax M2.7 streaming emits tool calls as inline XML
// inside the text channel rather than as structured tool_use events.
// Without stripping, the user sees <function_calls>, <invoke>, and
// <parameter> blocks rendered on the reply card. Real example from a
// 2026-06-26 streaming run:
//
//	"让我先\n\n<function_calls>\n<invoke name=\"bash\">\n<parameter
//	name=\"command\">pwd</parameter>\n</invoke>\n</function_calls>"
//	→ "让我先"
func TestStripToolCallXML_removes_function_calls_block(t *testing.T) {
	input := "让我先\n\n<function_calls>\n<invoke name=\"bash\">\n<parameter name=\"command\">pwd ls -la</parameter>\n</invoke>\n</function_calls>\n\n好的，我看看。"
	got := StripToolCallXML(input)
	if strings.Contains(got, "<") || strings.Contains(got, "function_calls") {
		t.Errorf("StripToolCallXML left XML in result: %q", got)
	}
	if !strings.Contains(got, "让我先") {
		t.Errorf("StripToolCallXML lost surrounding text: %q", got)
	}
	if !strings.Contains(got, "好的") {
		t.Errorf("StripToolCallXML lost tail text: %q", got)
	}
}

// TestStripToolCallXML_handles_orphan_closing_tag asserts that an
// orphan closing tag (no matching open tag, or where the open tag was
// already removed) is also stripped. minimax M2.7 streaming sometimes
// leaves a stray `</markdown>` at the tail of a chunk that the user
// sees as `-markdown>` on the live card.
func TestStripToolCallXML_handles_orphan_closing_tag(t *testing.T) {
	input := "好的，我来</markdown>看看代码"
	got := StripToolCallXML(input)
	if strings.Contains(got, "markdown") {
		t.Errorf("StripToolCallXML left orphan closing tag: %q", got)
	}
	if !strings.Contains(got, "好的") || !strings.Contains(got, "看看代码") {
		t.Errorf("StripToolCallXML lost surrounding text: %q", got)
	}
}