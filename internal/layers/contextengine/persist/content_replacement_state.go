// Package persist: T04 — ContentReplacementState (DM-20260702-008 / D2-S15-A02-T04).
//
// Decision-freeze: once a toolUseID has been observed, the persist
// pipeline MUST make the same persist-or-not choice on every subsequent
// turn. This is the per-thread state that preserves prompt cache
// stability across microcompact replays — the LLM sees byte-identical
// in-band content turn after turn, even if the persist path is taken
// on turn N and not on turn N+1 (or vice versa).
//
// Mirrors clawcode toolResultStorage.ts:386-413 (ContentReplacementState +
// provisionContentReplacementState + reconstructContentReplacementState).
package persist

// ContentReplacementState tracks per-conversation-thread decisions about
// whether a given tool result was persisted and what preview string
// replaced it. State is stable so the persist pipeline makes the same
// choice on every turn:
//
//   - SeenIds:     IDs that have passed through the persist gate (replaced
//     or not). Once seen, a result's fate is frozen for the thread.
//   - Replacements: subset of SeenIds that were persisted, mapped to the
//     exact preview string shown to the LLM. Re-application is a map
//     lookup — no file I/O, byte-identical, cannot fail.
//
// Concurrency: methods are safe for concurrent use as long as callers do
// not race MarkSeen with Lookup on the same key. The typical pattern is
// "mark seen THEN set replacement" which is idempotent because once a
// replacement is set it never changes.
type ContentReplacementState struct {
	// SeenIds is the set of toolUseIDs that have been observed by the
	// persist gate. A result in SeenIds but not in Replacements means
	// "we saw it, it was small enough not to persist, never persist it
	// later" (would change a prefix the LLM already cached).
	SeenIds map[string]struct{}

	// Replacements maps toolUseID -> the exact preview string the LLM
	// saw. Re-application is a map lookup: zero I/O, byte-identical,
	// cannot fail. The map value MUST be the final in-band content
	// (i.e. the <persisted-output> XML wrapper), not just the preview
	// head — microcompact re-apply must produce the wire-identical block.
	Replacements map[string]string
}

// NewContentReplacementState returns an empty state. Use this for a
// fresh conversation thread (no resume).
func NewContentReplacementState() *ContentReplacementState {
	return &ContentReplacementState{
		SeenIds:      make(map[string]struct{}),
		Replacements: make(map[string]string),
	}
}

// NewContentReplacementStateFrom reconstructs a state from a list of
// records loaded from the transcript. Used on resume so the persist
// pipeline makes the same choices it made in the original session.
//
// Records whose IDs are not in candidateIDs are skipped — they're inert
// (e.g. IDs that were cleared by a later /clear or autocompact).
func NewContentReplacementStateFrom(
	candidateIDs []string,
	records []ContentReplacementRecord,
) *ContentReplacementState {
	s := NewContentReplacementState()
	candidateSet := make(map[string]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		candidateSet[id] = struct{}{}
		s.SeenIds[id] = struct{}{}
	}
	for _, r := range records {
		if r.Kind != "tool-result" {
			continue
		}
		if _, ok := candidateSet[r.ToolUseID]; !ok {
			continue
		}
		s.Replacements[r.ToolUseID] = r.Replacement
	}
	return s
}

// MarkSeen records that the given toolUseID has passed through the
// persist gate. Idempotent: marking an ID that's already seen is a no-op.
//
// Callers should call MarkSeen BEFORE deciding to persist. The atomic
// invariant the persist pipeline maintains is:
//
//	ID in SeenIds && ID not in Replacements → never persist this ID
//	ID in SeenIds && ID in Replacements      → always re-apply the same replacement
//	ID not in SeenIds                        → eligible for new decisions
func (s *ContentReplacementState) MarkSeen(toolUseID string) {
	if s == nil {
		return
	}
	s.SeenIds[toolUseID] = struct{}{}
}

// Lookup returns the cached replacement for toolUseID and whether one
// exists. Callers that get (true, _) MUST re-apply the cached value
// verbatim — any modification would break prompt cache.
func (s *ContentReplacementState) Lookup(toolUseID string) (string, bool) {
	if s == nil {
		return "", false
	}
	v, ok := s.Replacements[toolUseID]
	return v, ok
}

// RecordReplacement stores the decision that toolUseID was persisted
// and the LLM-visible replacement string. After this call, Lookup
// returns (replacement, true) for the same toolUseID, and any future
// Apply() call will re-apply the cached value byte-identically.
func (s *ContentReplacementState) RecordReplacement(toolUseID, replacement string) {
	if s == nil {
		return
	}
	s.SeenIds[toolUseID] = struct{}{}
	s.Replacements[toolUseID] = replacement
}

// IsSeen reports whether toolUseID has previously passed through the
// persist gate. A result that is seen but not in Replacements is
// "frozen unreplaced" — must never be replaced later.
func (s *ContentReplacementState) IsSeen(toolUseID string) bool {
	if s == nil {
		return false
	}
	_, ok := s.SeenIds[toolUseID]
	return ok
}

// Size returns the number of decisions tracked. Useful for /status
// metrics and assertions in tests.
func (s *ContentReplacementState) Size() int {
	if s == nil {
		return 0
	}
	return len(s.SeenIds)
}

// Apply returns the in-band LLM content for a tool result, honoring
// the decision-freeze invariant:
//
//   - Lookup hit      → re-apply the cached replacement (zero I/O)
//   - IsSeen but not Lookup → return content unchanged (frozen unreplaced)
//   - Never seen      → return ("", false) so the caller runs the
//     persist decision and stores the outcome via RecordReplacement.
//
// The third return value distinguishes "frozen unreplaced" (must keep
// content as-is) from "never seen" (caller decides).
func (s *ContentReplacementState) Apply(toolUseID, content string) (out string, isFrozen, isFresh bool) {
	if s == nil {
		return content, false, true
	}
	if cached, ok := s.Lookup(toolUseID); ok {
		return cached, true, false
	}
	if s.IsSeen(toolUseID) {
		return content, true, false // frozen unreplaced — keep verbatim
	}
	return "", false, true // fresh — caller decides
}

// ContentReplacementRecord is the serialized form of one decision,
// written to the transcript so resume can reconstruct state.
//
// Mirrors clawcode ContentReplacementRecord: { kind, toolUseId, replacement }.
// `replacement` is the exact string the model saw — stored rather than
// derived on resume so code changes to the preview template, size
// formatting, or path layout can't silently break prompt cache.
type ContentReplacementRecord struct {
	Kind        string `json:"kind"` // always "tool-result" for now
	ToolUseID   string `json:"toolUseId"`
	Replacement string `json:"replacement"`
}
