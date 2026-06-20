package persist

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFoldAssistantOutput_BelowLimit_PassesThrough(t *testing.T) {
	store := NewToolResultStore(t.TempDir())
	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", "short", 1000, 800, 200)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}
	if got != "short" {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestFoldAssistantOutput_AboveLimit_FoldsHeadTail(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// 2000 chars; limit 100; head 50; tail 30.
	content := strings.Repeat("abcdefghij", 200)
	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 100, 50, 30)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}

	if !strings.Contains(got, "<prior-output-summary>") {
		t.Errorf("expected summary marker, got %q", got)
	}
	if !strings.Contains(got, "chars truncated; see ") {
		t.Errorf("expected truncation marker, got %q", got)
	}
	if !strings.Contains(got, "/turn-outputs/t1-") {
		t.Errorf("expected turn-outputs path, got %q", got)
	}

	// File must exist on disk.
	sessDir := filepath.Join(dir, "sess1", "turn-outputs")
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", sessDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	fullPath := filepath.Join(sessDir, entries[0].Name())
	body, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != content {
		t.Errorf("file body mismatch: got %d bytes want %d", len(body), len(content))
	}
}

func TestFoldAssistantOutput_NilStore_StillTruncates(t *testing.T) {
	content := strings.Repeat("abcdefghij", 100)
	got, err := FoldAssistantOutput(nil, "sess1", 1, "assistant", content, 100, 50, 30)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}
	if !strings.Contains(got, "no store available") {
		t.Errorf("expected no-store marker, got %q", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("expected truncation marker, got %q", got)
	}
}

func TestFoldAssistantOutput_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// No max/head/tail → defaults.
	content := strings.Repeat("x", 20000)
	got, err := FoldAssistantOutput(store, "sess1", 1, "assistant", content, 0, 0, 0)
	if err != nil {
		t.Fatalf("FoldAssistantOutput: %v", err)
	}
	if !strings.Contains(got, "<prior-output-summary>") {
		t.Errorf("expected summary marker with defaults, got %q", got)
	}
}

func TestTruncateTail(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "llo"},
		{"hello", 0, ""},
		{"hello", -1, ""},
		{"😀😁😂🤣😃", 2, "🤣😃"},
	}
	for _, tt := range tests {
		if got := truncateTail(tt.s, tt.n); got != tt.want {
			t.Errorf("truncateTail(%q, %d): got %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

// _ = context is to silence unused-import warnings if a test is later
// deleted; keeps the file compilable across edits.
var _ = context.Background