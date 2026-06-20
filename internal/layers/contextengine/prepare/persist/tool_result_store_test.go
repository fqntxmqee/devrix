package persist

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldCap(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     bool
	}{
		{"read_file capped", "read_file", true},
		{"bash:grep capped", "bash:grep", true},
		{"bash:rg capped", "bash:rg", true},
		{"bash:cat capped", "bash:cat", true},
		{"task_create NOT capped", "task_create", false},
		{"delegate_worker NOT capped", "delegate_worker", false},
		{"empty NOT capped", "", false},
		{"bash (no subcommand) NOT capped", "bash", false},
		{"unknown NOT capped", "unknown_tool", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldCap(tt.toolName); got != tt.want {
				t.Errorf("ShouldCap(%q): got %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestPersist_BelowLimit_ReturnsUnchanged(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	small := "short output"
	got, err := store.Persist(context.Background(), "sess1", "read_file", "call_1", small, 1000)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if got != small {
		t.Errorf("expected unchanged content, got %q", got)
	}

	// Nothing should have been written.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files, found %d", len(entries))
	}
}

func TestPersist_AboveLimit_WritesFileAndReturnsMarker(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	big := strings.Repeat("abcdefghij", 2000) // 20 000 bytes
	got, err := store.Persist(context.Background(), "sess/abc", "read_file", "call_1", big, 1024)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}

	if !strings.Contains(got, "<persisted-output>") {
		t.Errorf("expected marker start, got %q", got)
	}
	if !strings.Contains(got, "Output too large") {
		t.Errorf("expected 'Output too large' label, got %q", got)
	}
	if !strings.Contains(got, "Preview (first") {
		t.Errorf("expected preview header, got %q", got)
	}
	if !strings.Contains(got, "/sess_abc/") {
		t.Errorf("expected sanitized session subdir in marker, got %q", got)
	}
	if strings.Contains(got, "sess/abc") {
		t.Errorf("expected sanitization of session segment, got %q", got)
	}

	// File must exist under per-session dir.
	sessDir := filepath.Join(dir, "sess_abc")
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
	if string(body) != big {
		t.Errorf("file body mismatch: len got %d want %d", len(body), len(big))
	}
}

func TestList_EmptyWhenNoSessionDir(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	records, err := store.List("nope")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestList_ReturnsRecords(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	ctx := context.Background()
	if _, err := store.Persist(ctx, "sess1", "read_file", "c1", strings.Repeat("x", 5000), 100); err != nil {
		t.Fatalf("Persist 1: %v", err)
	}
	if _, err := store.Persist(ctx, "sess1", "read_file", "c2", strings.Repeat("y", 5000), 100); err != nil {
		t.Fatalf("Persist 2: %v", err)
	}

	records, err := store.List("sess1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
	for _, r := range records {
		if r.Size == 0 {
			t.Errorf("expected non-zero size, got %+v", r)
		}
		if !strings.HasPrefix(r.FullPath, dir) {
			t.Errorf("FullPath should be under root: got %s", r.FullPath)
		}
	}
}

func TestGC_RemovesOldFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewToolResultStore(dir)

	// Manually create a file with an old mtime.
	sessDir := filepath.Join(dir, "sess1")
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	oldPath := filepath.Join(sessDir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	oldTime := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// And a fresh one.
	freshPath := filepath.Join(sessDir, "fresh.txt")
	if err := os.WriteFile(freshPath, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	removed, err := store.GC(7)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("expected old file removed")
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Errorf("expected fresh file preserved: %v", err)
	}
}

func TestTruncateHead_PreservesUTF8(t *testing.T) {
	// 4-byte chars × 5 = 20 bytes; truncateHead should cut at rune boundary.
	s := "😀😁😂🤣😃" // 5 runes, 20 bytes
	got := truncateHead(s, 3)
	if got != "😀😁😂\n…" {
		t.Errorf("truncateHead: got %q, want %q", got, "😀😁😂\n…")
	}
}

func TestSanitizeSegment(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"sess_abc", "sess_abc"},
		{"sess/abc", "sess_abc"},
		{"sess\\abc", "sess_abc"},
		{"sess..abc", "sess_abc"},
		{"", "_"},
	}
	for _, tt := range tests {
		if got := sanitizeSegment(tt.in); got != tt.want {
			t.Errorf("sanitizeSegment(%q): got %q, want %q", tt.in, got, tt.want)
		}
	}
}