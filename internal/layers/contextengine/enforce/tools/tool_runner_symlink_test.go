package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

// T: D2-S18-A81-T01 (DM-20260630-013 RH-D2-02)
//
// resolveWorkspacePath must reject symlinks whose realpath escapes the
// workspace root. Without this guard, a malicious or stale symlink inside
// the workspace could rewrite files outside it.
func TestResolveWorkspacePath_rejectsExternalSymlink(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(workDir, "leak.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveWorkspacePath(workDir, "leak.txt"); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

// T: D2-S18-A81-T02 (DM-20260630-013 RH-D2-02)
//
// Symlinks whose realpath stays inside the workspace are valid and must
// remain allowed. We must not over-block legitimate indirection.
func TestResolveWorkspacePath_allowsInternalSymlink(t *testing.T) {
	workDir := t.TempDir()
	target := filepath.Join(workDir, "real.txt")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(workDir, "alias.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}

	got, err := resolveWorkspacePath(workDir, "alias.txt")
	if err != nil {
		t.Fatalf("internal symlink should be allowed: %v", err)
	}
	if got != linkPath {
		t.Fatalf("expected %q, got %q", linkPath, got)
	}
}

// T: D2-S18-A81-T03 (DM-20260630-013 RH-D2-02 regression)
//
// Non-symlink paths must continue to resolve as before — the new
// containment check is a no-op when there is no symlink involved.
func TestResolveWorkspacePath_plainPath(t *testing.T) {
	workDir := t.TempDir()
	got, err := resolveWorkspacePath(workDir, "file.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(workDir, "file.md")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// T: D2-S18-A81-T04 (DM-20260630-013 symlink directory regression)
//
// A symlink directory pointing outside the workspace must not allow creation of
// a new missing child below that symlink. EvalSymlinks(target) alone misses this
// because the final target does not exist yet.
func TestResolveWorkspacePath_rejectsNewFileUnderExternalSymlinkDir(t *testing.T) {
	workDir := t.TempDir()
	outsideDir := t.TempDir()
	linkPath := filepath.Join(workDir, "outside")
	if err := os.Symlink(outsideDir, linkPath); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveWorkspacePath(workDir, "outside/new.txt"); err == nil {
		t.Fatal("expected symlink directory escape to be rejected for missing child")
	}
}

// T: D2-S18-A80-T03 (DM-20260630-013 plan-mode bash fail-closed)
func TestRunBash_planModeDenied(t *testing.T) {
	workDir := t.TempDir()
	sc := &types.SessionContext{
		PermissionMode: types.PermissionPlan,
		PlanFilePath:   filepath.Join(workDir, "plan.md"),
	}
	ctx := WithToolSessionContext(WithToolWorkDir(context.Background(), workDir), sc)
	got, err := runBash(ctx, workDir, `printf x > other.md`, newToolExecConfig(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !strings.Contains(got.Error, "plan mode: bash denied") {
		t.Fatalf("expected plan-mode bash denial, got %#v", got)
	}
}

func TestRedactCommandForAudit(t *testing.T) {
	command := `curl -H "Authorization: Bearer secret-token" 'https://x.test?api_key=abc' PASSWORD=hunter2`
	got := redactCommandForAudit(command)
	for _, secret := range []string{"secret-token", "api_key=abc", "hunter2"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted command still contains %q: %s", secret, got)
		}
	}
}
