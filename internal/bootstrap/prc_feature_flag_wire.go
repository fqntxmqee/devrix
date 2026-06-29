// PR-C Feature Flag wiring helpers (DM-20260629-009).
//
// Status: env helper + factory landed in PR-C. Production wire of the
// HardEvidence reject path and Similarity Check intercept is wired in
// PR-C S4-3 (verifier.go / decomposer.go). When the flags are unset,
// the helper returns the default-disabled guard so PR-C ships with
// 0 behavior change.
//
// Feature Flag: D7_HARD_EVIDENCE_ENABLED
//   - "true" | "1" | "yes" | "on" → enabled (verifier rejects Pass with insufficient evidence)
//   - anything else (incl. unset) → disabled (default; verifier behavior unchanged)
//
// Feature Flag: D7_SIMILARITY_CHECK_ENABLED
//   - "true" | "1" | "yes" | "on" → enabled (decomposer intercepts Jaccard > InterceptThreshold)
//   - anything else (incl. unset) → disabled (default; decomposer behavior unchanged)
package bootstrap

import (
	"os"
	"strings"
)

// HardEvidenceEnabled reports the value of D7_HARD_EVIDENCE_ENABLED.
// Default false → 0 behavior change in PR-C.
func HardEvidenceEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("D7_HARD_EVIDENCE_ENABLED")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// SimilarityCheckEnabled reports the value of D7_SIMILARITY_CHECK_ENABLED.
// Default false → 0 behavior change in PR-C.
func SimilarityCheckEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("D7_SIMILARITY_CHECK_ENABLED")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
