// Package contracts — ToolSurface v4 extension documentation.
//
// DSAFT: D2-S15-A02-T16 (DM-20260702-009 devrix-d2-tool-input-aware-concurrency-and-classifier).
//
// v3 ToolSurface (tool_surface.go) was static-bool: ToolSpec.ConcurrencySafe
// applied to every call of a tool. v4 adds two per-input methods directly to
// the existing interface:
//
//   - IsConcurrencySafe(input json.RawMessage) bool
//   - ToAutoClassifierInput(input json.RawMessage) string
//
// Go's interface model requires all methods of a named interface to be in a
// single type declaration; this file therefore does NOT declare a new
// interface. Instead, the v4 methods are appended to ToolSurface in
// tool_surface.go, and this file holds:
//
//   - The ToolSurfaceV4 type alias (see tool_surface.go) for readability at
//     partitionToolCalls (T18) call sites.
//   - Documentation of the v4 contract semantics — what 4 surfaces override
//     and what 15 default to.
//
// This split is intentional: tool_surface.go holds the structural
// definition; this file is the changelog / why-we-added-this entry. The
// D2 surface package's orthogonal_flags_v2.go holds the default impl.
//
// 4 surfaces override (real per-input logic):
//
//	bash       → isReadOnlyBashCommand (BashASTPolicy-extended check)
//	read_file  → 恒 true (read-only op, AC18 8K 回归锁 removed)
//	write_file → 恒 false (write op, must serialize)
//	edit_file  → per-input target path check (same → false, different → true)
//
// 15 surfaces default (1-line delegation to orthogonal_flags_v2.go):
//
//	tracker/query_diagnostics  → true
//	lsp/lsp_* (6 tools)        → false
//	free_fork/free_fork        → false
//	verify/verify_plan_execution → false
//	ask_user_question/ask_user_question → false
//	tool_search/tool_search    → true
//	plugin (dynamic)           → spec.ConcurrencySafe (v2 static)
//	delegate/delegate_* (5 tools) → false
package contracts
