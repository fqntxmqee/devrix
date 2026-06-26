package textutil

// DedupRepeatedText removes a duplicate long substring from buffer. It
// finds the longest repeated substring S of length ≥ minDupRunes that
// appears at two non-overlapping positions, then removes the second
// occurrence (and keeps the prefix and any tail). Returns the original
// buffer unchanged when no qualifying duplicate is found.
//
// This is the cross-domain safety net for LLM streaming loops: when
// the provider emits the same long phrase twice in a single reply,
// both the visible reply card (D1) and the fold summary injected into
// the next-turn prompt (D2 FoldAssistantOutput) need to collapse the
// duplicate so it does not propagate to the user or bias the next
// LLM call.
//
// minDupRunes is clamped to a floor of 8 to avoid pathological inputs
// matching arbitrary short common words ("the", "and"). The O(n^2)
// scan is fine for buffer sizes up to ~64K runes (a reply card).
func DedupRepeatedText(buffer string, minDupRunes, minGapRunes int) string {
	if minDupRunes < 8 {
		minDupRunes = 8
	}
	if minGapRunes < 0 {
		minGapRunes = 0
	}
	runes := []rune(buffer)
	n := len(runes)
	if n < minDupRunes*2+minGapRunes {
		return buffer
	}
	// Find the longest LCP pair (i, j) with i < j, j - i ≥ minDupRunes,
	// and j - (i + LCP) ≥ minGapRunes. The first and second occurrences
	// of the matched substring are at i and j.
	bestLen := 0
	bestSecondStart := 0
	for i := 0; i <= n-minDupRunes*2-minGapRunes; i++ {
		// j must be far enough from i that a non-overlapping LCP of
		// length ≥ minDupRunes is possible. j ≥ i + minDupRunes
		// means the two occurrences can share at most minDupRunes-1
		// leading characters — the LCP we measure starts from i, so
		// it can extend across j without false overlap.
		for j := i + minDupRunes; j <= n-minDupRunes; j++ {
			if runes[i] != runes[j] {
				continue
			}
			k := 0
			for i+k < n && j+k < n && runes[i+k] == runes[j+k] {
				k++
			}
			// Require the LCP to overlap both occurrences strictly so
			// the dedup is non-trivial (k > 0 already guaranteed by
			// the runes[i]==runes[j] start condition). We do NOT
			// require j+k < n — a buffer that ends with a duplicate
			// (e.g. a short loop appended to a unique tail) is a
			// legitimate dedup target and the result simply drops the
			// second occurrence, leaving a shorter buffer with no
			// information loss on the unique prefix.
			if k <= bestLen {
				continue
			}
			// Require enough unique runes in the candidate LCP so a long
			// run of identical padding (e.g. "中"*356) does not beat a
			// shorter but real duplicate sentence.
			if !hasMinUniqueRunes(runes[i:i+k], minUniqueRunes(k)) {
				continue
			}
			bestLen = k
			bestSecondStart = j
		}
	}
	if bestLen < minDupRunes {
		return buffer
	}
	// Remove the duplicate block at [bestSecondStart, bestSecondStart+bestLen),
	// keeping both the prefix before the second occurrence and any
	// legitimate tail that may continue after it.
	result := make([]rune, 0, n-bestLen)
	result = append(result, runes[:bestSecondStart]...)
	result = append(result, runes[bestSecondStart+bestLen:]...)
	return string(result)
}

// DedupRepeatedTextIterative applies DedupRepeatedText repeatedly until
// the buffer stops shrinking or maxPasses is reached. The single-pass
// DedupRepeatedText only removes ONE longest-LCP pair, so a buffer
// where the same phrase appears 3+ times (e.g. minimax M2.7 streaming
// loops: "我来帮你review" emitted 4-5 times) keeps the leftover
// duplicates after one pass. Iterating collapses the rest. Each pass
// is O(n^2) on the shrinking buffer, so we cap maxPasses.
//
// maxPasses <= 0 is treated as 8 — a generous cap that handles the
// 4-5x duplicates observed in practice without risking runaway cost
// on adversarial inputs.
func DedupRepeatedTextIterative(buffer string, minDupRunes, minGapRunes, maxPasses int) string {
	if maxPasses <= 0 {
		maxPasses = 8
	}
	for pass := 0; pass < maxPasses; pass++ {
		next := DedupRepeatedText(buffer, minDupRunes, minGapRunes)
		if next == buffer {
			return next
		}
		buffer = next
	}
	return buffer
}

// minUniqueRunes returns the minimum number of unique runes that a
// candidate LCP of length k must contain to be considered a real
// duplicate. The threshold combines a fixed floor of 5 (so short LCPs
// still need some variety) with a k/30 ratio (so long padding runs
// are rejected). Calibrated on three fixtures:
//
//   - 43-rune minimax M2.7 streaming loop with ~30 unique runes:
//     5 + 43/30 ≈ 6, threshold passes.
//   - 225-rune natural-language duplicate with prefix + x*200 +
//     "结束。": ~23 unique runes, 5 + 225/30 ≈ 12, threshold passes.
//   - 356-rune "中"*356 padding run: 1 unique rune, 5 + 356/30 ≈ 16,
//     threshold rejects.
func minUniqueRunes(k int) int {
	return 5 + k/30
}

// hasMinUniqueRunes reports whether rs contains at least threshold
// distinct runes. Implemented with a small map because the LCP is
// already bounded to ~64K runes.
func hasMinUniqueRunes(rs []rune, threshold int) bool {
	seen := make(map[rune]struct{}, threshold+1)
	for _, r := range rs {
		seen[r] = struct{}{}
		if len(seen) >= threshold {
			return true
		}
	}
	return false
}