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

// --- ToolSpec v3 control plane metadata (D2-S15-A02-T08) -----------------
//
// DefaultV3MetadataFor returns the 6 control plane fields for the named
// tool. DSAFT: D2-S15-A02-T08 (19 tools explicit default metadata — the
// 治本 narrative MUST NOT defer to a Phase E migration).
//
// The returned tuple is the per-tool truth table for the 6 v3 fields.
// T14 (surface_metadata_gate_test.go) enforces that every registered
// surface's Tools() returns specs whose v3 fields are non-default
// (i.e., DefaultV3MetadataFor has been applied with the correct name).
//
// Naming convention:
//   read_file / grep / glob   → Probe + Bounded(15)  (H12 consensus:
//                              "re-read in self-loop recovery" is Probe)
//   write_file/edit_file/bash → Action + StateChangeRequired
//   lsp_*                     → Fact for read-only methods, Probe for
//                              workspace_symbol / code_action
//   free_fork                 → Experiment + Quotient(0.8)
//   delegate_*                → Probe + EvidenceRequired(min=1) + Bounded(3)
//   task_*                    → Action + Bounded(n) per tool
func DefaultV3MetadataFor(toolName string) (contracts.EmissionClass, contracts.ConvergenceContract, contracts.IterationBound, contracts.SourceUncertainty, int, string) {
	const (
		// Per-tool persistence thresholds (DM-20260702-008 / D2-S15-A02-T07).
		// Mirrors clawcode DEFAULT_MAX_RESULT_SIZE_CHARS = 50_000 +
		// per-tool overrides. We keep them per-tool because the LLM's
		// recovery style varies — Read re-reads via offset/limit so 8K is
		// fine, Bash output is re-issued so 30K is the sweet spot, etc.
		//
		// The growthbook override (persist.GetPersistenceThreshold) can
		// shift individual tools up or down at runtime without recompile.
		maxCharsReadFile        = 8 * 1024   // 8K  — Read re-reads via offset/limit
		maxCharsGrepGlob        = 20 * 1024  // 20K — match clawcode grep/glob
		maxCharsBash            = 30 * 1024  // 30K — bash output re-issued
		maxCharsEditWrite       = 100 * 1024 // 100K — Edit/Write/NotebookEdit/Web*/LSP/Agent/Task/Plan
		maxCharsMCPAuth         = 10 * 1024  // 10K — MCP auth responses
		maxCharsAskUserQuestion = 4 * 1024   // 4K  — small UX surface
		maxCharsToolSearch      = 4 * 1024   // 4K  — list-of-tools response
		maxCharsLSPRead         = 4 * 1024   // 4K  — go-to-def / hover / etc.
		maxCharsTaskStop        = 2 * 1024   // 2K  — control message
	)
	marker := contracts.DefaultTruncateMarkerText

	switch toolName {
	case "read_file":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// DM-20260702-008 / D2-S15-A02-T11: read_file is the recovery
			// path (offset/limit re-reads, T10). OpenEnded is correct
			// because the LLM uses Read to recover from oversized
			// results, NOT to discover content. The bound is preserved
			// as MaxN for dashboards but the channel no longer hard-rejects.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsReadFile, marker
	case "write_file":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "edit_file":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "bash":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 10},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsBash, marker
	case "grep":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// T11: OpenEnded — see read_file above.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsGrepGlob, marker
	case "glob":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			// T11: OpenEnded — see read_file above.
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsGrepGlob, marker
	case "query_diagnostics":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "verify_plan_execution":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "ask_user_question":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 2},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsAskUserQuestion, marker
	case "tool_search":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsToolSearch, marker
	case "lsp_go_to_definition", "lsp_find_references", "lsp_incoming_calls", "lsp_hover":
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsLSPRead, marker
	case "lsp_workspace_symbol":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsLSPRead, marker
	case "lsp_code_action":
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsLSPRead, marker
	case "free_fork":
		return contracts.EC_Experiment,
			contracts.ConvergenceContract{Kind: contracts.CC_QuotientThreshold, Threshold: 0.8},
			contracts.IterationBound{Kind: contracts.IB_Quotient, Quotient: 0.8},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsEditWrite, marker
	case "task_output":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 5},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "task_stop":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_StateChangeRequired},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 1},
			contracts.SourceUncertainty{Source: contracts.SK_User, Value: 0.85},
			maxCharsTaskStop, marker
	case "task_list_background":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	case "task_output_background":
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	}

	// Pattern-based fallbacks (delegate_*, task_*, lsp_*).
	if hasPrefix(toolName, "delegate_") {
		return contracts.EC_Probe,
			contracts.ConvergenceContract{Kind: contracts.CC_EvidenceRequired, MinEvidence: 1},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_LLM, Value: 0.4},
			maxCharsEditWrite, marker
	}
	if hasPrefix(toolName, "task_") {
		return contracts.EC_Action,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_Bounded, MaxN: 3},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsEditWrite, marker
	}
	if hasPrefix(toolName, "lsp_") {
		return contracts.EC_Fact,
			contracts.ConvergenceContract{Kind: contracts.CC_None},
			contracts.IterationBound{Kind: contracts.IB_OpenEnded},
			contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 1.0},
			maxCharsLSPRead, marker
	}

	// Unknown tool name — T14 gate will fail the build. Returning the
	// zero defaults lets a fresh ToolSpec compile cleanly; the gate
	// prevents any registered surface from hitting this path.
	return contracts.EC_Action,
		contracts.ConvergenceContract{Kind: contracts.CC_None},
		contracts.IterationBound{Kind: contracts.IB_OpenEnded},
		contracts.SourceUncertainty{Source: contracts.SK_Deterministic, Value: 0.0},
		0, ""
}

// ApplyV3Metadata fills the 6 ToolSpec v3 control plane fields on the
// given spec from DefaultV3MetadataFor. Surface implementations call
// this once per tool after constructing the v2 9-field spec.
//
// DSAFT: D2-S15-A02-T08 (truth table) + T09/T10/T11 (surface call sites)
// + T14 (gate test forbids any registered spec from skipping this call).
func ApplyV3Metadata(spec *contracts.ToolSpec, toolName string) {
	ec, cc, ib, su, max, marker := DefaultV3MetadataFor(toolName)
	spec.EmissionClass = ec
	spec.ConvergenceContract = cc
	spec.IterationBound = ib
	spec.SourceUncertainty = su
	spec.MaxResultSizeChars = max
	spec.TruncateMarkerText = marker
}
