package surface

import (
	"encoding/json"
	"strings"
)

// ToolSurface v4 — per-input default helpers for the 15 tools that don't
// override IsConcurrencySafe / ToAutoClassifierInput with real per-input logic.
//
// DSAFT: D2-S15-A02-T17 (DM-20260702-009 devrix-d2-tool-input-aware-concurrency-and-classifier).
//
// 4 surfaces override (real per-input logic, see builtin_surface.go):
//
//	bash       → IsReadOnlyBashCommand (保守写检测)
//	read_file  → 恒 true (read-only op, AC18 8K 回归锁 removed)
//	write_file → 恒 false (write op, must serialize)
//	edit_file  → 恒 false (同 target 路径 false, batch 内 path-merge 由 T18 partitionToolCalls 处理)
//
// 15 surfaces default (call DefaultIsConcurrencySafeFor / DefaultToAutoClassifierInputFor):
//
//	tracker/query_diagnostics  → true / ""
//	lsp/lsp_* (6 tools)        → false / ""
//	free_fork/free_fork        → false / ""
//	verify/verify_plan_execution → false / ""
//	ask_user_question/ask_user_question → false / ""
//	tool_search/tool_search    → true / ""
//	plugin (dynamic)           → spec.ConcurrencySafe / ""
//	delegate/delegate_* (5 tools) → false / ""
//
// Why per-input helpers exist (instead of hardcoding in each surface):
//   - PluginSurface has dynamic tool list, can't hardcode the v2 bool.
//   - Tests for the v2 fallback are written ONCE in orthogonal_flags_v2_test.go.
//   - Future GB override (T25' M1 复用 PERSIST) is one site to extend.

// DefaultIsConcurrencySafeFor returns the v2 static ConcurrencySafe bool for
// the named tool, which is the conservative fallback for tools without
// per-input logic. Returns false for unknown tool names (保守默认).
//
// DSAFT: D2-S15-A02-T17. Mirrors OrthogonalFlagFor concurrency column.
func DefaultIsConcurrencySafeFor(toolName string) bool {
	_, _, _, cs := OrthogonalFlagFor(toolName)
	return cs
}

// DefaultToAutoClassifierInputFor returns the auto-classifier projection for
// the named tool. For the 15 default surfaces, the classifier transcript
// skips these (P2 stub 不需要逐 tool 投影, 升 P1 时再实现 per-tool 投影)。
//
// Always returns "" for default surfaces. Override surfaces (bash, read_file,
// write_file, edit_file) implement their own projection in BuiltinSurface.
//
// DSAFT: D2-S15-A02-T17.
func DefaultToAutoClassifierInputFor(toolName string, _ json.RawMessage) string {
	return ""
}

// IsReadOnlyBashCommand returns true when the bash command string is safe
// to run concurrently with other IsConcurrencySafe=true calls of bash.
//
// Approach: deny-list of write patterns. Default is read-only; if any
// write pattern matches, return false. Conservative (false-positive safe —
// if unsure, treat as write and serialize).
//
// Write patterns detected:
//   - Output redirects:  > , >> , 2> , &> , 2>&1 (last alone is not a write)
//   - Destructive commands: rm, mv, cp, chmod, chown, touch, mkdir, rmdir,
//     ln, truncate, dd, mkfs, sudo, kill, killall, pkill
//   - Stream editors:    sed -i / sed -i.backup, awk ... > FILE
//   - Process spawns:    exec , source / .   (shell state mutation)
//   - Package ops:       apt , yum , dnf , brew , pip install/uninstall
//   - Git writes:        git commit, push, checkout, reset, rebase, merge,
//     stash, tag, branch -d, remote add
//
// Read-only commands recognized (return true unconditionally):
//   - echo, printf, pwd, ls, cat, head, tail, wc, file, stat, find,
//     locate, which, type, whereis, env, printenv, whoami, hostname,
//     date, uname, df, du, free, top, ps, jobs, history, alias,
//     shopt, set (without -o/+o mutating)
//
// DSAFT: D2-S15-A02-T17. Per-input impl of bash IsConcurrencySafe.
//
// NOT a full bash parser — that's what BashASTPolicy does for deny rules.
// This is a lightweight string check for the concurrency decision.
func IsReadOnlyBashCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	// Write patterns. Order matters — check redirects first (they can
	// apply to any command including the "safe" ones below).
	writePatterns := []string{
		// Output redirects (2>&1 alone is not a write; check for `>` first)
		">>", " > ", "> ", "2> ", "&> ",
		// File mutators
		" rm ", " mv ", " cp ", " chmod ", " chown ", " touch ",
		" mkdir ", " rmdir ", " ln ", " truncate ", " dd ", " mkfs ",
		" sudo ", " kill ", " killall ", " pkill ",
		// Stream editors with file writes
		"sed -i", "sed -i.",
		// Shell mutation
		" exec ", " source ", " . ",
		// Package ops
		" apt ", " apt-get ", " yum ", " dnf ", " brew ",
		" pip install", " pip uninstall", " npm install", " npm uninstall",
		" yarn add", " yarn remove",
		// Git writes
		"git commit", "git push", "git checkout", "git reset",
		"git rebase", "git merge", "git stash", "git tag",
		"git branch -d", "git branch -D", "git remote add",
		"git config", "git clean",
	}
	lower := " " + strings.ToLower(cmd) + " "
	for _, pat := range writePatterns {
		if strings.Contains(lower, pat) {
			return false
		}
	}

	return true
}

// IsConcurrencySafeForBuiltinTool is the per-input v4 implementation for
// the 4 builtin tools that override (bash, read_file, write_file, edit_file).
// All other builtin tools (grep, glob) fall back to DefaultIsConcurrencySafeFor.
//
// DSAFT: D2-S15-A02-T17.
func IsConcurrencySafeForBuiltinTool(toolName string, input json.RawMessage) bool {
	switch toolName {
	case "read_file":
		// AC18: 8K anti-pattern regression lock — read_file is concurrency
		// safe REGARDLESS of input size. Removed size-based decision in
		// commit 6a6b9add; preserved here as a permanent regression lock.
		return true
	case "write_file":
		// Write op, must serialize.
		return false
	case "edit_file":
		// Per design.md: 同 target 路径 false. The IsConcurrencySafe
		// signature is per-call (no batch context), so we conservatively
		// return false. Batch-level path-merge is T18 partitionToolCalls
		// responsibility (PR-B).
		return false
	case "bash":
		// Per-input bash read-only detection.
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return false // fail-safe: parse error → sequential
		}
		return IsReadOnlyBashCommand(in.Command)
	}
	// grep, glob: read-only static, fall back to v2.
	return DefaultIsConcurrencySafeFor(toolName)
}

// ToAutoClassifierInputForBuiltinTool is the per-input v4 implementation
// for the 4 builtin tools that override. All other builtin tools (grep,
// glob) fall back to DefaultToAutoClassifierInputFor (returns "").
//
// DSAFT: D2-S15-A02-T17.
func ToAutoClassifierInputForBuiltinTool(toolName string, input json.RawMessage) string {
	switch toolName {
	case "bash":
		// Project the command string for the classifier transcript.
		// Fail-safe: return raw input on parse error + emit metric.
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return string(input)
		}
		return in.Command
	case "read_file":
		var in struct {
			FilePath string `json:"file_path"`
			Path     string `json:"path"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return string(input)
		}
		if in.FilePath != "" {
			return in.FilePath
		}
		return in.Path
	case "write_file", "edit_file":
		var in struct {
			FilePath string `json:"file_path"`
			Path     string `json:"path"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return string(input)
		}
		if in.FilePath != "" {
			return in.FilePath
		}
		return in.Path
	}
	// grep, glob: P2 stub 不需要, 走 default ("").
	return DefaultToAutoClassifierInputFor(toolName, input)
}
