// T01/T08 tests for PersistToFile / BuildPersistedMessage / generatePreview.
// DM-20260702-008 / D2-S15-A02-T01.
package compression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// flatCounter treats every 4 chars as 1 token and never truncates, so
// 10 000 chars of input is reported as 2 500 tokens — well over the
// 800-token gate in the budget step.
type flatCounter struct{}

func (flatCounter) CountText(text string) int { return (len(text) + 3) / 4 }
func (flatCounter) CountMessages(_ []types.Message) int { return 0 }
func (flatCounter) CountWithSystemPrompt(_ string, _ []types.Message) int { return 0 }
func (flatCounter) TruncateToTokens(text string, _ int) string { return text }
func (flatCounter) EncodingForModel(_ string) string { return "test" }

var _ contracts.ITokenCounter = flatCounter{}

func TestPersistToFile_UnderThreshold_ReturnsUnchanged(t *testing.T) {
	dir := t.TempDir()
	content := "short tool output"
	res, err := PersistToFile(content, "call_1", 800, dir, "sess1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Preview != content {
		t.Errorf("under-threshold content must be returned unchanged, got %q", res.Preview)
	}
	if res.FilePath != "" {
		t.Errorf("no file should be written under threshold, got %s", res.FilePath)
	}
}

func TestPersistToFile_OverThreshold_WritesFileAndPreview(t *testing.T) {
	dir := t.TempDir()
	// 8.5K chars > 800-token (≈3.2K-char) threshold.
	content := strings.Repeat("a", 8500)
	res, err := PersistToFile(content, "call_over", 800, dir, "sessA")
	if err != nil {
		t.Fatalf("persist failed: %v", err)
	}
	if res.OriginalSize != len(content) {
		t.Errorf("OriginalSize = %d, want %d", res.OriginalSize, len(content))
	}
	if res.FilePath == "" {
		t.Fatal("FilePath must be set on persisted result")
	}
	onDisk, err := os.ReadFile(res.FilePath)
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}
	if string(onDisk) != content {
		t.Errorf("persisted file content mismatch (len on disk = %d, want %d)",
			len(onDisk), len(content))
	}
	if len(res.Preview) > PreviewSizeBytes {
		t.Errorf("preview must be <= %d bytes, got %d", PreviewSizeBytes, len(res.Preview))
	}
	if res.Preview != content[:len(res.Preview)] {
		t.Errorf("preview must be a head slice of the original content")
	}
	if !res.HasMore {
		t.Errorf("HasMore must be true when content was truncated")
	}
	wantPath := filepath.Join(dir, "sessA", "tool-results", "call_over.txt")
	if res.FilePath != wantPath {
		t.Errorf("FilePath = %q, want %q", res.FilePath, wantPath)
	}
}

func TestPersistToFile_ReplaySafe_IdempotentOnEEXIST(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("b", 5000)
	first, err := PersistToFile(content, "call_replay", 800, dir, "sessB")
	if err != nil {
		t.Fatalf("first persist: %v", err)
	}
	// Second invocation with same toolUseID must NOT error — file already
	// exists from the first call, microcompact replays must be safe.
	second, err := PersistToFile(content, "call_replay", 800, dir, "sessB")
	if err != nil {
		t.Fatalf("replay persist must not error, got: %v", err)
	}
	if second.FilePath != first.FilePath {
		t.Errorf("replay must return the same path, got %q vs %q", second.FilePath, first.FilePath)
	}
	onDisk, _ := os.ReadFile(second.FilePath)
	if string(onDisk) != content {
		t.Errorf("replay must not rewrite the file (len on disk = %d, want %d)",
			len(onDisk), len(content))
	}
}

func TestPersistToFile_ImageBlock_Skipped(t *testing.T) {
	dir := t.TempDir()
	// Even when the size triggers the threshold, an image block must
	// NOT be persisted — the LLM needs the bytes inline for multimodal
	// inference (mirrors clawcode maybePersistLargeToolResult).
	imageContent := "data:image/png;base64," + strings.Repeat("A", 10000)
	res, err := PersistToFile(imageContent, "call_img", 100, dir, "sessC")
	if err != nil {
		t.Fatalf("image block: %v", err)
	}
	if res.FilePath != "" {
		t.Errorf("image block must not produce a FilePath, got %q", res.FilePath)
	}
	if res.Preview != imageContent {
		t.Errorf("image block must be returned as-is to the caller, got len %d", len(res.Preview))
	}
	// The devrix adapter's "[image attached: ...]" marker should also short-circuit.
	marker := "[image attached: foo.png]\nsome inline text"
	res2, err := PersistToFile(marker, "call_img2", 10, dir, "sessC")
	if err != nil {
		t.Fatalf("marker image block: %v", err)
	}
	if res2.FilePath != "" || res2.Preview != marker {
		t.Errorf("marker-prefixed image must be returned as-is, got FilePath=%q Preview=%q",
			res2.FilePath, res2.Preview)
	}
}

func TestPersistToFile_EmptySessionID_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := PersistToFile(strings.Repeat("x", 5000), "call_x", 100, dir, "")
	if err == nil {
		t.Fatal("empty sessionID must produce a typed error for fall-back path")
	}
	if _, ok := err.(*PersistToFileError); !ok {
		t.Errorf("expected *PersistToFileError, got %T", err)
	}
}

func TestPersistToFile_EmptyProjectDir_ReturnsError(t *testing.T) {
	_, err := PersistToFile(strings.Repeat("y", 5000), "call_y", 100, "", "sessD")
	if err == nil {
		t.Fatal("empty projectDir must produce a typed error for fall-back path")
	}
}

func TestPersistToFile_InvalidProjectDir_ReturnsError(t *testing.T) {
	// mkdir under a regular file → ENOTDIR. A deterministic fail that
	// gives the caller a typed error so it can fall back to truncate.
	roFile := t.TempDir() + "/regular_file"
	if err := os.WriteFile(roFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	bogus := filepath.Join(roFile, "nope")
	_, err := PersistToFile(strings.Repeat("z", 5000), "call_z", 100, bogus, "sessE")
	if err == nil {
		t.Fatal("unwritable projectDir must produce a typed error so the caller can fall back")
	}
	if _, ok := err.(*PersistToFileError); !ok {
		t.Errorf("expected *PersistToFileError for fall-back, got %T", err)
	}
}

func TestBuildPersistedMessage_WrapsPreviewInXML(t *testing.T) {
	res := PersistedToolResult{
		FilePath:     "/tmp/foo/sess/tool-results/abc.txt",
		OriginalSize: 12345,
		Preview:      "first 2000 chars of content",
		HasMore:      true,
	}
	msg := BuildPersistedMessage(res)
	if !strings.HasPrefix(msg, PersistedOutputTag) {
		t.Errorf("message must start with %s, got: %q", PersistedOutputTag, firstLine(msg))
	}
	if !strings.HasSuffix(msg, PersistedOutputClosingTag) {
		t.Errorf("message must end with %s, got: %q", PersistedOutputClosingTag, firstLine(msg))
	}
	if !strings.Contains(msg, res.FilePath) {
		t.Errorf("message must reference the persisted file path")
	}
	if !strings.Contains(msg, res.Preview) {
		t.Errorf("message must include the preview head")
	}
	if !strings.Contains(msg, "...") {
		t.Errorf("truncated preview must show '...' marker, got: %s", firstLine(msg))
	}
}

func TestGeneratePreview_CutAtNewline(t *testing.T) {
	// Body: "line\n" repeated. With a 100-byte cap, the cut should land
	// at a newline boundary (>= 50% of cap) so the preview is a whole
	// number of "line" tokens — no partial "line" suffix and no trailing
	// newline. This is what makes the preview human-readable in a log.
	body := strings.Repeat("line\n", 500) // 2500 chars
	preview, hasMore := generatePreview(body, 100)
	if !hasMore {
		t.Fatal("content over cap must produce hasMore=true")
	}
	if strings.HasSuffix(preview, "\n") {
		t.Errorf("preview should be cut AT a newline (no trailing newline), got %q", preview)
	}
	if !strings.HasSuffix(preview, "line") {
		t.Errorf("preview should end at a whole 'line' token boundary, got %q", preview)
	}
}

func TestGeneratePreview_NoCutWhenUnderCap(t *testing.T) {
	preview, hasMore := generatePreview("hello", 100)
	if hasMore {
		t.Error("content under cap must produce hasMore=false")
	}
	if preview != "hello" {
		t.Errorf("under-cap content must round-trip, got %q", preview)
	}
}

func TestToolResultBudget_PersistsAndFallsBack(t *testing.T) {
	// End-to-end smoke: a 10K-char tool result must round-trip through
	// the budget step and come back wrapped in <persisted-output> when
	// the persist dir is writable, and wrapped in [TRUNCATED ...] when
	// the dir is unwritable.
	dir := t.TempDir()
	large := strings.Repeat("L", 10000)
	msgs := []types.Message{
		{ID: "tool_msg_1", SessionID: "sess_int", Role: types.MessageRoleTool, Content: large, Timestamp: time.Now()},
	}
	out := toolResultBudget(flatCounter{}, msgs, 800, dir, "sess_int")
	if !strings.Contains(out[0].Content, PersistedOutputTag) {
		t.Errorf("writable projectDir must produce a <persisted-output> wrapper, got: %s",
			firstLine(out[0].Content))
	}
	wantPath := filepath.Join(dir, "sess_int", "tool-results", "tool_msg_1.txt")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("persisted file should exist on disk at %s, got: %v", wantPath, err)
	}

	// Now run the same input with an unwritable project dir — must
	// fall back to the truncate-with-marker path.
	fallback := toolResultBudget(flatCounter{}, msgs, 800, "/nonexistent-readonly-root-xyz", "sess_int")
	if !strings.Contains(fallback[0].Content, "complete=false") {
		t.Errorf("unwritable dir must fall back to truncate-with-marker, got: %s",
			firstLine(fallback[0].Content))
	}
	if strings.Contains(fallback[0].Content, PersistedOutputTag) {
		t.Errorf("fallback must NOT contain <persisted-output>, got: %s",
			firstLine(fallback[0].Content))
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
