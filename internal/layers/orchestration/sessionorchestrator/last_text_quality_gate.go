package sessionorchestrator

import (
	"context"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
)

// DM-20260630-011 (devrix-session-conclusion-completeness) —
// LastTextQualityGate classifies the LLM's last-turn text (which becomes
// the IM "任务总结" card content) BEFORE it leaves D7. The result drives
// two downstream behaviours:
//
//  1. meta["summary_quality"] on the terminal `complete` event — IM
//     adapters (Feishu) can render a different card when the summary is
//     structurally thin (e.g. show "暂无结论" + fall back to Content).
//  2. summary override: when the summary is too_short / inconclusive,
//     the IM adapter path will eventually need to fall back to event.Content
//     (or stats). We surface the classification but PRESERVE the original
//     text on meta["summary"] — D1 EmitComplete owns the actual fallback
//     decision so we don't lose information by collapsing early.
//
// Why a separate classifier (vs. reusing executionflow/verify.DetectEmptyConclusion):
//   - DetectEmptyConclusion is a binary detector (triggered or not) that
//     gates the AutoClose verdict — it's a hard signal that the round
//     should fail-closed. LastTextQualityGate is a soft signal that just
//     informs downstream rendering.
//   - The two CAN share the same kind vocabulary (inconclusive, etc.) so
//     dashboards correlate them in Jaeger.
//
// Kind taxonomy:
//
//	valid         — runeLen ≥ 400 (no marker match) → real review findings
//	thin          — 200 ≤ runeLen < 400            → suspected incomplete but plausible
//	too_short     — runeLen < 100                  → pipeline artifact / nothing to say
//	inconclusive  — short text AND template marker → planning recap leaked as summary

// SummaryQualityKind enumerates the 4 classification outcomes.
type SummaryQualityKind string

const (
	SummaryQualityValid        SummaryQualityKind = "valid"
	SummaryQualityThin         SummaryQualityKind = "thin"
	SummaryQualityTooShort     SummaryQualityKind = "too_short"
	SummaryQualityInconclusive SummaryQualityKind = "inconclusive"
)

// LastTextQuality thresholds (rune count). NOT magic numbers — these
// anchor real empirical brackets:
//
//	< 100 → too_short: a "summary" shorter than ~50 chars Chinese /
//	         ~25 words English almost never conveys a real finding
//	< 400 → thin: a real review verdict typically needs ≥ 400 chars Chinese
//	         to convey context + finding + recommendation; anything shorter
//	         is at least suspect
//	≥ 400 → valid: default threshold; long enough to be a real summary
//
// Thresholds chosen to match the same 800-rune cap used by
// executionflow/verify.DetectEmptyConclusion (emptyConclusionMaxRunes)
// so the two classifiers share consistent noise boundaries.
const (
	lastTextQualityTooShortMaxRunes = 100
	lastTextQualityThinMaxRunes     = 400
)

// lastTextQualityMarkers mirrors the emptyConclusionMarkers list from
// executionflow/verify/anomaly_kind_incomplete.go. Kept as a local copy
// (not an import) because the two classifiers use different policies:
// DetectEmptyConclusion short-circuits at marker presence; here we use
// markers to UPGRADE a short classification to "inconclusive" (marker +
// short together is the strongest signal of a planning artifact).
var lastTextQualityMarkers = []string{
	"<scope_contract>",
	"<directive_template>",
	"<task_recap>",
	"<planning>",
	"let me continue",
	"let me read",
	"let me explore",
	"i'll examine",
	"i will examine",
	"继续探索",
	"继续查看",
	"继续阅读",
}

// LastTextQualityResult is the output of ClassifyLastTextQuality. The
// Immutable contract: receiver modifications return a new struct.
type LastTextQualityResult struct {
	Kind    SummaryQualityKind
	RuneLen int
}

// ClassifyLastTextQuality applies the structural rules above to a
// resolved summary text and returns the classification + rune count.
// Designed to be zero-allocation on the hot path (no regex).
func ClassifyLastTextQuality(summary string) LastTextQualityResult {
	trimmed := strings.TrimSpace(summary)
	runeLen := len([]rune(trimmed))

	// Floor: completely empty resolvedSummary is too_short (matches the
	// D1 EmitComplete "summary == '' → fallback to Content" branch).
	if runeLen == 0 {
		return LastTextQualityResult{Kind: SummaryQualityTooShort, RuneLen: 0}
	}
	if runeLen < lastTextQualityTooShortMaxRunes {
		return LastTextQualityResult{Kind: SummaryQualityTooShort, RuneLen: runeLen}
	}
	if runeLen < lastTextQualityThinMaxRunes {
		// Short but non-empty: if a template marker is also present, this
		// is the planning-artifact-leaked case → upgrade to inconclusive.
		if containsLastTextMarker(trimmed) {
			return LastTextQualityResult{Kind: SummaryQualityInconclusive, RuneLen: runeLen}
		}
		return LastTextQualityResult{Kind: SummaryQualityThin, RuneLen: runeLen}
	}
	// Long text: still check for markers — a 5000-rune template dump is
	// anomalous but observable in Jaeger. Classify as inconclusive so
	// dashboards can alert on the marker presence (not just short text).
	if containsLastTextMarker(trimmed) {
		return LastTextQualityResult{Kind: SummaryQualityInconclusive, RuneLen: runeLen}
	}
	return LastTextQualityResult{Kind: SummaryQualityValid, RuneLen: runeLen}
}

// EmitLastTextQuality classifies + emits the D7 lasttext.quality_gate span.
// The end func is fire-and-forget; on the impl path it ends the span with
// or without an error.
func EmitLastTextQuality(ctx context.Context, sessionID string, summary string, exitReason string) LastTextQualityResult {
	res := ClassifyLastTextQuality(summary)
	end := hardening.EmitLastTextQualityGate(
		ctx,
		sessionID,
		string(res.Kind),
		res.RuneLen,
		exitReason,
	)
	end(nil)
	return res
}

// containsLastTextMarker is the case-insensitive substring check used
// by ClassifyLastTextQuality. Local copy of verify.containsMarker to
// avoid dragging orchtypes dependency into the hot classifier path;
// identical implementation for behavioural parity.
func containsLastTextMarker(text string) bool {
	if len(text) == 0 {
		return false
	}
	for _, marker := range lastTextQualityMarkers {
		if len(marker) == 0 || len(text) < len(marker) {
			continue
		}
		if containsSubstringCI(text, marker) {
			return true
		}
	}
	return false
}

// containsSubstringCI does a case-insensitive substring search without
// strings.ToLower allocations. Same algorithm as verify.containsMarker.
func containsSubstringCI(text, sub string) bool {
	if len(sub) == 0 || len(text) < len(sub) {
		return false
	}
	for i := 0; i+len(sub) <= len(text); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a := text[i+j]
			b := sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
