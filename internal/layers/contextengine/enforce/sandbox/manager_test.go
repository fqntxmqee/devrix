package sandbox_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/sandbox"
	"github.com/devrix/devrix/internal/shared/config"
)

func TestManager_EnterIsolatesWritesFromPrimaryWorkDir(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "sandboxes")
	mgr := sandbox.NewManager(config.SandboxConfig{Enabled: true, BaseDir: baseDir})
	primary := t.TempDir()
	path, err := mgr.Enter(context.Background(), "sess1", "impl-a", primary)
	if err != nil {
		t.Fatalf("Enter: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "out.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write sandbox: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primary, "out.txt")); !os.IsNotExist(err) {
		t.Fatal("primary WorkDir must not receive sandbox writes")
	}
	if err := mgr.Exit(context.Background(), path, false); err != nil {
		t.Fatalf("Exit: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("sandbox path should be removed after Exit")
	}
}
