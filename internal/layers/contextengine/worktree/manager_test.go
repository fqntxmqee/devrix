package worktree_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/worktree"
	"github.com/devrix/devrix/internal/shared/config"
)

// Covers: L5-4-12-01
func TestManager_should_isolate_writes_from_main_workdir(t *testing.T) {
	mainDir := t.TempDir()
	baseDir := filepath.Join(t.TempDir(), "worktrees")
	mgr := worktree.NewManager(config.WorktreeConfig{Enabled: true, BaseDir: baseDir})

	wtPath, err := mgr.Enter(context.Background(), "sess_wt", "impl-a", mainDir)
	if err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "worker.txt"), []byte("worker-only"), 0o644); err != nil {
		t.Fatalf("write worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mainDir, "worker.txt")); !os.IsNotExist(err) {
		t.Fatal("main workdir should not contain worker.txt")
	}
	if err := mgr.Exit(context.Background(), wtPath, false); err != nil {
		t.Fatalf("Exit: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatal("worktree path should be removed after Exit")
	}
}

func TestManager_should_reject_invalid_slug(t *testing.T) {
	mgr := worktree.NewManager(config.WorktreeConfig{Enabled: true, BaseDir: t.TempDir()})
	if _, err := mgr.Enter(context.Background(), "sess", "../escape", "/tmp"); err == nil {
		t.Fatal("expected invalid slug error")
	}
}
