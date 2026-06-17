package surface

import "github.com/devrix/devrix/internal/shared/contracts"

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
	case "lsp":
		return true, false, false, false
	case "free_fork":
		return false, false, true, false
	case "query_diagnostics":
		return true, false, false, true
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
// Only long-run tools (free_fork, delegate_*) opt into InterruptCancel.
// Everything else (and unknown names) is InterruptBlock.
//
// TOOL-SURFACE-1-A01-F05 (DM-20260618-001): the surface MUST return this
// from InterruptBehavior and (for InterruptCancel) select on ctx.Done()
// inside Execute.
func InterruptBehaviorFor(toolName string) contracts.InterruptMode {
	switch toolName {
	case "free_fork":
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
