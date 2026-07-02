package contracts

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/shared/types"
)

// ToolSpec is a neutral LLM tool schema (decoupled from D3 llmgateway.ToolCall
// and D2 tools.ToolSchema). All cross-layer tool exchanges use ToolSpec.
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007 devrix-tool-surface-contract)
// TOOL-SURFACE-1-A01-F02 (DM-20260618-001 devrix-tool-spec-enrichment):
//
//	4 orthogonal bool flags supplement the legacy Risk enum so that
//	PerAgentFilter / PerRiskFilter / turn_adapter can make fine-grained
//	decisions without parsing Risk strings.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  string // JSON Schema
	Risk        types.RiskLevel

	// ReadOnly: tool does not modify the filesystem (read_file / glob / grep / lsp / verify).
	// PerAgentFilter consumes this to auto-extend the explore agent's visible set.
	ReadOnly bool

	// Destructive: tool performs irreversible operations (rm / force_push / delete_branch).
	// PerRiskFilter in plan_mode MAY consult this together with OpenWorld to decide
	// whether the LLM can call the tool without human confirmation.
	Destructive bool

	// OpenWorld: tool's side effects extend beyond the local machine
	// (web_fetch / send_im_message / free_fork spawning child agents).
	// PerRiskFilter uses this in plan_mode to drop the tool from the visible set.
	OpenWorld bool

	// ConcurrencySafe: multiple invocations of the same tool may run in parallel
	// without mutual interference (e.g. read_file on different paths).
	// turn_adapter.ExecuteRound uses this to decide parallel vs sequential dispatch.
	ConcurrencySafe bool

	// DeferLoading marks tools whose full schema is not sent to the LLM on
	// every turn. turn_adapter.Prepare filters these out of the system
	// prompt; the LLM must call tool_search to retrieve the schema on
	// demand. Empty / unused tools (delegate_*, *_background) get this
	// flag at BuildSurfaces time. Runtime ToolFilter.ShouldDefer can also
	// add it (e.g. plan_mode → defer all open-world tools).
	//
	// DSAFT: TOOL-SURFACE-1-A01-F08 (DM-20260618-003 devrix-surface-lazy-loading).
	DeferLoading bool

	// --- ToolSpec v3 (D2-S15-A02-T06): 6 control plane fields ---
	// DSAFT: D2-S15-A02-T06 — control plane; runtime-bound; defaults in tool_surface_v3.go.
	EmissionClass       EmissionClass       `json:"emission_class"`
	ConvergenceContract ConvergenceContract `json:"convergence_contract"`
	IterationBound      IterationBound      `json:"iteration_bound"`
	SourceUncertainty   SourceUncertainty   `json:"source_uncertainty"`
	MaxResultSizeChars  int                 `json:"max_result_size_chars"`
	TruncateMarkerText  string              `json:"truncate_marker_text"`
}

// ToolResult is the return type of ToolSurface.Execute.
//
// DSAFT: TOOL-SURFACE-1-A01-F04
type ToolResult struct {
	Output string
	Error  string
}

// InterruptMode describes how a tool responds to a context cancellation signal.
//
// DSAFT: TOOL-SURFACE-1-A01-F05 (DM-20260618-001 devrix-tool-spec-enrichment).
// The 1:1 mapping with clawcode Tool.interruptBehavior (Tool.ts:410-416)
// lets long-run tools opt out of waiting for natural completion when the
// user issues a new message mid-turn.
type InterruptMode string

const (
	// InterruptCancel: the surface MUST select on ctx.Done() and return
	// ctx.Err() within 200ms of cancellation.
	InterruptCancel InterruptMode = "cancel"

	// InterruptBlock: the surface ignores ctx cancellation and runs to
	// natural completion. The default for short-run tools.
	InterruptBlock InterruptMode = "block"
)

// ToolSurface is a discoverable entry point for a group of related tools.
//
// Per devrix Facet Decomposition (DM-020 D-c + architecture-design.md §1.1),
// ToolSurface is a 拆面 contract exposed to D2 (consumer) by D2 surface
// implementations. Library packages (freefork / tracker / verify / etc.) do
// not depend on this contract — the dependency direction is:
//
//	contracts ← surface (in tools/surface) ← library
//
// Design principles:
//   - Accept interfaces, return structs (ToolSpec / ToolResult are structs)
//   - 6 methods, each 1-3 lines in typical implementations
//   - Does not hold ctx; Execute / Tools accept ctx
//   - Does NOT make permission decisions (IPermissionGate runs in
//     turn_adapter.ExecuteRound, BEFORE surf.Execute)
//
// DSAFT: TOOL-SURFACE-1-A01 (DM-20260617-007) + TOOL-SURFACE-1-A01-F05
// (DM-20260618-001 — InterruptBehavior addition) +
// TOOL-SURFACE-1-A01-F07 (DM-20260618-002 — CheckPermission addition).
type ToolSurface interface {
	// Name returns the surface identifier (used in devrix.yaml config,
	// log tags, and `devrix tool list` output).
	Name() string

	// Tools returns the list of tools this surface exposes for the given
	// (workDir, sessionID) context. Implementations may filter
	// conditionally (e.g. LSPToolSurface checks lsp.enabled).
	//
	// The returned slice should be deterministic for stable LLM tool
	// schema hashing (callers may cache it per session).
	Tools(ctx context.Context, workDir, sessionID string) []ToolSpec

	// RiskLevel returns the RiskLevel for a single tool name. Unknown
	// names return types.RiskLevelLow (defensive default).
	//
	// Called by turn_adapter.ExecuteRound to populate
	// IPermissionGate.Request's risk argument.
	RiskLevel(name string) types.RiskLevel

	// Execute dispatches a single tool call through the surface's
	// internal mechanism. Returns ToolResult{Output, Error}; non-empty
	// Error means the caller should not block.
	//
	// workDir and sessionID are passed explicitly (not via ctx value) so
	// surfaces do not need to know about D1/D2 ctx conventions.
	Execute(ctx context.Context, name, input, workDir string) (*ToolResult, error)

	// InterruptBehavior returns the interrupt mode for the named tool.
	// Long-run tools (FreeForkSurface.free_fork) MUST return InterruptCancel
	// and select on ctx.Done() inside Execute; everything else returns
	// InterruptBlock by convention.
	//
	// The default is InterruptBlock (existing 7 surfaces); only surfaces
	// that genuinely run >5s in normal use override this.
	InterruptBehavior(name string) InterruptMode

	// CheckPermission is the per-tool pre-dispatch hook. turn_adapter
	// calls it BEFORE Execute; a non-Allow decision skips Execute and
	// the LLM gets a PermissionDeniedError / PermissionAskRequiredError
	// envelope in result.Results[i].Error.
	//
	// 5 surfaces return Allow unconditionally (read-only / stateless
	// tools). 2 surfaces override:
	//   - BuiltinSurface  → BashASTPolicy parses the command and
	//     denies rm -rf /, dd, mkfs, sudo, chmod 777 /.
	//   - FreeForkSurface → delegates to IPermissionGate.CheckPermission
	//     (multi-agent spawns need the global policy).
	//
	// Performance budget: < 5ms p99 (BashASTPolicy is the hot path).
	// DSAFT: TOOL-SURFACE-1-A01-F07 (DM-20260618-002).
	CheckPermission(ctx context.Context, spec ToolSpec, input json.RawMessage) Decision

	// --- v4: per-input concurrency decision + auto-classifier projection ---
	//
	// DSAFT: D2-S15-A02-T16 (DM-20260702-009 devrix-d2-tool-input-aware-concurrency-and-classifier).
	//
	// v3 was static-bool: ToolSpec.ConcurrencySafe is decided at BuildSurfaces
	// time and applies to every call of that tool (治标 — `bash` was always
	// sequential even for `ls`; `read_file` was always parallel even on the
	// same path). v4 upgrades both decisions to per-input so that partitionToolCalls
	// (T18) can build safe/unsafe batches that respect actual data dependencies.
	//
	// D5 decision: json.RawMessage (跟 CheckPermission 对齐) — type cohesion
	// over YAGNI extension. D7 decision: ClassifierResult naming (P2 stub).
	//
	// 4 surfaces override (BuiltinSurface handles bash + read_file + write_file
	// + edit_file with real per-input logic). 15 surfaces fall back to the v2
	// static bool via orthogonal_flags_v2.go's default helpers.

	// IsConcurrencySafe returns whether this specific input can be
	// executed concurrently with other IsConcurrencySafe=true calls of
	// the same tool without mutual interference.
	//
	// Semantics:
	//   - true  → safe to batch with other true calls; partitionToolCalls
	//             groups these into the same errgroup.
	//   - false → must run sequentially (own batch).
	//
	// Fail-safe: returns false on parse failure, NEVER panics. On parse
	// error the surface SHOULD emit metric auto_mode.malformed_tool_input
	// (P2 stub metric; production metric activates when classifier goes P1).
	//
	// Per-tool semantics:
	//   - bash       → isReadOnlyBashCommand (BashASTPolicy extended)
	//   - read_file  → 恒 true (read-only op, AC18 8K 回归锁 removed in 6a6b9add)
	//   - write_file → 恒 false (write op, must serialize)
	//   - edit_file  → per-input target path check (same path → false,
	//                  different paths → true)
	//   - 15 default → v2 static ToolSpec.ConcurrencySafe
	IsConcurrencySafe(input json.RawMessage) bool

	// ToAutoClassifierInput projects a tool call into the compact string
	// the auto-mode classifier (D7 P2 stub — AutoModeClassifier.Classify)
	// consumes in its transcript. Returns "" to skip the call.
	//
	// Examples: bash("ls -la") → "ls -la"; read_file("foo.go") → "foo.go";
	// ask_user_question → "" (skip; not security-relevant).
	//
	// Fail-safe: on parse failure, return raw input + emit metric
	// auto_mode.malformed_tool_input. NEVER panics.
	ToAutoClassifierInput(input json.RawMessage) string
}

// ToolSurfaceV4 is a type alias for the v4-extended ToolSurface interface.
// The v4 extension adds IsConcurrencySafe + ToAutoClassifierInput directly
// to the v3 ToolSurface (no separate interface — Go interfaces can't be
// split across files). This alias exists for documentation / type-assertion
// readability at partitionToolCalls (T18) call sites: callers can write
// `var v4 contracts.ToolSurfaceV4 = surface` instead of commenting "this
// surface must implement v4 methods". The assertion is always true because
// ToolSurface == ToolSurfaceV4 after this change.
//
// DSAFT: D2-S15-A02-T16 (DM-20260702-009).
type ToolSurfaceV4 = ToolSurface
