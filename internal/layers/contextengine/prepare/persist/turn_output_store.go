// Package persist — turn_output_store.go implements DM-20260620-001 / AC2:
//
// Oversized assistant outputs (long LLM text dumps / tool synthesis
// responses) are folded head/tail style: the first 800 + last 200 chars
// are kept in-band, the middle is replaced with a "[middle N chars
// truncated; see {path}]" marker, and the full content is persisted to
// disk under turn-outputs/.
//
// Reuses the same ToolResultStore for the on-disk layout so the
// implementation shares GC / sanitisation logic.
//
// DSAFT: D2-S17-A06 extension.
package persist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devrix/devrix/internal/shared/textutil"
)

// DefaultAssistantHead is the number of leading runes preserved when an
// assistant output is folded.
const DefaultAssistantHead = 800

// DefaultAssistantTail is the number of trailing runes preserved when an
// assistant output is folded.
const DefaultAssistantTail = 200

// DefaultMaxAssistantChars is the soft cap above which an assistant
// message is folded. 8K characters roughly maps to ~2K tokens which is
// well under the per-message allocation typical LLM providers reserve.
const DefaultMaxAssistantChars = 8000

// DefaultFoldDedupMinDup is the minimum duplicate length (runes) that
// DedupRepeatedText looks for inside each fold segment. Tuned to 20
// to match the D1 streaming-time dedup threshold — short enough to
// catch the minimax M2.7 21-rune tail-rewrite loop (e.g. "测试覆盖、
// 规范一致性），适合并行分解执行。" repeated verbatim in the
// buffer), large enough to avoid false positives on common phrases.
const DefaultFoldDedupMinDup = 20

// DefaultFoldDedupMinGap is the minimum non-overlapping gap (runes)
// between the two duplicate positions required for a match. A 2-rune
// gap matches the streaming-time dedup so both layers stay consistent.
const DefaultFoldDedupMinGap = 2

// TurnOutputRecord is a persisted oversized assistant output.
type TurnOutputRecord struct {
	SessionID string    `json:"session_id"`
	TurnNum   int       `json:"turn_num"`
	Role      string    `json:"role"`
	FullPath  string    `json:"full_path"`
	Size      int       `json:"size"`
	HeadLen   int       `json:"head_len"`
	TailLen   int       `json:"tail_len"`
	CreatedAt time.Time `json:"created_at"`
}

// FoldAssistantOutput persists content to disk when it exceeds maxChars
// and returns the head/tail folded version. If content is within the
// limit, content is returned unchanged and no file is written.
//
// Parameters:
//
//   - store        : backing store (we reuse its on-disk root).
//   - sessionID    : session identifier (used as sub-dir).
//   - turnNum      : turn index (used in the file name + record).
//   - role         : message role, recorded for debugging.
//   - content      : full assistant content.
//   - maxChars     : soft cap; 0 → DefaultMaxAssistantChars.
//   - headChars    : leading runes preserved; 0 → DefaultAssistantHead.
//   - tailChars    : trailing runes preserved; 0 → DefaultAssistantTail.
//
// Returned format:
//
//	<prior-output-summary>
//	…(first 800 chars)
//
//	…[middle 12345 chars truncated; see /path/to/file]
//
//	…(last 200 chars)
//	</prior-output-summary>
func FoldAssistantOutput(
	store *ToolResultStore,
	sessionID string,
	turnNum int,
	role string,
	content string,
	maxChars, headChars, tailChars int,
) (string, error) {
	if maxChars <= 0 {
		maxChars = DefaultMaxAssistantChars
	}
	if headChars <= 0 {
		headChars = DefaultAssistantHead
	}
	if tailChars <= 0 {
		tailChars = DefaultAssistantTail
	}
	if utf8.RuneCountInString(content) <= maxChars {
		return content, nil
	}
	if store == nil {
		// No store → fall back to a pure head truncation so we still keep
		// the response bounded.
		return truncateHead(content, maxChars) + "\n...[no store available; truncated]", nil
	}

	root, err := store.resolveRoot()
	if err != nil {
		return "", fmt.Errorf("turn output store: resolve root: %w", err)
	}
	dir := filepath.Join(root, sanitizeSegment(sessionID), "turn-outputs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("turn output store: mkdir %s: %w", dir, err)
	}

	id, err := randomID(8)
	if err != nil {
		return "", fmt.Errorf("turn output store: id: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102T150405.000")
	fullPath := filepath.Join(dir, fmt.Sprintf("t%d-%s-%s.txt", turnNum, stamp, id))

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("turn output store: write %s: %w", fullPath, err)
	}

	head := truncateHead(content, headChars)
	tail := truncateTail(content, tailChars)
	// DM-20260625-008: dedup LLM streaming-loop pollution out of the
	// fold summary before it lands in the next-turn prompt. When the
	// provider emits the same long phrase twice in a single reply
	// (minimax M2.7 streaming artifact), without this pass the second
	// copy gets carried verbatim into <prior-output-summary> content
	// and biases the next LLM call toward replaying the same loop. Run
	// dedup on head and tail separately so the fold boundary does not
	// create a spurious cross-segment match.
	head = textutil.DedupRepeatedText(head, DefaultFoldDedupMinDup, DefaultFoldDedupMinGap)
	tail = textutil.DedupRepeatedText(tail, DefaultFoldDedupMinDup, DefaultFoldDedupMinGap)
	mid := utf8.RuneCountInString(content) - headChars - tailChars
	if mid < 0 {
		mid = 0
	}

	var b strings.Builder
	b.WriteString("<prior-output-summary>\n")
	b.WriteString(head)
	b.WriteString("\n\n...[middle ")
	fmt.Fprintf(&b, "%d", mid)
	b.WriteString(" chars truncated; see ")
	b.WriteString(fullPath)
	b.WriteString("]\n\n")
	b.WriteString(tail)
	b.WriteString("\n</prior-output-summary>")
	_ = context.Background()
	return b.String(), nil
}

// truncateTail returns the last n runes of s. Returns the full string
// when n ≥ len(s).
func truncateTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[len(runes)-n:])
}