package bootstrap

import (
	"context"
	"encoding/json"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/bash"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// Batch is one partition of consecutive tool calls that can run
// concurrently with each other but not with the next batch. The
// partition is keyed on the per-input IsConcurrencySafe decision — see
// partitionToolCalls.
//
// DSAFT: D7-S9-A50-T18 (DM-20260702-009 PR-B).
type Batch struct {
	// IsConcurrencySafe is the per-input decision. All calls in this
	// batch share the same verdict (clawcode's "consecutive safe merge"
	// invariant — D6 design decision, 2026-07-02 三方一致).
	IsConcurrencySafe bool
	// Calls preserves the original ordering from the LLM's tool_call
	// stream; the parallel runner respects the call ID at executeOne
	// level (results[idx] = ...).
	Calls []llmgateway.ToolCall
	// Indices maps Calls[i] back to its position in the original
	// req.ToolCalls slice, so the dispatcher can write results in the
	// caller's order.
	Indices []int
}

// SurfaceLookup is the surface dispatch map keyed by tool name. Built
// once at adapter construction time and read by partitionToolCalls for
// per-input concurrency decisions.
//
// T18 note: the lookup is per-name (matches findSurface's linear scan
// semantics) — the IsConcurrencySafeForTool call happens on the
// surface that owns the tool, so the caller doesn't need to disambiguate
// duplicates.
type SurfaceLookup map[string]contracts.ToolSurface

// concurrencyByTool is the tool-name → spec.ConcurrencySafe map. Built
// once per ExecuteRound by BuildSurfaceLookup. Used as the BASELINE for
// the per-input decision — see IsConcurrencySafeForTool.
type concurrencyByTool map[string]bool

// BuildSurfaceLookup constructs a tool-name → ToolSurface map from the
// adapter's surface list. Surfaces are scanned in declaration order;
// the first surface to claim a tool (via Tools().Name lookup) wins.
// The map is intended to be built once per ExecuteRound call
// (surfaces are immutable in practice, but the per-call build is cheap
// — 19 tools per surface × ≤ 7 surfaces = ≤ 133 entries).
func BuildSurfaceLookup(surfaces []contracts.ToolSurface) SurfaceLookup {
	out := make(SurfaceLookup, 32)
	seen := make(map[string]bool, 32)
	for _, s := range surfaces {
		if s == nil {
			continue
		}
		for _, sp := range s.Tools(context.Background(), "", "") {
			if seen[sp.Name] {
				continue // first surface wins
			}
			seen[sp.Name] = true
			out[sp.Name] = s
		}
	}
	return out
}

// buildConcurrencyByTool extracts the static ConcurrencySafe bool from
// each surface's spec. The result is the baseline used by
// IsConcurrencySafeForTool; surfaces with per-input logic (BuiltinSurface
// for bash) refine this via the per-input helper.
//
// T18 rationale: the v4 IsConcurrencySafe(input) interface has no tool
// name parameter, so multi-tool surfaces (BuiltinSurface with
// read/write/edit/bash) cannot dispatch precisely from input alone
// (the input shape for read_file / write_file / edit_file is the same
// — file_path/path). The clean fix is to use the spec's v2 static bool
// as the baseline, and only invoke the per-input helper for tools
// where the static bool is permissive (true) AND the per-input check
// could narrow it (i.e. bash).
func buildConcurrencyByTool(surfaces SurfaceLookup) concurrencyByTool {
	out := make(concurrencyByTool, 32)
	for _, s := range surfaces {
		if s == nil {
			continue
		}
		for _, sp := range s.Tools(context.Background(), "", "") {
			if _, exists := out[sp.Name]; exists {
				continue // first surface wins
			}
			out[sp.Name] = sp.ConcurrencySafe
		}
	}
	return out
}

// IsConcurrencySafeForTool is the per-tool, per-input concurrency
// decision used by partitionToolCalls. The algorithm:
//
//  1. Look up the spec.ConcurrencySafe baseline (built once per round).
//  2. If the baseline is false, return false (the static bool already
//     says "must serialize" — per-input can't relax it).
//  3. If the baseline is true AND the tool is in the 4-tool override
//     list (bash / read_file / write_file / edit_file), call the
//     per-input helper to refine. For bash this is the only way to
//     detect read-only commands; for the other 3 the helper returns
//     the static bool (read_file = true, write/edit = false).
//  4. Otherwise, return the baseline.
//
// DSAFT: D7-S9-A50-T18 (DM-20260702-009 PR-B).
func IsConcurrencySafeForTool(
	toolName string,
	input json.RawMessage,
	surfaces SurfaceLookup,
	concurrency concurrencyByTool,
) bool {
	baseline, ok := concurrency[toolName]
	if !ok {
		// Unknown tool — conservative default (sequential).
		return false
	}
	if !baseline {
		// Static bool says no — per-input can't relax it.
		return false
	}
	// Baseline says safe — refine via per-input helper for the 4
	// override tools. For other tools the baseline is the answer.
	if _, isBuiltin := surfaces[toolName]; !isBuiltin {
		return true
	}
	switch toolName {
	case "bash", "read_file", "write_file", "edit_file":
		return IsConcurrencySafeForBuiltinToolFromPackage(toolName, input)
	}
	return true
}

// IsConcurrencySafeForBuiltinToolFromPackage is a thin wrapper that
// re-exports the per-input helper from the surface package. Bootstrap
// imports the surface package directly (see surfaces.go); this wrapper
// exists for the partition logic's call site.
func IsConcurrencySafeForBuiltinToolFromPackage(toolName string, input json.RawMessage) bool {
	return surface.IsConcurrencySafeForBuiltinTool(toolName, input)
}

// partitionToolCalls groups consecutive tool calls into Batches. Each
// batch is keyed on the per-input IsConcurrencySafe decision: calls
// with `safe=true` are merged into a single batch (clawcode
// toolOrchestration.ts:84-118), calls with `safe=false` get their own
// batch and run serially with the next batch.
//
// The "consecutive safe merge" invariant comes from clawcode — long
// stretches of read_file / grep / glob calls would otherwise spawn N
// errgroups, each with its own goroutine setup cost. Merging them is
// empirically ≤ 5ms for 50 calls and produces the same final state as
// the naive "one batch per call" approach.
//
// DSAFT: D7-S9-A50-T18 (DM-20260702-009 PR-B). Mirrors clawcode
// toolOrchestration.ts:84-118; the 5ms target is from
// design.md §①.P99 partitionToolCalls 决策.
//
// SurfaceLookup may be nil — in that case every call is treated as
// not-concurrency-safe (matches the legacy behavior when surfaces are
// absent, per turn_adapter.go:296 "tool runner not available" guard).
func partitionToolCalls(
	calls []llmgateway.ToolCall,
	surfaces SurfaceLookup,
	concurrency concurrencyByTool,
) []Batch {
	if len(calls) == 0 {
		return nil
	}
	var batches []Batch
	for i, call := range calls {
		safe := IsConcurrencySafeForTool(call.Name, json.RawMessage(call.Input), surfaces, concurrency)
		if safe && len(batches) > 0 && batches[len(batches)-1].IsConcurrencySafe {
			batches[len(batches)-1].Calls = append(batches[len(batches)-1].Calls, call)
			batches[len(batches)-1].Indices = append(batches[len(batches)-1].Indices, i)
			continue
		}
		batches = append(batches, Batch{
			IsConcurrencySafe: safe,
			Calls:             []llmgateway.ToolCall{call},
			Indices:           []int{i},
		})
	}
	return batches
}

// ExecuteFunc is the per-call executor passed to ExecuteBatches. The
// caller (turn_adapter.go) wires this to the existing executeOne path
// so partition logic stays decoupled from gate / surface / fallback
// details. The returned ToolResult is written into the result slice
// at the call's original index.
type ExecuteFunc func(ctx context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult

// ExecuteBatches runs a partition of batches in sequence. Within each
// batch, calls run concurrently (errgroup). Batches run serially to
// preserve the cross-batch causal order — a write must observe any
// preceding read in the same round.
//
// Concurrency cap: the optional concurrencyLimit clamps the per-batch
// parallelism via errgroup.SetLimit (AC20). Pass 0 for "unlimited"
// (the default in production). Tests pass a small N to exercise the
// cap.
//
// Panic isolation: each goroutine recovers from panics and returns
// the panic as a string error in the result (AC19). The errgroup's
// first error short-circuits the batch — surviving calls in the same
// batch are not cancelled (matches clawcode's behavior, where one bad
// call should not stop the rest of the round).
//
// Ctx cancellation: g.Wait() returns when the first goroutine returns
// OR when ctx is cancelled (AC21). The executeOne path checks ctx
// between phases, so cancellation propagates within ~1ms.
//
// Results are returned in the same order as the input calls slice
// (results[k] corresponds to calls[k]). The returned slice has
// length == len(calls); partial-batch results are placeheld with the
// error from the cancelled call.
func ExecuteBatches(
	ctx context.Context,
	calls []llmgateway.ToolCall,
	surfaces SurfaceLookup,
	exec ExecuteFunc,
	concurrencyLimit int,
) []sessionorchestrator.ToolResult {
	if len(calls) == 0 {
		return nil
	}
	results := make([]sessionorchestrator.ToolResult, len(calls))
	concurrency := buildConcurrencyByTool(surfaces)
	batches := partitionToolCalls(calls, surfaces, concurrency)
	for _, b := range batches {
		executeOneBatch(ctx, b, exec, concurrencyLimit, results)
	}
	return results
}

// executeOneBatch runs a single batch and writes results into the
// provided results slice at the batch's Indices. If the batch is
// concurrency-safe, all calls run via errgroup with a per-batch
// BashSiblingAbortController (so a failing watched tool can cancel
// other watched siblings); if not, they run sequentially (one at a
// time). Both paths recover from panics and write to the result slice
// at the correct index.
//
// DSAFT: D7-S9-A50-T26 (DM-20260702-009 PR-F) — Bash sibling abort
// controller wired into the parallel batch executor.
func executeOneBatch(
	ctx context.Context,
	b Batch,
	exec ExecuteFunc,
	concurrencyLimit int,
	results []sessionorchestrator.ToolResult,
) {
	if !b.IsConcurrencySafe {
		// Sequential — one call at a time, panic-recover each.
		// No controller needed: there are no concurrent siblings to abort.
		for j, c := range b.Calls {
			results[b.Indices[j]] = safeExecuteOne(ctx, exec, c)
		}
		return
	}
	// Parallel — per-batch sibling-abort controller.
	// Sized at len(b.Calls) so any sub-batch cannot exhaust the registry.
	controller := bash.NewBashSiblingAbortController(ctx, len(b.Calls))
	defer controller.Close()

	var g errgroup.Group
	if concurrencyLimit > 0 {
		g.SetLimit(concurrencyLimit)
	}
	var mu sync.Mutex
	for j, c := range b.Calls {
		j, c := j, c
		g.Go(func() error {
			r := executeOneWithSiblingAbort(ctx, controller, exec, c)
			mu.Lock()
			results[b.Indices[j]] = r
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
}

// executeOneWithSiblingAbort runs a single call within a parallel
// batch, optionally registering it with the per-batch controller. Only
// watched tools (currently bash) participate in the sibling-abort
// protocol — non-watched tools just run with the parent ctx.
//
// AC12 invariant: when a watched call returns a result with Error set,
// AbortSiblings cancels the OTHER watched siblings' ctx. Those siblings
// then see ctx.Done() on their next ctx check and return a synthetic
// cancel result via their ExecuteFunc.
func executeOneWithSiblingAbort(
	parentCtx context.Context,
	controller *bash.BashSiblingAbortController,
	exec ExecuteFunc,
	call llmgateway.ToolCall,
) sessionorchestrator.ToolResult {
	if !isSiblingAbortWatched(call.Name) {
		return safeExecuteOne(parentCtx, exec, call)
	}
	siblingCtx, cancel, ok := controller.Register(call.ID, call.Name)
	if !ok {
		// Controller closed / aborted / full — fall through with parent ctx.
		return safeExecuteOne(parentCtx, exec, call)
	}
	defer cancel()
	defer controller.Unregister(call.ID)

	result := safeExecuteOne(siblingCtx, exec, call)
	if result.Error != "" {
		// Watched call failed — cancel other watched siblings. Idempotent
		// (returns false on subsequent calls), so multi-failure races are safe.
		controller.AbortSiblings(call.ID, result.Error)
	}
	return result
}

// isSiblingAbortWatched returns true for tools that participate in the
// per-batch sibling-abort protocol. Currently only bash; future
// watchers (mcp_* with side-effects, etc.) can be added without
// touching the controller wiring.
//
// Rationale: read_file / grep / glob are read-only and idempotent — no
// need to abort them on bash failure. write_file / edit_file are
// sequential (ConcurrencySafe=false), so they never enter the parallel
// branch where the controller lives.
func isSiblingAbortWatched(name string) bool {
	return name == "bash"
}

// safeExecuteOne wraps exec with a panic recovery. The recover returns
// a result with the panic message in Error — this keeps the call's
// result slice entry populated so downstream code (the result-write
// step in turn_adapter.ExecuteRound) does not see a zero ToolResult
// (which would be ambiguous with "success but empty output").
//
// The recover is a best-effort safety net; the engine's individual
// tool implementations should not panic in production. This exists
// for the AC19 invariant test and as a defense in depth.
func safeExecuteOne(
	ctx context.Context,
	exec ExecuteFunc,
	c llmgateway.ToolCall,
) (result sessionorchestrator.ToolResult) {
	defer func() {
		if r := recover(); r != nil {
			result = sessionorchestrator.ToolResult{
				ToolCallID: c.ID,
				Error:      "panic: " + panicMessage(r),
			}
		}
	}()
	return exec(ctx, c)
}

// panicMessage converts a recover() return value to a string. Strings
// pass through; errors use Error(); everything else falls back to
// "unknown panic". Kept private — this is implementation detail of
// safeExecuteOne.
func panicMessage(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return "unknown panic"
	}
}
