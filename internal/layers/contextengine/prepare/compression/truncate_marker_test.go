// T: D2-S15-A02-T13 — TruncateWithMarker unit tests.
package compression

import (
	"strings"
	"testing"
)

const testMarker = "[TRUNCATED at %d/%d chars, complete=false, REREAD may help]"

// D2-S15-A02-T13: short output → no marker, no truncation.
func TestMarker_ShortOutputNoMarker(t *testing.T) {
	text := "hello world"
	out, truncated := TruncateWithMarker(text, 100, testMarker)
	if truncated {
		t.Errorf("short text should not be truncated")
	}
	if out != text {
		t.Errorf("short text should be returned unchanged, got %q", out)
	}
}

// D2-S15-A02-T13: long output → marker appended, complete=false visible.
func TestMarker_AlwaysAppended(t *testing.T) {
	text := strings.Repeat("a", 1000) // 1000 chars
	out, truncated := TruncateWithMarker(text, 200, testMarker)
	if !truncated {
		t.Errorf("long text should be truncated")
	}
	if !strings.Contains(out, "complete=false") {
		t.Errorf("truncated output must contain 'complete=false' marker, got %q", out)
	}
	if !strings.Contains(out, "[TRUNCATED") {
		t.Errorf("truncated output must contain '[TRUNCATED' marker prefix, got %q", out)
	}
}

// D2-S15-A02-T13: marker has correct (kept, total) positions.
func TestMarker_PositionsCorrect(t *testing.T) {
	text := strings.Repeat("a", 1000)
	out, _ := TruncateWithMarker(text, 200, testMarker)
	if !strings.Contains(out, "/1000 chars") {
		t.Errorf("marker should report total=1000, got %q", out)
	}
}

// D2-S15-A02-T13: maxChars=0 → no truncation (no enforcement).
func TestMarker_ZeroMaxNoTruncate(t *testing.T) {
	text := strings.Repeat("a", 1000)
	out, truncated := TruncateWithMarker(text, 0, testMarker)
	if truncated {
		t.Errorf("maxChars=0 should not truncate")
	}
	if out != text {
		t.Errorf("maxChars=0 should return text unchanged")
	}
}

// D2-S15-A02-T13: maxChars=1 (tiny) → still has marker.
func TestMarker_VerySmallMax(t *testing.T) {
	text := "this is a long string that needs truncation"
	out, truncated := TruncateWithMarker(text, 1, testMarker)
	if !truncated {
		t.Errorf("maxChars=1 should trigger truncation")
	}
	if !strings.Contains(out, "complete=false") {
		t.Errorf("even with maxChars=1, marker must be present")
	}
}

// D2-S15-A02-T13: SanitizeMarker rejects empty marker.
func TestSanitizeMarker_EmptyRejected(t *testing.T) {
	err := SanitizeMarker("")
	if err == nil {
		t.Errorf("empty marker should be rejected")
	}
}

// D2-S15-A02-T13: SanitizeMarker rejects marker without complete=false.
func TestSanitizeMarker_MissingCompleteFalse(t *testing.T) {
	err := SanitizeMarker("[truncated]")
	if err == nil {
		t.Errorf("marker without 'complete=false' should be rejected")
	}
}

// D2-S15-A02-T13: SanitizeMarker accepts marker with complete=false.
func TestSanitizeMarker_Valid(t *testing.T) {
	err := SanitizeMarker(testMarker)
	if err != nil {
		t.Errorf("valid marker should be accepted, got %v", err)
	}
}

// D2-S15-A02-T13: integration with default ToolSpec v3 marker.
func TestMarker_DefaultMarkerTemplate(t *testing.T) {
	const defaultMarker = "[TRUNCATED at %d/%d chars, complete=false, REREAD may help]"
	if err := SanitizeMarker(defaultMarker); err != nil {
		t.Errorf("default marker should pass SanitizeMarker, got %v", err)
	}
}
