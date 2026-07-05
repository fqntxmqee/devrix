package textutil

import "strings"

// ResolutionClaimsOpenTag / CloseTag bracket a structured per-ObsID claims
// block the LLM emits in its final message. The block's content is a JSON
// array of {obs_id, answer, confidence, supporting_evidence} objects.
//
// DM-20260704-006 S4 Phase 1.5 (D7-S16-A105-T01/T02): the Execute layer
// strips this block from user-visible text AND extracts the JSON so the
// Verify/Decide nodes have typed ResolutionClaim[] without parsing prose.
// The marker mirrors the <prior-output-summary> pattern but is the inverse
// direction: claim blocks ARE extracted (not discarded) so downstream
// observability can correlate claims with the LLM's user reply.
//
// Wire format note: the marker is intentionally a plain ASCII tag pair
// (not the structured pt:"…,…" kernel used by StrategicPlanFrame). The
// LLM emits free-form text + one trailing block; the kernel of pt-tags
// is reserved for the user-frame guide layer that drives Plan construction
// (see internal/shared/prompttags/linefield.go). Using a different wire
// shape here keeps the two contracts from colliding when both render in
// the same LLM response.
const (
	ResolutionClaimsOpenTag  = "<resolution_claims>"
	ResolutionClaimsCloseTag = "</resolution_claims>"
)

// ExtractResolutionClaimsJSON returns the JSON payload between the
// resolution_claims markers (trimmed) and the input text with the block
// stripped out. When the input has no balanced block the JSON is empty
// and the cleaned text equals the input — callers see "no claims filed"
// which is the canonical Phase 1.5 "LLM did not participate in RC-1"
// state (Plan had no strategies → caller must not even call this).
//
// Unbalanced markers (open without close) drop the open tag and everything
// after it, mirroring StripPriorOutputSummary's defensive behavior. Better
// to lose a tail fragment than render half a tag.
//
// The returned cleaned text is trimmed at the join points (no leading/
// trailing whitespace) so downstream IM renderers see clean prose.
func ExtractResolutionClaimsJSON(text string) (jsonPayload string, cleaned string) {
	lower := strings.ToLower(text)
	openIdx := strings.Index(lower, ResolutionClaimsOpenTag)
	if openIdx < 0 {
		return "", strings.TrimSpace(text)
	}
	rest := text[openIdx+len(ResolutionClaimsOpenTag):]
	closeIdx, closeLen := findCaseInsensitive(rest, ResolutionClaimsCloseTag)
	var prefix, suffix, payload string
	if closeIdx < 0 {
		// Unbalanced open — drop everything from the open tag onward.
		prefix = strings.TrimSpace(text[:openIdx])
		return "", prefix
	}
	prefix = strings.TrimSpace(text[:openIdx])
	payload = strings.TrimSpace(rest[:closeIdx])
	suffix = strings.TrimSpace(rest[closeIdx+closeLen:])
	switch {
	case prefix == "":
		cleaned = suffix
	case suffix == "":
		cleaned = prefix
	default:
		cleaned = prefix + "\n" + suffix
	}
	return payload, cleaned
}

// StripResolutionClaims removes all <resolution_claims> blocks from the
// input. Unlike ExtractResolutionClaimsJSON this is a pure strip — useful
// at the D1 IM boundary where the user-facing renderer does not need the
// claims payload, only clean prose.
func StripResolutionClaims(text string) string {
	_, cleaned := ExtractResolutionClaimsJSON(text)
	return cleaned
}
