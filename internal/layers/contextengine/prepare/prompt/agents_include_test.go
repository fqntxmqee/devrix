package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandAgentsIncludes_should_inline_file(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "AGENTS.md")
	include := filepath.Join(dir, "extra.md")
	if err := os.WriteFile(include, []byte("included body"), 0o644); err != nil {
		t.Fatal(err)
	}
	content := "header\n@extra.md\nfooter"
	got := expandAgentsIncludes(content, main, true, nil, 0)
	if !strings.Contains(got, "included body") {
		t.Fatalf("expected included content, got %q", got)
	}
	if strings.Contains(got, "@extra.md") {
		t.Fatal("include directive should be replaced")
	}
}

func TestExpandAgentsIncludes_should_skip_code_blocks(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "AGENTS.md")
	include := filepath.Join(dir, "skip.md")
	if err := os.WriteFile(include, []byte("must-not-load"), 0o644); err != nil {
		t.Fatal(err)
	}
	content := "```\n@skip.md\n```\n"
	got := expandAgentsIncludes(content, main, true, nil, 0)
	if strings.Contains(got, "must-not-load") {
		t.Fatal("includes inside code blocks must be ignored")
	}
}

func TestExpandAgentsIncludes_should_prevent_cycles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.md")
	b := filepath.Join(dir, "b.md")
	if err := os.WriteFile(a, []byte("@b.md"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("@a.md\nb-body"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := expandAgentsIncludes("@b.md", a, true, nil, 0)
	if !strings.Contains(got, "b-body") {
		t.Fatalf("expected b content once, got %q", got)
	}
}
