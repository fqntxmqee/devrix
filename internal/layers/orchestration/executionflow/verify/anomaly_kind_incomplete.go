package verify

import (
	"context"
	"strconv"

	"github.com/devrix/devrix/internal/layers/orchestration/hardening"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

// DM-20260630-011 (devrix-session-conclusion-completeness) — DetectTaskIncomplete.
//
// Triggers when the LLM Observe node produced multiple high-strength
// system-level uncertainty signals, indicating the agent is uncertain
// whether the task has been completed correctly. Surfaced as
// AnomalyKindTaskIncomplete so the verifier can fail-closed the round.
//
// Threshold rationale (NOT a magic number; matches the same 0.7 floor
// used in orchtypes/uncertainty_report.go obsUncertaintyAnomalyThreshold
// and prior.adaptivePriorObsThreshold — all three share the constant
// by design so the system has one consistent "is this a real signal
// vs noise" floor):
//   - ≥ 2 observations ensures single false positives don't trip the rule
//   - avg strength ≥ 0.7 ensures signal is non-trivial
//
// Returns DetectionResult with the standard 4-attribute span payload.
func DetectTaskIncomplete(ctx context.Context, sessionID string, report orchtypes.UncertaintyReport) DetectionResult {
	obs := report.FilterByKind(orchtypes.ObsUncertainty)
	if len(obs) < 2 {
		return nonTriggered(ctx, AnomalyKindTaskIncomplete, sessionID, len(obs))
	}
	var sum float64
	sysCount := 0
	for _, o := range obs {
		if o.Category == orchtypes.CatSystem && o.Strength >= obsUncertaintyDetectorMinStrength {
			sum += o.Strength
			sysCount++
		}
	}
	if sysCount < 2 {
		return nonTriggered(ctx, AnomalyKindTaskIncomplete, sessionID, sysCount)
	}
	avg := sum / float64(sysCount)
	if avg < obsUncertaintyDetectorMinStrength {
		return nonTriggered(ctx, AnomalyKindTaskIncomplete, sessionID, sysCount)
	}

	kind := AnomalyKindTaskIncomplete
	severity := SeverityLow
	switch {
	case avg >= 0.95:
		severity = SeverityHigh
	case avg >= 0.85:
		severity = SeverityMedium
	}

	res := DetectionResult{
		Triggered:  true,
		Kind:       kind,
		Severity:   severity,
		Threshold:  sysCount,
		EvidenceID: buildEvidenceID(sessionID, len(report.Anomalies), sysCount),
	}
	_, end := hardening.EmitSystemAnomalyDetect(
		ctx, sessionID, string(kind), string(severity), strconv.Itoa(sysCount), res.EvidenceID,
	)
	end(nil)
	endTrigger := hardening.EmitAnomalyTrigger(
		ctx, sessionID, string(kind), string(severity), strconv.Itoa(sysCount), res.EvidenceID,
	)
	endTrigger(nil)
	return res
}

// DM-20260630-011 — DetectEmptyConclusion.
//
// Triggers when the LLM's last turn text contains template/repeat markers
// (scope_contract, directive_template, task_recap, planning) — strong
// signal that the agent produced a planning/recap artifact instead of
// actual review findings. Surfaced as AnomalyKindEmptyConclusion so the
// verifier fails-closed and the D1 IM adapter can render a clear
// "任务结论不完整" warning instead of the templated text.
//
// Unlike DetectTaskIncomplete which is LLM-signal-driven, this detector
// is heuristic-string-driven. It targets the exact sess_1782814140202_7000
// failure mode (553-char scope_contract recap leaked to feishu card).
func DetectEmptyConclusion(ctx context.Context, sessionID, lastTurnText string) DetectionResult {
	if lastTurnText == "" {
		return nonTriggered(ctx, AnomalyKindEmptyConclusion, sessionID, 0)
	}
	runeLen := len([]rune(lastTurnText))
	if runeLen > emptyConclusionMaxRunes {
		// Real review content > 800 runes almost always contains findings
		// (too verbose for a template); don't false-positive on long
		// legitimate reviews.
		return nonTriggered(ctx, AnomalyKindEmptyConclusion, sessionID, runeLen)
	}
	matched := 0
	for _, marker := range emptyConclusionMarkers {
		if containsMarker(lastTurnText, marker) {
			matched++
		}
	}
	if matched == 0 {
		return nonTriggered(ctx, AnomalyKindEmptyConclusion, sessionID, runeLen)
	}

	severity := SeverityMedium
	if runeLen < 600 {
		severity = SeverityHigh
	}
	res := DetectionResult{
		Triggered:  true,
		Kind:       AnomalyKindEmptyConclusion,
		Severity:   severity,
		Threshold:  matched,
		EvidenceID: buildEvidenceID(sessionID, matched, runeLen),
	}
	_, end := hardening.EmitSystemAnomalyDetect(
		ctx, sessionID, string(AnomalyKindEmptyConclusion), string(severity), strconv.Itoa(matched), res.EvidenceID,
	)
	end(nil)
	endTrigger := hardening.EmitAnomalyTrigger(
		ctx, sessionID, string(AnomalyKindEmptyConclusion), string(severity), strconv.Itoa(matched), res.EvidenceID,
	)
	endTrigger(nil)
	return res
}

// DM-20260630-011 — constants for the 2 new detectors.

const (
	// obsUncertaintyDetectorMinStrength mirrors the ObsUncertainty anomaly
	// threshold (0.7). Single source of truth across orchtypes + prior +
	// verify so signal/noise boundary is consistent.
	obsUncertaintyDetectorMinStrength = 0.7

	// emptyConclusionMaxRunes — last-turn texts above this length almost
	// always contain real findings; cap avoids false positives on long
	// legitimate reviews. Chosen at 800 runes ≈ 400-500 Chinese chars /
	// 1.5x a scope-contract template.
	emptyConclusionMaxRunes = 800
)

// emptyConclusionMarkers — DM-20260630-011 AC5. The 4 fallback template
// markers from the LLM that indicate "this is not a real review".
// Picked from the actual session_1782814140202_7000 LLM output plus
// common planning/recap tag patterns observed in earlier traces.
var emptyConclusionMarkers = []string{
	"<scope_contract>",
	"<directive_template>",
	"<task_recap>",
	"<planning>",
}

// containsMarker is a case-insensitive substring check. Avoids pulling
// in strings.ToLower on the hot path by using a simple loop.
func containsMarker(text, marker string) bool {
	if len(marker) == 0 || len(text) < len(marker) {
		return false
	}
	for i := 0; i+len(marker) <= len(text); i++ {
		match := true
		for j := 0; j < len(marker); j++ {
			a := text[i+j]
			b := marker[j]
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

// nonTriggered builds a DetectionResult with Triggered=false and emits
// the standard observability span so dashboards see "this detector ran
// and decided NO". Used by the early-return paths to keep span coverage
// uniform (per DM-20260629-001 PR-6 T40 convention).
func nonTriggered(ctx context.Context, kind AnomalyKind, sessionID string, threshold int) DetectionResult {
	res := DetectionResult{
		Triggered:  false,
		Kind:       kind,
		Severity:   SeverityNone,
		Threshold:  threshold,
		EvidenceID: buildEvidenceID(sessionID, threshold, 0),
	}
	_, end := hardening.EmitSystemAnomalyDetect(
		ctx, sessionID, string(kind), string(SeverityNone), strconv.Itoa(threshold), res.EvidenceID,
	)
	end(nil)
	return res
}