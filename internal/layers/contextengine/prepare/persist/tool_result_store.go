// Package persist contains D2-side persistence helpers for the prepare layer.
//
// tool_result_store.go implements DM-20260620-001 / AC1: oversized tool
// results (e.g. large grep / read_file / bash output) are persisted to disk
// and replaced in the in-band LLM message with a small preview marker so
// subsequent turns do not blow up the LLM context budget.
//
// DSAFT: D2-S17-A05 extension.
package persist

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultRoot is the default on-disk root for persisted tool results.
const DefaultRoot = "~/.devrix/tool-results"

// DefaultMaxChars is the soft cap above which a tool result is persisted.
// Chosen to keep a single tool result under ~3K tokens of typical prose.
const DefaultMaxChars = 12000

// DefaultPreviewChars is the number of leading characters preserved in the
// in-band preview marker.
const DefaultPreviewChars = 2000

// DefaultRetentionDays is how long persisted records are retained before GC.
const DefaultRetentionDays = 7

// sizeCappedTools is the allowlist of tool names whose output is subject
// to the size cap. We deliberately exclude JSON-returning orchestration
// tools (task_*, delegate_*, etc.) so structured payloads are never split.
var sizeCappedTools = map[string]bool{
	"read_file": true,
	"bash:grep": true,
	"bash:rg":   true,
	"bash:find": true,
	"bash:ls":   true,
	"bash:cat":  true,
	"bash:head": true,
	"bash:tail": true,
}

// ShouldCap reports whether the given tool name should be subject to the
// result-size cap. Exposed so callers (e.g. D7 orchestrator) can early-out
// without running rune counts on every result.
func ShouldCap(toolName string) bool {
	return sizeCappedTools[toolName]
}

// ToolResultRecord is a persisted oversized tool result.
type ToolResultRecord struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	ToolName   string    `json:"tool_name"`
	ToolUseID  string    `json:"tool_use_id"`
	FullPath   string    `json:"full_path"`
	Size       int       `json:"size"`
	PreviewLen int       `json:"preview_len"`
	CreatedAt  time.Time `json:"created_at"`
}

// ToolResultStore writes oversized tool results to disk and returns a
// preview marker suitable for replacing the in-band LLM content.
//
// Concurrency: methods are safe for concurrent use; each Persist() writes
// to a fresh file under a per-session sub-directory.
type ToolResultStore struct {
	Root string
}

// NewToolResultStore constructs a store rooted at root. An empty root
// defaults to DefaultRoot (resolved via expandHome at Persist time).
func NewToolResultStore(root string) *ToolResultStore {
	if strings.TrimSpace(root) == "" {
		root = DefaultRoot
	}
	return &ToolResultStore{Root: root}
}

// Persist writes content to disk if it exceeds maxChars and returns the
// in-band preview marker that should replace content in the LLM message.
// If content is within the limit, content is returned unchanged and no
// file is written.
//
// Format of the preview marker:
//
//	<persisted-output>
//	Output too large (12.3 KB). Full output saved to: /path/to/file
//	Preview (first 2000 chars):
//	{preview}
//	...</persisted-output>
func (s *ToolResultStore) Persist(
	ctx context.Context,
	sessionID, toolName, toolUseID string,
	content string,
	maxChars int,
) (string, error) {
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	if utf8.RuneCountInString(content) <= maxChars {
		return content, nil
	}

	root, err := s.resolveRoot()
	if err != nil {
		return "", fmt.Errorf("tool result store: resolve root: %w", err)
	}
	dir := filepath.Join(root, sanitizeSegment(sessionID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("tool result store: mkdir %s: %w", dir, err)
	}

	id, err := randomID(8)
	if err != nil {
		return "", fmt.Errorf("tool result store: id: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102T150405.000")
	fullPath := filepath.Join(dir, fmt.Sprintf("%s-%s.txt", stamp, id))

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("tool result store: write %s: %w", fullPath, err)
	}

	preview := truncateHead(content, DefaultPreviewChars)
	marker := buildMarker(fullPath, len(content), DefaultPreviewChars, preview)
	_ = ctx // future: telemetry hook
	_ = toolUseID
	return marker, nil
}

// List returns all persisted records for the given session, newest first.
// Records are inferred from files on disk under <root>/<sessionID>/.
func (s *ToolResultStore) List(sessionID string) ([]ToolResultRecord, error) {
	root, err := s.resolveRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, sanitizeSegment(sessionID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("tool result store: readdir %s: %w", dir, err)
	}
	records := make([]ToolResultRecord, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		records = append(records, ToolResultRecord{
			SessionID: sessionID,
			FullPath:  fullPath,
			Size:      int(info.Size()),
			CreatedAt: info.ModTime().UTC(),
		})
	}
	return records, nil
}

// GC removes persisted records older than retentionDays. Returns the
// number of files removed.
func (s *ToolResultStore) GC(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}
	root, err := s.resolveRoot()
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	removed := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		return nil
	})
	if err != nil {
		return removed, fmt.Errorf("tool result store: gc: %w", err)
	}
	return removed, nil
}

// resolveRoot expands ~ if present and returns an absolute path.
func (s *ToolResultStore) resolveRoot() (string, error) {
	root := s.Root
	if strings.HasPrefix(root, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, strings.TrimPrefix(root, "~"))
	}
	return filepath.Abs(root)
}

// sanitizeSegment strips path separators from session IDs so they cannot
// escape the per-session sub-directory.
func sanitizeSegment(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "_"
	}
	return s
}

// randomID returns a hex-encoded random ID of the given byte length.
func randomID(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// truncateHead returns the first n runes of s as a string, preserving
// UTF-8 boundaries. Appends a trailing ellipsis indicator when truncated.
func truncateHead(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "\n…"
		}
		count++
	}
	return s
}

// buildMarker renders the in-band preview message.
//
// size is byte length (not runes) for human readability; preview is the
// already-truncated head of the original content.
func buildMarker(fullPath string, size, previewLen int, preview string) string {
	var b strings.Builder
	b.WriteString("<persisted-output>\n")
	fmt.Fprintf(&b, "Output too large (%.1f KB). Full output saved to: %s\n",
		float64(size)/1024.0, fullPath)
	fmt.Fprintf(&b, "Preview (first %d chars):\n%s\n", previewLen, preview)
	b.WriteString("</persisted-output>")
	return b.String()
}