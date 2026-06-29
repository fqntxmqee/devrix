package interfaces

import (
	"errors"
	"strconv"
	"strings"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// HardEvidence represents the minimum evidence required for a VerdictPass.
// It is a kind-specific 5-field record; the Verified method enforces the
// rules in PR-C design §2.1 HardEvidence IV-5.
//
// PR-C IV-5: Hard Evidence kind-specific (code vs chat strict separation).
type HardEvidence struct {
	Kind           string      // "code" / "chat" / "unknown"
	TestResult     *TestResult // optional, code-relevant
	LogExcerpt     string      // optional, code-relevant
	ArtifactHash   string      // optional, code-relevant
	EntityHash     string      // optional, chat-relevant
	CoherenceScore float64     // optional, chat-relevant, [0, 1]
}

// Hard evidence task kinds (ClosedSet; PR-C design §2.1).
const (
	HardEvidenceKindCode     = "code"
	HardEvidenceKindChat     = "chat"
	HardEvidenceKindUnknown  = "unknown"
	HardEvidenceMinCoherence = 0.5
	HardEvidenceMinCoverage  = 1 // percent
)

// NewHardEvidence constructs a HardEvidence with the given kind and
// all evidence fields zero. Use the With* methods to populate fields.
func NewHardEvidence(kind string) HardEvidence {
	return HardEvidence{Kind: normalizeKind(kind)}
}

// WithKind returns a shallow copy of h with Kind replaced (normalized).
func (h HardEvidence) WithKind(kind string) HardEvidence {
	h.Kind = normalizeKind(kind)
	return h
}

// WithTestResult returns a shallow copy of h with TestResult replaced.
func (h HardEvidence) WithTestResult(t *TestResult) HardEvidence {
	h.TestResult = t
	return h
}

// WithLogExcerpt returns a shallow copy of h with LogExcerpt replaced.
func (h HardEvidence) WithLogExcerpt(s string) HardEvidence {
	h.LogExcerpt = s
	return h
}

// WithArtifactHash returns a shallow copy of h with ArtifactHash replaced.
func (h HardEvidence) WithArtifactHash(s string) HardEvidence {
	h.ArtifactHash = s
	return h
}

// WithEntityHash returns a shallow copy of h with EntityHash replaced.
func (h HardEvidence) WithEntityHash(s string) HardEvidence {
	h.EntityHash = s
	return h
}

// WithCoherenceScore returns a shallow copy of h with CoherenceScore replaced.
func (h HardEvidence) WithCoherenceScore(s float64) HardEvidence {
	h.CoherenceScore = s
	return h
}

// normalizeKind lowercases and validates the kind. Unknown values
// collapse to "unknown" to keep the ClosedSet sound.
func normalizeKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "code":
		return HardEvidenceKindCode
	case "chat":
		return HardEvidenceKindChat
	default:
		return HardEvidenceKindUnknown
	}
}

// Verified returns true if the HardEvidence satisfies the kind-specific
// minimum set. See PR-C design §2.1 HardEvidence for the rules.
//
//   - kind="code":     TestResult != nil && (CoveragePct >= 1 || LogExcerpt != "" || ArtifactHash != "")
//   - kind="chat":     CoherenceScore >= 0.5 || EntityHash != ""
//   - kind="unknown":  same as "code" (conservative)
//
// IV-5: kind-specific 严格分离 (chat is NOT subjected to test/log/artifact checks).
func (h HardEvidence) Verified() bool {
	switch h.Kind {
	case HardEvidenceKindChat:
		return h.CoherenceScore >= HardEvidenceMinCoherence || h.EntityHash != ""
	case HardEvidenceKindCode, HardEvidenceKindUnknown:
		if h.TestResult != nil && h.TestResult.CoveragePct >= HardEvidenceMinCoverage {
			return true
		}
		if strings.TrimSpace(h.LogExcerpt) != "" {
			return true
		}
		if strings.TrimSpace(h.ArtifactHash) != "" {
			return true
		}
		return false
	default:
		// Defensive: unknown kinds default to conservative (code-like).
		if h.TestResult != nil && h.TestResult.CoveragePct >= HardEvidenceMinCoverage {
			return true
		}
		if strings.TrimSpace(h.LogExcerpt) != "" {
			return true
		}
		if strings.TrimSpace(h.ArtifactHash) != "" {
			return true
		}
		return false
	}
}

// Evidence is the existing report-side evidence aggregate (declared in
// task_report.go / evidence.go). We re-declare a minimal projection here
// for clarity. The actual ExtractHardEvidence helper below uses the real
// Evidence struct at runtime.
//
// TestResult is the upstream test-summary record. CoveragePct is the
// numeric coverage percentage (0-100). 0 means "no test ran".
type TestResult struct {
	CoveragePct int  // 0-100; 0 means "no test ran"
	Passed      bool // true if last test invocation passed
}

// ExtractHardEvidenceFromEvidence bridges the upstream *Evidence (declared in
// task_report.go with string-based TestResult / LogExcerpt / ArtifactHash) to
// a HardEvidence suitable for Verified(). It parses the human-readable
// TestResult string for a "coverage XX%" pattern (e.g. "5/5 pass, coverage 87%").
//
// The upstream Evidence carries only code-relevant fields. Chat-relevant fields
// (CoherenceScore, EntityHash) are absent; callers handling chat tasks must
// override Kind via WithKind("chat") and populate those fields via
// WithCoherenceScore / WithEntityHash.
//
// Kind inference: defaults to "code" (conservative).
func ExtractHardEvidenceFromEvidence(ev *Evidence) HardEvidence {
	h := NewHardEvidence(HardEvidenceKindCode)
	if ev == nil {
		return h
	}
	if ev.TestResult != "" {
		if cov, ok := parseCoveragePercent(ev.TestResult); ok {
			h.TestResult = &TestResult{
				CoveragePct: cov,
				Passed:      isPassedTestResult(ev.TestResult),
			}
		}
	}
	if ev.LogExcerpt != "" {
		h.LogExcerpt = ev.LogExcerpt
	}
	if ev.ArtifactHash != "" {
		h.ArtifactHash = ev.ArtifactHash
	}
	return h
}

// parseCoveragePercent extracts the leading integer from a "coverage XX%" or
// "coverage: XX%" phrase in a human-readable test summary. Returns (0, false)
// if no "coverage" keyword is found or no numeric value follows the keyword.
func parseCoveragePercent(s string) (int, bool) {
	lower := strings.ToLower(s)
	idx := strings.Index(lower, "coverage")
	if idx < 0 {
		return 0, false
	}
	rest := s[idx+len("coverage"):]
	rest = strings.TrimLeft(rest, " :-")
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0, false
	}
	return n, true
}

// isPassedTestResult best-effort detects "pass" / "passed" / "ok" markers in a
// human-readable TestResult string. Conservative: returns false on no match.
func isPassedTestResult(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "pass") || strings.Contains(lower, " ok")
}

// Note: *Evidence referenced by ExtractHardEvidenceFromEvidence is the upstream
// type declared in task_report.go (string-based TestResult / LogExcerpt /
// ArtifactHash, plus CoherenceScore / EntityHash reserved for chat-tagged
// reports). It is reused here to keep the interfaces package independent of
// any D7 sub-package imports (IV-1: pure types).

// Errors (PR-C 7122 range, sharederrors.WithCode pattern).

// ErrHardEvidenceMissing is returned when a VerdictPass verdict lacks
// the minimum kind-specific evidence.
var ErrHardEvidenceMissing = errors.New("interfaces: hard evidence missing for Pass verdict")

// NewHardEvidenceMissingError is the canonical wrap helper for
// ORCH_HARD_EVIDENCE_MISSING_7122.
func NewHardEvidenceMissingError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_HARD_EVIDENCE_MISSING_7122",
		"hard evidence missing for Pass verdict",
		ErrHardEvidenceMissing,
	)
}
