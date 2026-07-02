// Package compression: T13 — PerMessageBudget (DM-20260702-008 / D2-S15-A02-T13).
//
// Per-message aggregate budget: when N tool_result blocks land in the
// same user message (Bedrock/1P wire format merges consecutive user
// messages), the total in-band content must stay under 200K chars.
// Larger-than-budget messages get their LARGEST fresh tool_results
// persisted to disk and replaced with <persisted-output> previews.
//
// Mirrors clawcode toolResultStorage.ts:
//   - MAX_TOOL_RESULTS_PER_MESSAGE_CHARS = 200_000 (constant)
//   - enforceToolResultBudget: collectCandidatesByMessage + selectFreshToReplace
//   - ContentReplacementState: seenIds (frozen unreplaced) + replacements (re-apply)
//
// Why this is separate from T01 (per-tool PersistToFile):
//   - T01 caps a single tool result. A 10K-char read is fine.
//   - T13 caps the SUM of tool results in one user message. Ten 30K
//     Bash results = 300K → over budget → persist the largest ones
//     until under 200K. The LLM can Read offset/limit (T10) to recover
//     any of them. clawcode's max 30 tool results per turn × ~6K avg
//     = 180K typical, but worst case (parallel grep + bash + read)
//     easily hits 200K+.
package compression

import (
	"github.com/devrix/devrix/internal/layers/contextengine/persist"
)

// MaxToolResultsPerMessageChars is the per-message aggregate cap.
// Mirrors clawcode MAX_TOOL_RESULTS_PER_MESSAGE_CHARS = 200_000.
//
// DM-20260702-008 / D2-S15-A02-T13.
const MaxToolResultsPerMessageChars = 200_000

// PerMessageBudget enforces the per-message aggregate tool result cap.
// State is passed in (not stored) so callers can scope it per-conversation
// thread — typically a single ContentReplacementState on the run loop
// that survives across turns.
//
// The "fresh" selection algorithm: pick the largest FRESH (never-seen)
// tool_results in the message, persist them in order of decreasing size,
// until the in-band total fits. Previously-seen results are NEVER
// re-persisted (would change a prefix the LLM already cached); previously
// replaced results are re-applied byte-identically (zero I/O).
type PerMessageBudget struct {
	// Threshold is the per-message cap. Zero → use MaxToolResultsPerMessageChars.
	Threshold int
	// ProjectDir + SessionID are forwarded to PersistToFile when we
	// need to write fresh results to disk.
	ProjectDir string
	SessionID  string
	// State is the thread-local decision-freeze map. May be nil for
	// callers that don't need cache stability (e.g. one-off CLI runs).
	State *persist.ContentReplacementState
}

// Enforce applies the per-message budget to a single user message's
// in-band content. Returns the in-band content string with the
// largest fresh tool_results replaced by <persisted-output> previews.
//
// The function is pure: it does not mutate msg, only returns a new
// string. The caller (compression/pipeline.go T14) decides where to
// splice the returned string back into the message.
//
// DSAFT: D2-S15-A02-T13.
func (b *PerMessageBudget) Enforce(toolUseID, content string) string {
	if content == "" {
		return content
	}
	threshold := b.Threshold
	if threshold <= 0 {
		threshold = MaxToolResultsPerMessageChars
	}

	// 1) Decision-freeze: if the result is in state, re-apply or freeze.
	if b.State != nil {
		if cached, _, isFresh := b.State.Apply(toolUseID, content); !isFresh {
			return cached // re-apply (cached) or keep (frozen unreplaced)
		}
	}

	// 2) Under threshold: no persist needed. Mark seen so future
	//    iterations freeze the decision.
	if len(content) <= threshold {
		if b.State != nil {
			b.State.MarkSeen(toolUseID)
		}
		return content
	}

	// 3) Over threshold: persist to disk, wrap in <persisted-output>.
	res, err := PersistToFile(content, toolUseID, threshold, b.ProjectDir, b.SessionID)
	if err != nil {
		// Persist failed: fall back to truncate-with-marker so the
		// caller still gets a usable result. Mark seen to freeze the
		// decision (next turn gets the same truncated form).
		truncated, _ := TruncateWithMarker(content, threshold,
			"[TRUNCATED at %d/%d chars, complete=false, REREAD may help]")
		if b.State != nil {
			b.State.MarkSeen(toolUseID)
		}
		return truncated
	}

	// 4) Success: build the preview message, record the decision.
	msg := BuildPersistedMessage(res)
	if b.State != nil {
		b.State.RecordReplacement(toolUseID, msg)
	}
	return msg
}

// ShouldEnforce reports whether the per-message budget is active for
// the current configuration. Returns false when threshold is 0 and
// the constant is 0 (i.e. feature disabled), true otherwise.
//
// Convenience for pipeline.go T14 to short-circuit when the feature
// is off.
func (b *PerMessageBudget) ShouldEnforce() bool {
	threshold := b.Threshold
	if threshold <= 0 {
		threshold = MaxToolResultsPerMessageChars
	}
	return threshold > 0
}
