// Package compression: T01 — PersistToFile (DM-20260702-008 / D2-S15-A02-T01).
//
// Replaces the 8K TruncateToTokens self-loop with on-disk persistence +
// <persisted-output> XML reference, mirroring clawcode's
// src/utils/toolResultStorage.ts:persistToolResult + buildLargeToolResultMessage.
// Information is NEVER physically lost: the LLM can Read the saved file
// to recover the full payload via the offset/limit path (T10).
package compression

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/persist"
)

// PersistedOutputTag wraps the preview message in an XML tag that
// downstream code (and the LLM itself) can detect to distinguish a
// persisted-result reference from full inline content.
//
// Mirrors clawcode PERSISTED_OUTPUT_TAG.
const PersistedOutputTag = "<persisted-output>"

// PersistedOutputClosingTag is the matching closing tag.
const PersistedOutputClosingTag = "</persisted-output>"

// PreviewSizeBytes is the byte cap for the inline preview kept in-band
// after persistence. Mirrors clawcode PREVIEW_SIZE_BYTES = 2000.
const PreviewSizeBytes = 2000

// toolResultsSubdir is the on-disk subdirectory under the session dir.
// Mirrors clawcode TOOL_RESULTS_SUBDIR.
const toolResultsSubdir = "tool-results"

// PersistedToolResult describes the artifact of a successful PersistToFile.
type PersistedToolResult struct {
	// FilePath is the on-disk location of the full content.
	FilePath string
	// OriginalSize is the byte length of the full content (pre-persistence).
	OriginalSize int
	// Preview is the head of the content, truncated at a newline boundary
	// when possible and capped at PreviewSizeBytes.
	Preview string
	// HasMore reports whether the preview was truncated.
	HasMore bool
}

// PersistToFileError reports why persistence failed. Callers fall back
// to truncate-with-marker on this case so the task is NEVER abandoned.
type PersistToFileError struct {
	Reason string
	Cause  error
}

func (e *PersistToFileError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("persist: %s: %v", e.Reason, e.Cause)
	}
	return "persist: " + e.Reason
}

// PersistToFile persists content to <projectDir>/<sessionID>/tool-results/<toolUseID>.txt
// when content exceeds maxChars, returning a preview that callers wrap in
// <persisted-output>...</persisted-output> for the in-band LLM message.
//
// Behavior:
//   - content <= maxChars: returns content unchanged (no error, no file written).
//   - content > maxChars: writes full content to disk (atomic via O_EXCL),
//     returns preview head + file path.
//   - hasImageBlock(content) == true: returns content unchanged (image bytes
//     must reach the model as-is, mirroring clawcode maybePersistLargeToolResult).
//   - on any I/O error: returns *PersistToFileError; caller falls back to
//     TruncateWithMarker so the task is NEVER abandoned.
//
// DSAFT: D2-S15-A02-T01 + D2-S15-A02-T03 (image skip).
func PersistToFile(
	content string,
	toolUseID string,
	maxChars int,
	projectDir string,
	sessionID string,
) (PersistedToolResult, error) {
	if hasImageBlock(content) {
		// Image blocks must reach the model intact. Mirrors clawcode
		// maybePersistLargeToolResult's hasImageBlock short-circuit.
		return PersistedToolResult{Preview: content, OriginalSize: len(content)}, nil
	}
	if maxChars <= 0 || utf8.RuneCountInString(content) <= maxChars {
		return PersistedToolResult{Preview: content, OriginalSize: len(content)}, nil
	}
	if strings.TrimSpace(projectDir) == "" {
		return PersistedToolResult{}, &PersistToFileError{Reason: "empty projectDir"}
	}
	if strings.TrimSpace(sessionID) == "" {
		return PersistedToolResult{}, &PersistToFileError{Reason: "empty sessionID"}
	}
	if strings.TrimSpace(toolUseID) == "" {
		return PersistedToolResult{}, &PersistToFileError{Reason: "empty toolUseID"}
	}

	// Path: <projectDir>/<sessionID>/tool-results/<toolUseID>.txt
	// O_EXCL makes tool_use_id collisions idempotent — same id seen twice
	// means the file is already there from a prior turn (microcompact replay)
	// and we just return the preview without rewriting.
	dir := filepath.Join(projectDir, sanitizeSegment(sessionID), toolResultsSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return PersistedToolResult{}, &PersistToFileError{
			Reason: "mkdir " + dir, Cause: err,
		}
	}
	fpath := filepath.Join(dir, sanitizeSegment(toolUseID)+".txt")

	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			// Replay-safe: file already on disk from a prior turn.
			preview, hasMore := generatePreview(content, PreviewSizeBytes)
			return PersistedToolResult{
				FilePath:     fpath,
				OriginalSize: len(content),
				Preview:      preview,
				HasMore:      hasMore,
			}, nil
		}
		return PersistedToolResult{}, &PersistToFileError{
			Reason: "open " + fpath, Cause: err,
		}
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		// Best-effort cleanup so we don't leave a half-written file that
		// would be returned by a later O_EXCL OpenFile.
		_ = os.Remove(fpath)
		return PersistedToolResult{}, &PersistToFileError{
			Reason: "write " + fpath, Cause: err,
		}
	}
	if err := f.Close(); err != nil {
		return PersistedToolResult{}, &PersistToFileError{
			Reason: "close " + fpath, Cause: err,
		}
	}

	preview, hasMore := generatePreview(content, PreviewSizeBytes)
	return PersistedToolResult{
		FilePath:     fpath,
		OriginalSize: len(content),
		Preview:      preview,
		HasMore:      hasMore,
	}, nil
}

// BuildPersistedMessage renders the in-band LLM message wrapping the
// preview in the clawcode-style <persisted-output> XML. Caller passes
// the PersistedToolResult returned by PersistToFile.
//
// Mirrors clawcode buildLargeToolResultMessage:189-198.
func BuildPersistedMessage(r PersistedToolResult) string {
	var b strings.Builder
	b.WriteString(PersistedOutputTag)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "Output too large (%s). Full output saved to: %s\n\n",
		formatFileSize(r.OriginalSize), r.FilePath)
	b.WriteString("Preview (first ")
	b.WriteString(formatFileSize(PreviewSizeBytes))
	b.WriteString("):\n")
	b.WriteString(r.Preview)
	if r.HasMore {
		b.WriteString("\n...")
	}
	b.WriteByte('\n')
	b.WriteString(PersistedOutputClosingTag)
	return b.String()
}

// generatePreview returns the first maxBytes bytes of content, cut at
// a newline boundary when the cut lands at >= 50% of maxBytes. Mirrors
// clawcode generatePreview:340-360 — avoid cutting mid-line for readability.
func generatePreview(content string, maxBytes int) (string, bool) {
	if len(content) <= maxBytes {
		return content, false
	}
	truncated := content[:maxBytes]
	lastNL := strings.LastIndex(truncated, "\n")
	cutPoint := maxBytes
	if lastNL > maxBytes/2 {
		cutPoint = lastNL
	}
	return content[:cutPoint], true
}

// hasImageBlock detects image-bearing content so we skip persistence —
// image bytes must reach the model as-is (they're multimodal payload,
// not text the LLM can choose to Read a file for).
//
// In devrix, image attachments are surfaced via Message.Attachments with
// Type == AttachmentTypeImage. We use a heuristic on Content too
// (data:image URI prefix, or our adapter's "[image attached: ...]" marker)
// for the case where the encoder serialized the image inline. Mirrors
// clawcode hasImageBlock:300-310.
func hasImageBlock(content string) bool {
	if strings.HasPrefix(content, "data:image/") {
		return true
	}
	// Defensive: devrix adapters sometimes inline a short marker
	// (e.g. "[image attached: foo.png]") that signals the LLM should
	// treat the content as opaque image payload.
	return strings.HasPrefix(content, "[image attached:")
}

// sanitizeSegment strips path separators and dot-dots so a hostile
// toolUseID cannot escape <projectDir>/<sessionID>/. UUIDs are safe,
// but we don't assume the caller validated the input.
func sanitizeSegment(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "_"
	}
	return s
}

// formatFileSize renders a human-readable byte count (e.g. "1.2 KB").
// Mirrors clawcode formatFileSize for log/UX consistency.
func formatFileSize(n int) string {
	const (
		kb = 1024
		mb = 1024 * 1024
	)
	switch {
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// _ keeps the imported persist package referenced for downstream use
// (T05 will use the growthbook override against this same store family).
var _ = persist.DefaultRoot
