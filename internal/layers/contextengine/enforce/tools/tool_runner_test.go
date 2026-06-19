package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
)

func mustBuiltinToolRunner(t *testing.T) tools.IToolRunner {
	runner, err := tools.NewBuiltinToolRunner()
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

// T: D2-S1-A01-T01
func TestBuiltinToolRunner_should_run_bash_pwd_when_workdir_set(t *testing.T) {
	dir := t.TempDir()
	runner := mustBuiltinToolRunner(t)
	ctx := tools.WithToolWorkDir(context.Background(), dir)

	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)

	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, tools.ToolCall{
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
	ctx := tools.WithToolWorkDir(context.Background(), dir)
	result, err := runner.Execute(ctx, tools.ToolCall{
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

// TestBuiltinToolRunner_BashSchemaDeclaresParameters reproduces the
// 2026-06-20 tool-call arg bleed: the bash tool was previously sent to the
// LLM without a Parameters JSON schema, so the model emitted
// `arguments: "{}"` for every bash call (visible in
// sess_1781908264924_6000.json — 4 successive bash calls with empty args,
// each rejected with "invalid command"). The fix adds Parameters so the
// LLM knows to send {"command": "..."}.
func TestBuiltinToolRunner_BashSchemaDeclaresParameters(t *testing.T) {
	reg, err := tools.NewBuiltinToolRegistry(nil)
	if err != nil {
		t.Fatalf("NewBuiltinToolRegistry: %v", err)
	}
	schemas, err := reg.ListTools(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var found bool
	for _, s := range schemas {
		if s.Name != "bash" {
			continue
		}
		found = true
		if s.Parameters == "" {
			t.Fatal("bash ToolSchema.Parameters is empty — LLM will see no schema and emit empty arguments")
		}
		var schema struct {
			Type       string         `json:"type"`
			Required   []string       `json:"required"`
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal([]byte(s.Parameters), &schema); err != nil {
			t.Fatalf("bash Parameters is not valid JSON: %v\n%s", err, s.Parameters)
		}
		if schema.Type != "object" {
			t.Errorf("bash schema type = %q, want %q", schema.Type, "object")
		}
		hasCommand := false
		for _, r := range schema.Required {
			if r == "command" {
				hasCommand = true
				break
			}
		}
		if !hasCommand {
			t.Errorf("bash schema required fields = %v, want \"command\" included", schema.Required)
		}
		if _, ok := schema.Properties["command"]; !ok {
			t.Errorf("bash schema missing properties.command: %#v", schema.Properties)
		}
	}
	if !found {
		t.Fatal("bash not present in BuiltinToolRegistry")
	}
}
