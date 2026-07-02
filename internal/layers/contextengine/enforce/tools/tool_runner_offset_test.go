// T: D2-S15-A02-T10 — read_file offset/limit unit tests.
package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// D2-S15-A02-T10: offset/limit defaults to Offset=0, Limit=8192.
func TestReadFile_OffsetLimit_Defaults(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// No offset/limit → defaults applied
	res, err := runReadFile(dir, `{"path":"small.txt"}`, &toolExecConfig{})
	if err != nil {
		t.Fatalf("runReadFile: %v", err)
	}
	if !strings.Contains(res.Output, "hello world") {
		t.Errorf("default read should return full content, got %q", res.Output)
	}
}

// D2-S15-A02-T10: Offset > 0 reads the tail (治本 recovery path).
func TestReadFile_OffsetLimit_OffsetReadsTail(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "medium.txt")
	content := strings.Repeat("a", 5000) + strings.Repeat("b", 5000) // 10K bytes
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := runReadFile(dir, `{"path":"medium.txt","offset":5000,"limit":8192}`, &toolExecConfig{})
	if err != nil {
		t.Fatalf("runReadFile: %v", err)
	}
	if strings.Contains(res.Output, "a") {
		t.Errorf("offset=5000 should not return any 'a' chars, got %q", first80(res.Output))
	}
	if !strings.Contains(res.Output, "b") {
		t.Errorf("offset=5000 should return the 'b' tail, got %q", first80(res.Output))
	}
}

// D2-S15-A02-T10: Offset past EOF returns empty (NOT an error) so the
// LLM gets a stable "end of file" signal.
func TestReadFile_OffsetLimit_PastEOFReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tiny.txt")
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := runReadFile(dir, `{"path":"tiny.txt","offset":10000,"limit":8192}`, &toolExecConfig{})
	if err != nil {
		t.Fatalf("runReadFile: %v", err)
	}
	if res.Error != "" {
		t.Errorf("offset past EOF must NOT be an error, got Error=%q", res.Error)
	}
	if res.Output != "" {
		t.Errorf("offset past EOF must return empty output, got %q", res.Output)
	}
}

// D2-S15-A02-T10: limit=0 falls back to 8192 (defensive).
func TestReadFile_OffsetLimit_LimitZeroFallback(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tenk.txt")
	if err := os.WriteFile(p, []byte(strings.Repeat("x", 10000)), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := runReadFile(dir, `{"path":"tenk.txt","limit":0}`, &toolExecConfig{})
	if err != nil {
		t.Fatalf("runReadFile: %v", err)
	}
	// 8192 default → reads first 8192 bytes; file has 10000.
	if len(res.Output) != 8192 {
		t.Errorf("limit=0 should fall back to 8192, got %d bytes", len(res.Output))
	}
}

func first80(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}
