package sessionorchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/shared/textutil"
)

// AppendResolutionClaimHint appends the RC-1 contract guide to the LLM
// directive when the Plan emitted one or more ResolutionStrategies. The
// guide lists each ObsID the LLM is expected to answer and the wire format
// it should emit (a <resolution_claims> JSON block) so Execute can fill
// ResolutionClaim[] without ad-hoc prose parsing.
//
// DM-20260704-006 S4 Phase 1.5 (D7-S16-A105-T01/T02). The hint is appended
// (not prepended) because the LLM already received the user directive +
// deliverable tags above; placing the hint at the tail keeps the schema
// tags near the top of the message where the model attends first.
//
// Empty strategies → empty hint: callers in Phase 1 (Plan did not file
// RC-1) get zero contamination of their directive. By the time Phase 3's
// Decide routing lands, the legacy verdict-based path will still see the
// plain directive.
func AppendResolutionClaimHint(directive string, strategies []interfaces.ResolutionStrategy) string {
	if len(strategies) == 0 {
		return directive
	}
	seen := make(map[string]struct{}, len(strategies))
	var ids []string
	for _, s := range strategies {
		id := strings.TrimSpace(s.ObsID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return directive
	}
	var b strings.Builder
	b.WriteString("\n\nResolutionClaim contract (RC-1, devrix-d7-uncertainty-resolution-traceability):\n")
	b.WriteString("For each observation_id below, append a single JSON object with fields\n")
	b.WriteString(`  {"obs_id":"<id>","answer":"<1-2 sentences>","confidence":<0..1>,"supporting_evidence":"<file:line or quote>"}\n`)
	b.WriteString("to a <resolution_claims>...</resolution_claims> block at the END of your final reply.\n")
	b.WriteString("Emit one object per obs_id (skip ones you cannot answer — the gap itself is signal).\n")
	b.WriteString("ObsIDs: ")
	b.WriteString(strings.Join(ids, ", "))
	return strings.TrimSpace(directive) + b.String()
}

// ParseResolutionClaims extracts a typed []interfaces.ResolutionClaim from
// the LLM's final message by reading the <resolution_claims> JSON block
// AND stripping it from the displayed text. Returns nil claims + cleaned
// text when the block is absent or malformed (so callers degrade to the
// Phase 1.5 safety-net behavior: no claims → every strategy reads
// "no_resolution_claim" in the Verify report).
//
// The malformed-payload path is intentionally non-fatal: a JSON parse
// error means the LLM tried but failed, which the Phase 3 Decide hook
// then surfaces as UnresolvedObs[Reason=no_resolution_claim]. Failing
// the entire round for a malformed claim would punish the LLM for what
// Phase 1 was designed to be a soft contract.
//
// Contract used by item_pipeline.go to:
//   1. Set result.ResolutionClaims (typed) so the worktree round carries them
//   2. Pass cleaned text to result.Content (user-visible summary)
func ParseResolutionClaims(content string) (claims []interfaces.ResolutionClaim, cleaned string) {
	payload, stripped := textutil.ExtractResolutionClaimsJSON(content)
	cleaned = stripped
	if strings.TrimSpace(payload) == "" {
		return nil, cleaned
	}
	if err := json.Unmarshal([]byte(payload), &claims); err != nil {
		// Leave the original content alone (cleaned is the stripped form,
		// which still contains user-visible prose; we DO NOT echo the
		// failed JSON back to the user — that leaks wire-format details).
		// Return the stripped text only.
		return nil, cleaned
	}
	// Defensive: drop entries whose ObsID is empty so Verify sees a
	// homogeneous slice. The validate-on-construct path in NewResolutionReport
	// would reject them anyway, but we surface a cleaner report.
	out := claims[:0]
	for _, c := range claims {
		if strings.TrimSpace(c.ObsID) == "" {
			continue
		}
		out = append(out, c)
	}
	return out, cleaned
}

// ResolutionClaimsFingerprint produces a short stable key for a claims
// slice. Used by ItemPipelineRunner only when logging the resolved round
// so dashboards can group "answered N observations" without parsing JSON.
//
// Returns "" for an empty slice so log lines can skip the field entirely.
func ResolutionClaimsFingerprint(claims []interfaces.ResolutionClaim) string {
	if len(claims) == 0 {
		return ""
	}
	ids := make([]string, 0, len(claims))
	for _, c := range claims {
		ids = append(ids, c.ObsID)
	}
	return fmt.Sprintf("obs=%d[%s]", len(claims), strings.Join(ids, ","))
}
