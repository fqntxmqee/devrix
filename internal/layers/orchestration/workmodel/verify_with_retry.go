package workmodel

import (
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// VerifierOutput captures a single Verifier sub-agent raw output together
// with the parsed VerdictKind + Confidence + RetryCount metadata. Used by
// EvidenceExtractor (Phase 4 PR-D3) and by the orchestrator-level retry
// loop (Phase 4 PR-D2 G8-1 fix).
//
// RetryCount records how many parse attempts were needed (0 = first try
// succeeded). Callers can use this metric to flag "verifier flaky" in D5
// dashboards.
type VerifierOutput struct {
	Raw         string            `json:"raw"`
	ParsedKind  types.VerdictKind `json:"parsed_kind"`
	Confidence  float64           `json:"confidence,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	RetryCount  int               `json:"retry_count,omitempty"`
}

// ParseVerifierOutput parses a single Verifier LLM output (JSON format
// with kind / confidence / reason fields). Returns an error on parse
// failure (malformed JSON, unknown VerdictKind, etc).
//
// This is the single-attempt parser; for retry semantics see
// ParseVerifierOutputWithRetry.
func ParseVerifierOutput(raw string) (VerifierOutput, error) {
	var parsed struct {
		Kind       string  `json:"kind"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return VerifierOutput{Raw: raw}, fmt.Errorf("verify: malformed verifier output: %w", err)
	}
	kind, err := types.ParseVerdictKind(parsed.Kind)
	if err != nil {
		return VerifierOutput{Raw: raw}, fmt.Errorf("verify: %w", err)
	}
	return VerifierOutput{
		Raw:        raw,
		ParsedKind: kind,
		Confidence: parsed.Confidence,
		Reason:     parsed.Reason,
	}, nil
}

// DefaultMaxParseRetries is the default cap for ParseVerifierOutputWithRetry.
// doc 17 §4.3 specifies 3 attempts as the safety net. Phase 4 PR-D2's G8-1
// P0-3 fix mandates that after maxRetries parse failures the function MUST
// return VerdictIndeterminate (NOT error and NOT VerdictFail) so the
// orchestrator surfaces a high-uncertainty result for human review rather
// than mis-classifying as a definitive FAIL.
const DefaultMaxParseRetries = 3

// ParseVerifierOutputWithRetry retries parse up to maxRetries times.
// After maxRetries failures, returns VerifierOutput with
// ParsedKind=VerdictIndeterminate (NOT error and NOT VerdictFail).
//
// G8-1 P0-3 fix rationale: prior to PR-D2, parse failure propagated as
// a hard error and the orchestrator surface the verifier as FAIL. This
// conflated two distinct signals:
//
//	(a) "verifier could not produce parseable output" (parser problem)
//	(b) "verifier produced parseable output saying FAIL" (verdict problem)
//
// Distinguishing (a) from (b) requires the retry+INDETERMINATE fallback.
// INDETERMINATE maps to ExitReasonVerifierAbstain which surfaces for
// human review (Phase 4 PR-D2 VerdictToExitReason).
//
// On success, RetryCount records the attempt index (0 = first try). On
// failure, RetryCount=maxRetries to signal exhaustion.
func ParseVerifierOutputWithRetry(raw string, maxRetries int) VerifierOutput {
	if maxRetries <= 0 {
		maxRetries = DefaultMaxParseRetries
	}
	for i := 0; i < maxRetries; i++ {
		out, err := ParseVerifierOutput(raw)
		if err == nil {
			out.RetryCount = i
			return out
		}
	}
	// maxRetries parse failures → INDETERMINATE (NOT error).
	return VerifierOutput{
		Raw:        raw,
		ParsedKind: types.VerdictIndeterminate,
		Confidence: 0,
		RetryCount: maxRetries,
	}
}