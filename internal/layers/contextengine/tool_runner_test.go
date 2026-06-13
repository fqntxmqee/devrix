package contextengine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine"
)

func mustBuiltinToolRunner(t *testing.T) contextengine.IToolRunner {
	runner, err := contextengine.NewBuiltinToolRunner()
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

// T: D2-S1-A01-T01
func TestBuiltinToolRunner_should_run_bash_pwd_when_workdir_set(t *testing.T) {
	dir := t.TempDir()
	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)

	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "bash",
		Input: `{"command":"pwd"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s output=%q", result.Error, result.Output)
	}
	got := strings.TrimSpace(result.Output)
	if got != dir {
		t.Fatalf("pwd output = %q, want %q", got, dir)
	}
}

func TestBuiltinToolRunner_should_read_file_in_workspace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello devrix"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "read_file",
		Input: `{"path":"hello.txt"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Output != "hello devrix" {
		t.Fatalf("output = %q", result.Output)
	}
}

func TestBuiltinToolRunner_should_reject_path_escape(t *testing.T) {
	dir := t.TempDir()
	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "read_file",
		Input: `{"path":"../../etc/passwd"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected path escape error")
	}
}

// T: D2-S8-A01-T01
func TestBuiltinToolRunner_should_reject_disallowed_bash_command(t *testing.T) {
	dir := t.TempDir()
	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)

	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "bash",
		Input: `{"command":"shutdown -h now"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected sandbox rejection")
	}
	if !strings.Contains(result.Error, "dangerous command pattern") {
		t.Fatalf("error = %q", result.Error)
	}
}

// T: D2-S8-A01-T01
func TestBuiltinToolRunner_should_accept_bash_with_workdir_absolute_path(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "note.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "bash",
		Input: `{"command":"cat ` + filepath.Join(dir, "pkg", "note.txt") + `"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s output=%q", result.Error, result.Output)
	}
	if strings.TrimSpace(result.Output) != "ok" {
		t.Fatalf("output = %q", result.Output)
	}
}

func TestBuiltinToolRunner_should_read_file_with_file_path_alias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("alias ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "read_file",
		Input: `{"file_path":"hello.txt"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Output != "alias ok" {
		t.Fatalf("output = %q", result.Output)
	}
}

func TestBuiltinToolRunner_should_write_file_in_workspace(t *testing.T) {
	dir := t.TempDir()
	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "write_file",
		Input: `{"path":"out.txt","content":"written"}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "written" {
		t.Fatalf("file content = %q", data)
	}
}

func TestBuiltinToolRunner_should_redirect_glob_json_from_bash(t *testing.T) {
	dir := t.TempDir()
	runner := mustBuiltinToolRunner(t)
	ctx := contextengine.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, contextengine.ToolCall{
		Name:  "bash",
		Input: `{"pattern":"**/*.go","path":"."}`,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected redirect hint")
	}
	if !strings.Contains(result.Error, "glob tool") {
		t.Fatalf("error = %q", result.Error)
	}
}
