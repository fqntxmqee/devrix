package prompt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeGitStatus_should_return_false_outside_git(t *testing.T) {
	dir := t.TempDir()
	if _, ok := computeGitStatus(dir); ok {
		t.Fatal("expected false for non-git directory")
	}
}

func TestComputeGitStatus_should_include_branch_in_repo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "README.md"), "hello")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")

	out, ok := computeGitStatus(dir)
	if !ok {
		t.Fatal("expected git status in initialized repo")
	}
	if !strings.Contains(out, "Current branch:") {
		t.Fatalf("missing branch line: %q", out)
	}
	if !strings.Contains(out, "Recent commits:") {
		t.Fatalf("missing recent commits: %q", out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
