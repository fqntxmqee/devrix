package harness_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/config"
)

// Covers: L5-2-9-02
func TestScanWorkspace_should_count_go_files(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# agents"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws, err := harness.ScanWorkspace(dir, config.HarnessPrefetchConfig{Enabled: true, MaxWalkDepth: 2})
	if err != nil {
		t.Fatalf("ScanWorkspace: %v", err)
	}
	if ws.GoFileCount != 2 {
		t.Fatalf("go files: got %d want 2", ws.GoFileCount)
	}
	if ws.TestFileCount != 1 {
		t.Fatalf("test files: got %d want 1", ws.TestFileCount)
	}
	if !ws.AgentsMDPresent {
		t.Fatal("expected AGENTS.md detected")
	}
}

// Covers: L5-2-9-02
func TestCheckGuards_should_require_writable_workdir(t *testing.T) {
	dir := t.TempDir()
	if err := harness.CheckGuards(dir); err != nil {
		t.Fatalf("CheckGuards: %v", err)
	}
}
