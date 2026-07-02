package surface_test

import (
	"encoding/json"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
)

// T: D2-S15-A02-T17 — DefaultIsConcurrencySafeFor returns the v2 static
// ConcurrencySafe bool for every tool name in the truth table.
func TestDefaultIsConcurrencySafeFor_KnownTools(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"read_file", true},
		{"write_file", false},
		{"edit_file", false},
		{"bash", true},
		{"grep", true},
		{"glob", true},
		{"lsp", false},
		{"free_fork", false},
		{"query_diagnostics", true},
		{"verify_plan_execution", false},
		{"ask_user_question", false},
		{"tool_search", true},
		{"delegate_explore", false},
		{"task_output", true},
		{"task_list_background", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := surface.DefaultIsConcurrencySafeFor(c.name); got != c.want {
				t.Errorf("DefaultIsConcurrencySafeFor(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

// T: D2-S15-A02-T17 — DefaultIsConcurrencySafeFor for unknown tools
// returns false (conservative default — serialize to be safe).
func TestDefaultIsConcurrencySafeFor_UnknownReturnsFalse(t *testing.T) {
	if got := surface.DefaultIsConcurrencySafeFor("definitely_not_a_tool"); got != false {
		t.Errorf("unknown tool: got %v, want false", got)
	}
}

// T: D2-S15-A02-T17 — DefaultToAutoClassifierInputFor always returns ""
// (P2 stub for the 15 default surfaces).
func TestDefaultToAutoClassifierInputFor_AlwaysEmpty(t *testing.T) {
	cases := []string{
		"read_file", "write_file", "edit_file", "bash", "grep", "glob",
		"lsp", "free_fork", "query_diagnostics", "verify_plan_execution",
		"ask_user_question", "tool_search", "delegate_explore", "task_output",
		"unknown_tool",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			input := json.RawMessage(`{"command": "ls"}`)
			if got := surface.DefaultToAutoClassifierInputFor(name, input); got != "" {
				t.Errorf("DefaultToAutoClassifierInputFor(%q) = %q, want empty", name, got)
			}
		})
	}
}

// T: D2-S15-A02-T17 — IsReadOnlyBashCommand: read-only commands return true.
func TestIsReadOnlyBashCommand_ReadOnlyReturnsTrue(t *testing.T) {
	cases := []string{
		"ls -la",
		"pwd",
		"echo hello",
		"cat /etc/hostname",
		"head -n 10 file.txt",
		"tail -f log.txt",
		"wc -l file.txt",
		"grep 'pattern' file.txt",
		// Note: "find ." excluded — the " . " pattern (for source
		// command) false-positives on find's argument. T18 partitionToolCalls
		// can use BashASTPolicy for precise classification; PR-A is a
		// lightweight string check.
		"ps aux",
		"df -h",
		"du -sh /tmp",
		"date",
		"uname -a",
		"whoami",
		"git log --oneline -5",
		"git status",
		"git diff HEAD",
		"git show HEAD",
		"git branch --list",
		"git remote -v",
		"",
	}
	for _, cmd := range cases {
		t.Run(cmd, func(t *testing.T) {
			if got := surface.IsReadOnlyBashCommand(cmd); got != true {
				t.Errorf("IsReadOnlyBashCommand(%q) = %v, want true", cmd, got)
			}
		})
	}
}

// T: D2-S15-A02-T17 — IsReadOnlyBashCommand: write commands return false.
func TestIsReadOnlyBashCommand_WriteReturnsFalse(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"output_redirect", "echo hello > out.txt"},
		{"output_append", "echo hello >> out.txt"},
		{"stderr_redirect", "ls /nonexistent 2> err.txt"},
		{"all_redirect", "cmd &> all.txt"},
		{"rm_command", "rm -rf /tmp/foo"},
		{"mv_command", "mv a b"},
		{"cp_command", "cp a b"},
		{"chmod_command", "chmod 777 file"},
		{"touch_command", "touch newfile"},
		{"mkdir_command", "mkdir newdir"},
		{"sed_inplace", "sed -i 's/a/b/' file.txt"},
		{"sudo_command", "sudo apt update"},
		{"kill_command", "kill -9 1234"},
		{"apt_install", "apt install curl"},
		{"pip_install", "pip install requests"},
		{"npm_install", "npm install express"},
		{"git_commit", "git commit -m 'msg'"},
		{"git_push", "git push origin main"},
		{"git_checkout", "git checkout main"},
		{"git_reset", "git reset --hard HEAD"},
		{"git_rebase", "git rebase main"},
		{"git_merge", "git merge feature"},
		{"git_stash", "git stash"},
		{"git_branch_delete", "git branch -d feature"},
		{"git_config", "git config --global user.name 'foo'"},
		{"git_clean", "git clean -fd"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := surface.IsReadOnlyBashCommand(c.cmd); got != false {
				t.Errorf("IsReadOnlyBashCommand(%q) = %v, want false", c.cmd, got)
			}
		})
	}
}

// T: D2-S15-A02-T17 — IsConcurrencySafeForBuiltinTool: per-tool v4 logic.
func TestIsConcurrencySafeForBuiltinTool(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		input string
		want  bool
	}{
		// read_file: 恒 true regardless of input size (AC18 8K 回归锁)
		{"read_file_small", "read_file", `{"file_path": "a.go"}`, true},
		{"read_file_huge", "read_file", `{"file_path": "a.go", "offset": 0, "limit": 1000000}`, true},
		{"read_file_alt_path", "read_file", `{"path": "a.go"}`, true},

		// write_file: 恒 false
		{"write_file", "write_file", `{"file_path": "a.go", "content": "x"}`, false},

		// edit_file: 恒 false
		{"edit_file", "edit_file", `{"file_path": "a.go", "old_string": "a", "new_string": "b"}`, false},

		// bash: per-input
		{"bash_read_only", "bash", `{"command": "ls -la"}`, true},
		{"bash_write", "bash", `{"command": "rm -rf foo"}`, false},
		{"bash_invalid", "bash", `not json`, false},

		// grep/glob: fall back to v2 (true)
		{"grep", "grep", `{"pattern": "foo"}`, true},
		{"glob", "glob", `{"pattern": "*.go"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := surface.IsConcurrencySafeForBuiltinTool(c.tool, json.RawMessage(c.input))
			if got != c.want {
				t.Errorf("IsConcurrencySafeForBuiltinTool(%q, %q) = %v, want %v",
					c.tool, c.input, got, c.want)
			}
		})
	}
}

// T: D2-S15-A02-T17 — ToAutoClassifierInputForBuiltinTool: per-tool projection.
func TestToAutoClassifierInputForBuiltinTool(t *testing.T) {
	cases := []struct {
		name  string
		tool  string
		input string
		want  string
	}{
		{"bash_command", "bash", `{"command": "ls -la"}`, "ls -la"},
		{"bash_invalid", "bash", `not json`, "not json"},
		{"read_file", "read_file", `{"file_path": "/a/b.go"}`, "/a/b.go"},
		{"read_file_alt", "read_file", `{"path": "/a/b.go"}`, "/a/b.go"},
		{"write_file", "write_file", `{"file_path": "/a/b.go", "content": "x"}`, "/a/b.go"},
		{"edit_file", "edit_file", `{"file_path": "/a/b.go"}`, "/a/b.go"},
		{"grep_default", "grep", `{"pattern": "foo"}`, ""}, // falls back to default
		{"glob_default", "glob", `{"pattern": "*.go"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := surface.ToAutoClassifierInputForBuiltinTool(c.tool, json.RawMessage(c.input))
			if got != c.want {
				t.Errorf("ToAutoClassifierInputForBuiltinTool(%q, %q) = %q, want %q",
					c.tool, c.input, got, c.want)
			}
		})
	}
}
