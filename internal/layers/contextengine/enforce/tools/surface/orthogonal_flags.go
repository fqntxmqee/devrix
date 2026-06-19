package surface

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// OrthogonalFlags is the 4-bool truth table for each tool name in devrix
// (TOOL-SURFACE-1-A01-F02 — DM-20260618-001 devrix-tool-spec-enrichment).
//
// Each entry is the authoritative classification that the corresponding
// surface MUST apply to its ToolSpec. The hard-coded table is intentional:
// it keeps the surface code mechanical and the S3-Gate review a 1:1
// diff against design.md §2.1.2.
//
// Truth table (column = flag, row = tool name):
//
//	tool_name    | ReadOnly | Destructive | OpenWorld | ConcurrencySafe
//	-------------+----------+-------------+-----------+----------------
//	read_file    |    Y     |      N      |     N     |       Y
//	write_file   |    N     |      Y      |     N     |       N
//	edit_file    |    N     |      Y      |     N     |       N
//	bash         |    N     |      Y      |     N     |       Y
//	grep         |    Y     |      N      |     N     |       Y
//	glob         |    Y     |      N      |     N     |       Y
//	lsp          |    Y     |      N      |     N     |       N
//	free_fork    |    N     |      N      |     Y     |       N
//	query_diagnostics | Y   |      N      |     N     |       Y
//	verify_plan_execution | Y |    N      |     N     |       N
//	delegate_*   |    N     |      N      |     Y     |       N
//	task_output  |    Y     |      N      |     N     |       Y
//	ask_user_question | Y  |      N      |     Y     |       N
//
// Tools not in the table get all-false (the conservative default; the
// surface MUST still emit a non-zero bool combination, so callers SHOULD
// extend this map when adding new tools).
type orthogonalFlags = contracts.ToolSpec // alias for the 4 bools

// OrthogonalFlagFor returns the 4 bool flags for a given tool name.
// Falls back to all-false for unknown names; this matches the design's
// "conservative default" rule (T22 assertion: at least one bool must be true).
func OrthogonalFlagFor(toolName string) (readOnly, destructive, openWorld, concurrencySafe bool) {
	switch toolName {
	case "read_file":
		return true, false, false, true
	case "write_file":
		return false, true, false, false
	case "edit_file":
		return false, true, false, false
	case "bash":
		return false, true, false, true
	case "grep":
		return true, false, false, true
	case "glob":
		return true, false, false, true
	case "lsp",
		"lsp_go_to_definition",
		"lsp_find_references",
		"lsp_incoming_calls",
		"lsp_hover",
		"lsp_workspace_symbol":
		return true, false, false, false
	case "free_fork":
		return false, false, true, false
	case "query_diagnostics":
		return true, false, false, true
	case "ask_user_question":
		return true, false, true, false
	case "verify_plan_execution":
		return true, false, false, false
	}
	// delegate_*, task_output, etc. follow a "spawn-agents" / "read-result"
	// rule below.
	switch {
	case hasPrefix(toolName, "delegate_"):
		return false, false, true, false
	case toolName == "task_output" || hasPrefix(toolName, "task_"):
		return true, false, false, true
	}
	return false, false, false, false
}

// InterruptBehaviorFor returns the InterruptMode for a given tool name.
// Only long-run tools (free_fork, delegate_*, ask_user_question) opt
// into InterruptCancel. Everything else (and unknown names) is
// InterruptBlock.
//
// TOOL-SURFACE-1-A01-F05 (DM-20260618-001): the surface MUST return this
// from InterruptBehavior and (for InterruptCancel) select on ctx.Done()
// inside Execute.
//
// ask_user_question (DM-20260618-006) opts into InterruptCancel so the
// D7 runLoop can abort a pending question when the user issues a new
// message mid-turn.
func InterruptBehaviorFor(toolName string) contracts.InterruptMode {
	switch toolName {
	case "free_fork", "ask_user_question":
		return contracts.InterruptCancel
	}
	if hasPrefix(toolName, "delegate_") {
		return contracts.InterruptCancel
	}
	return contracts.InterruptBlock
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// ShouldDeferByDefault returns true for tools whose full schema is omitted
// from the default system prompt and must be retrieved on demand via
// tool_search. The 6 hardcoded candidates are:
//   - delegate_* (5: delegate_explore / delegate_status / delegate_status_all
//     / delegate_plan / delegate_research) — spawns child agent, rarely
//     invoked outside plan-mode finalization.
//   - task_output_background (1: suffix match) — polling helper, low value.
//
// tool_search itself MUST always return false (otherwise deadlock).
//
// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003 devrix-surface-lazy-loading).
func ShouldDeferByDefault(toolName string) bool {
	if toolName == "tool_search" {
		return false
	}
	if hasPrefix(toolName, "delegate_") {
		return true
	}
	if hasPrefix(toolName, "task_") && toolName == "task_output_background" {
		return true
	}
	// Also catch `*_background` suffix generally (defensive for future tools).
	if len(toolName) > len("_background") &&
		toolName[len(toolName)-len("_background"):] == "_background" {
		return true
	}
	return false
}

// AllowAllCheckPermission is the default CheckPermission implementation
// for surfaces without per-tool policy. It returns DecisionAllow
// unconditionally. ToolSurface implementations can embed
// allowAllChecker to satisfy the interface with one line.
//
// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002 — see PR #68 for full
// integration; here we provide the helper so every surface compiles
// under the ToolSurface v2 contract).
type allowAllChecker struct{}

func (allowAllChecker) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
