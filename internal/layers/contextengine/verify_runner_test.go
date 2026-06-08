package contextengine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// Covers: L5-CTX-26
func TestBuiltinVerifyRunner_should_return_deadline_exceeded_on_timeout(t *testing.T) {
	dir := t.TempDir()
	runner := contextengine.NewBuiltinVerifyRunner(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := runner.Run(ctx, contextengine.VerifyCommand{
		Name:       "sleep",
		Executable: "sleep",
		Args:       []string{"10"},
		Timeout:    100 * time.Millisecond,
		WorkDir:    dir,
	})
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %v", err)
		}
		return
	}
	if result.ExitCode == 0 {
		t.Fatalf("expected killed/timed-out process, got exit 0 in %v", result.Duration)
	}
}

// Covers: L5-CTX-26
func TestBuiltinVerifyRunner_should_capture_nonzero_exit_code(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "exit42.py")
	if err := os.WriteFile(script, []byte("import sys\nsys.exit(42)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runner := contextengine.NewBuiltinVerifyRunner(dir)
	result, err := runner.Run(context.Background(), contextengine.VerifyCommand{
		Name:       "exit-42",
		Executable: "python3",
		Args:       []string{script},
		WorkDir:    dir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 42 {
		t.Fatalf("expected exit 42, got %d stderr=%q", result.ExitCode, result.Stderr)
	}
}

// Covers: L5-CTX-26
func TestBuiltinVerifyRunner_should_fail_for_missing_executable(t *testing.T) {
	dir := t.TempDir()
	runner := contextengine.NewBuiltinVerifyRunner(dir)
	_, err := runner.Run(context.Background(), contextengine.VerifyCommand{
		Name:       "missing",
		Executable: "definitely_not_a_real_binary_xyz_12345",
		Args:       []string{},
		WorkDir:    dir,
	})
	if err == nil {
		t.Fatal("expected missing executable error")
	}
}
