// Package compression provides D2 context-engine compression primitives,
// including the治本 TruncateWithMarker (D2-S15-A02-T13).
//
// The TruncateWithMarker function is the second half of the 8K token
// self-loop fix (demand.md RC-2): when D2 truncates a tool result, the
// LLM MUST see a clearly-visible marker that the result is incomplete.
// Without this, the LLM cannot distinguish "complete result" from
// "truncated result" — Akerlof 1970 signal pooling.
package compression

import (
	"fmt"
	"strings"
)

// TruncateWithMarker truncates text to at most maxChars runes. If the
// truncation occurs, a marker is appended that makes the truncation
// visible to the LLM. The marker template MUST contain the literal
// "complete=false" so a downstream parser (or the LLM itself) can
// detect the truncation.
//
// Returns (truncated_text, was_truncated).
//   - was_truncated == false: text is returned unchanged
//   - was_truncated == true: text is truncated to <= maxChars runes
//     (including the marker), and a marker is appended
//
// The marker template uses printf-style %d for the (kept, total) char
// positions. The default template is "[TRUNCATED at %d/%d chars,
// complete=false, REREAD may help]". Custom templates MUST preserve
// the "complete=false" substring (verified at registration time by
// the ToolSpec v3 surface gate; see orthogonal_flags_test.go).
//
// DSAFT: D2-S15-A02-T13.
func TruncateWithMarker(text string, maxChars int, marker string) (string, bool) {
	if maxChars <= 0 {
		// maxChars=0 means "no truncation requested" — return as-is.
		return text, false
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text, false
	}

	// Reserve room for the marker. The marker is short (typically ~60
	// chars), so subtract its approximate length from the budget.
	markerRunes := []rune(marker)
	// If maxChars is smaller than the marker, we have a problem:
	// the治本 narrative REQUIRES the marker to be visible. In that case,
	// return a truncated result that includes the marker (at the cost of
	// exceeding maxChars). The LLM still sees the "complete=false" signal.
	keepBudget := maxChars - len(markerRunes) - 4 // 4 for "0123" worst-case width
	if keepBudget < 0 {
		// maxChars < marker length: degrade gracefully — keep the marker
		// (it's more important than content) and just include 0 chars of
		// the original text. This is an edge case; callers SHOULD ensure
		// maxChars >= marker length for normal use.
		keepBudget = 0
	}
	if keepBudget > len(runes) {
		keepBudget = len(runes)
	}
	kept := string(runes[:keepBudget])
	filledMarker := fmt.Sprintf(marker, keepBudget, len(runes))
	// Defensive: ensure the result fits in maxChars.
	result := kept + filledMarker
	if len([]rune(result)) > maxChars {
		// Last-resort: cut content further.
		overBy := len([]rune(result)) - maxChars
		newKeep := keepBudget - overBy
		if newKeep < 0 {
			newKeep = 0
		}
		kept = string(runes[:newKeep])
		filledMarker = fmt.Sprintf(marker, newKeep, len(runes))
		result = kept + filledMarker
	}
	return result, true
}

// TruncateShortOutputNoMarker is a no-op helper for tools whose output
// is shorter than maxChars. It returns the original text with
// was_truncated=false. Provided for symmetry with TruncateWithMarker in
// cases where callers want a single function name.
//
// DSAFT: D2-S15-A02-T13 companion (TestShortOutputNoMarker).
func TruncateShortOutputNoMarker(text string, maxChars int, marker string) (string, bool) {
	return TruncateWithMarker(text, maxChars, marker)
}

// SanitizeMarker validates that a marker template contains the literal
// "complete=false" substring. The ToolSpec v3 surface gate enforces this
// (orthogonal_flags_test.go), but this helper is also exposed for
// callers that construct markers at runtime.
//
// Returns nil if the marker is valid; an error explaining the violation
// otherwise. Empty marker is treated as a violation (the治本 narrative
// requires visibility).
func SanitizeMarker(marker string) error {
	if marker == "" {
		return fmt.Errorf("compression: empty marker (LLM cannot detect truncation)")
	}
	if !strings.Contains(marker, "complete=false") {
		return fmt.Errorf("compression: marker missing 'complete=false' substring (LLM cannot distinguish complete from truncated)")
	}
	return nil
}
