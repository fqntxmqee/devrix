package contextengine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
)

// Covers: L5-CTX-14
func TestBuiltinVerifyRunner_should_run_go_version(t *testing.T) {
	dir := t.TempDir()
	runner := contextengine.NewBuiltinVerifyRunner(dir)
	result, err := runner.Run(context.Background(), contextengine.VerifyCommand{
		Name:       "go-version",
		Executable: "go",
		Args:       []string{"version"},
		WorkDir:    dir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d stderr=%s", result.ExitCode, result.Stderr)
	}
}

// Covers: L5-CTX-15
func TestBuiltinVerifyRunner_should_reject_workdir_mismatch(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "nested")
	_ = os.Mkdir(other, 0o755)
	runner := contextengine.NewBuiltinVerifyRunner(dir)
	_, err := runner.Run(context.Background(), contextengine.VerifyCommand{
		Name:       "go-version",
		Executable: "go",
		Args:       []string{"version"},
		WorkDir:    other,
	})
	if err == nil {
		t.Fatal("expected workdir mismatch error")
	}
}
