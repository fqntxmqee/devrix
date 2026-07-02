package compression

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// toolResultBudget persists oversized tool-result messages to disk and
// replaces their in-band content with a <persisted-output> XML reference
// pointing at the saved file. Returns a fresh slice (does not mutate
// the input).
//
// DM-20260702-008 devrix-token-design-v2 / D2-S15-A02-T02: replaces the
// 8K TruncateToTokens self-loop with on-disk persistence mirroring
// clawcode's processPreMappedToolResultBlock. The LLM can recover the
// full payload by Reading the saved file via offset/limit (T10).
//
// Fall-back contract: if PersistToFile returns an error, the function
// falls back to TruncateWithMarker so the task is NEVER abandoned by
// the budget pass — same fail-closed semantics as the previous design.
func toolResultBudget(
	counter tokenCounter,
	msgs []types.Message,
	maxPerResult int,
	projectDir string,
	sessionID string,
) []types.Message {
	const charsPerToken = 4 // tiktoken approximation, matches clawcode
	maxChars := maxPerResult * charsPerToken
	out := make([]types.Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if m.Role != types.MessageRoleTool {
			continue
		}
		// Token-count gate mirrors the prior design so we don't
		// pay I/O for already-small results.
		if counter.CountText(m.Content) <= maxPerResult {
			continue
		}
		out[i].Content = persistOrTruncate(
			m.Content, m.ID, maxChars, projectDir, sessionID, maxPerResult,
		)
	}
	return out
}

// persistOrTruncate runs PersistToFile and, on success, wraps the preview
// in <persisted-output> XML. On any error it falls back to
// TruncateWithMarker so the budget pass never drops content entirely.
//
// DSAFT: D2-S15-A02-T02 (fall-back to TruncateWithMarker keeps the
// 8K-self-loop fix signal visible even when persistence I/O fails).
func persistOrTruncate(
	content, toolUseID string,
	maxChars int,
	projectDir, sessionID string,
	maxTokensForMarker int,
) string {
	// Pull toolUseID from message metadata if not set on the ID field.
	effectiveID := toolUseID
	if effectiveID == "" {
		// devrix messages may carry the tool_use_id in Metadata; this
		// keeps the persist path working even when the upstream
		// adapter hasn't yet plumbed a dedicated field.
		if v, ok := lookupToolUseID(content); ok {
			effectiveID = v
		}
	}
	res, err := PersistToFile(content, effectiveID, maxChars, projectDir, sessionID)
	if err == nil {
		if res.FilePath == "" {
			// Under threshold or image block — return as-is.
			return res.Preview
		}
		return BuildPersistedMessage(res)
	}
	// Fall back to TruncateWithMarker so the LLM still sees the
	// complete=false signal (preserves the 8K self-loop治本 contract
	// when the disk write fails — e.g. read-only FS, ENOSPC).
	marker := "[TRUNCATED at %d/%d chars, complete=false, REREAD may help]"
	truncated, _ := TruncateWithMarker(content, maxTokensForMarker, marker)
	return truncated
}

// lookupToolUseID is a defensive helper: some devrix adapters stash the
// tool_use_id at the top of the content as "tool_use_id: <id>\n".
// We only treat it as authoritative when it's a single short token
// (alphanumeric + dashes) — anything else is treated as untrusted
// content and we let PersistToFile derive a path from the message ID.
func lookupToolUseID(content string) (string, bool) {
	const prefix = "tool_use_id: "
	if len(content) < len(prefix) {
		return "", false
	}
	if content[:len(prefix)] != prefix {
		return "", false
	}
	rest := content[len(prefix):]
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '\n' {
			id := rest[:i]
			if isSafeToolID(id) {
				return id, true
			}
			return "", false
		}
		if !isSafeToolIDChar(c) {
			return "", false
		}
	}
	return "", false
}

func isSafeToolIDChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_'
}

func isSafeToolID(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isSafeToolIDChar(s[i]) {
			return false
		}
	}
	return true
}

// snip drops the oldest messages (FIFO) until total tokens ≤ target or only
// minKeep messages remain. Used as a fallback before autocompact.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go.
func snip(counter tokenCounter, msgs []types.Message, target, minKeep int) []types.Message {
	if minKeep <= 0 {
		minKeep = 4
	}
	if len(msgs) <= minKeep {
		return msgs
	}
	out := append([]types.Message(nil), msgs...)
	for counter.CountMessages(out) > target && len(out) > minKeep {
		out = out[1:]
	}
	return out
}

// microcompact merges consecutive same-role messages (common when streaming
// tool results land in adjacent turns). Returns (msgs, applied).
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go.
func microcompact(msgs []types.Message, _ types.TokenBudget) ([]types.Message, bool) {
	if len(msgs) < 2 {
		return msgs, false
	}
	var out []types.Message
	changed := false
	for i := 0; i < len(msgs); i++ {
		if i+1 < len(msgs) && msgs[i].Role == msgs[i+1].Role {
			merged := msgs[i]
			merged.Content = msgs[i].Content + "\n---\n" + msgs[i+1].Content
			for i+2 < len(msgs) && msgs[i+2].Role == merged.Role {
				i++
				merged.Content += "\n---\n" + msgs[i].Content
			}
			out = append(out, merged)
			changed = true
			i++
			continue
		}
		out = append(out, msgs[i])
	}
	return out, changed
}

// collapse folds runs of 2+ short messages (content < minLen chars) into a
// single placeholder, preserving the head and tail of the run. Reduces visual
// noise from rapid-fire status messages. Returns (msgs, applied).
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go.
func collapse(msgs []types.Message, _ types.TokenBudget) ([]types.Message, bool) {
	const minLen = 20
	if len(msgs) < 3 {
		return msgs, false
	}
	var out []types.Message
	changed := false
	for i := 0; i < len(msgs); i++ {
		runStart := i
		for i+1 < len(msgs) && len(msgs[i].Content) < minLen && len(msgs[i+1].Content) < minLen {
			i++
		}
		if i-runStart >= 2 {
			out = append(out, msgs[runStart])
			folded := types.Message{
				ID:        msgs[runStart].ID + "_fold",
				SessionID: msgs[runStart].SessionID,
				Role:      msgs[runStart].Role,
				Content:   fmt.Sprintf("[折叠 %d 条消息]", i-runStart),
				Timestamp: msgs[i].Timestamp,
			}
			out = append(out, folded)
			out = append(out, msgs[i])
			changed = true
			continue
		}
		for j := runStart; j <= i; j++ {
			out = append(out, msgs[j])
		}
	}
	return out, changed
}

// assemble prepends the system prompt to the message list as a system-role
// message. If systemPrompt is empty, the input slice is returned unchanged.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-2: extracted from pipeline.go.
func assemble(systemPrompt string, msgs []types.Message) []types.Message {
	if systemPrompt == "" {
		return msgs
	}
	sys := types.Message{
		ID:      "system_prompt",
		Role:    types.MessageRoleSystem,
		Content: systemPrompt,
	}
	return append([]types.Message{sys}, msgs...)
}
