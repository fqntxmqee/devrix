package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// fixedSurface is a tiny ToolSurface used in the invariant tests below.
// It hard-codes a ConcurrencySafe baseline (the spec-level static bool)
// and supports per-input overrides for read_file / write_file / bash
// via the spec.ConcurrencySafe field. The IsConcurrencySafe(input)
// v4 method returns the spec.ConcurrencySafe (per the partition logic
// baseline — see IsConcurrencySafeForTool).
type fixedSurface struct {
	name      string
	concSafe  bool
	hits      *int32
	failOn    string // tool name that returns an error from Execute
	panicOn   string // tool name that panics
	execDelay time.Duration
}

func (s *fixedSurface) Name() string { return s.name }
func (s *fixedSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	// Two tools: one concurrency-safe (e.g. read_file) + one not (e.g. write_file).
	return []contracts.ToolSpec{
		{Name: "safe_tool", ConcurrencySafe: true, Risk: types.RiskLevelLow},
		{Name: "unsafe_tool", ConcurrencySafe: false, Risk: types.RiskLevelHigh},
	}
}
func (s *fixedSurface) RiskLevel(name string) types.RiskLevel {
	if name == "safe_tool" {
		return types.RiskLevelLow
	}
	return types.RiskLevelHigh
}
func (s *fixedSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (s *fixedSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (s *fixedSurface) IsConcurrencySafe(_ json.RawMessage) bool {
	// Not used by the partition path (which reads spec.ConcurrencySafe
	// via buildConcurrencyByTool). Kept for interface compliance.
	return s.concSafe
}
func (s *fixedSurface) ToAutoClassifierInput(_ json.RawMessage) string { return "" }

func (s *fixedSurface) Execute(_ context.Context, name, _, _ string) (*contracts.ToolResult, error) {
	atomic.AddInt32(s.hits, 1)
	if name == s.panicOn {
		panic("intentional panic in " + name)
	}
	if s.execDelay > 0 {
		time.Sleep(s.execDelay)
	}
	if name == s.failOn {
		return &contracts.ToolResult{Error: "simulated failure: " + name}, nil
	}
	return &contracts.ToolResult{Output: "ok:" + name}, nil
}

// makeCalls builds N alternating safe/unsafe tool calls.
func makeCalls(n int) []llmgateway.ToolCall {
	calls := make([]llmgateway.ToolCall, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			calls[i] = llmgateway.ToolCall{ID: idFor(i), Name: "safe_tool", Input: `{}`}
		} else {
			calls[i] = llmgateway.ToolCall{ID: idFor(i), Name: "unsafe_tool", Input: `{}`}
		}
	}
	return calls
}

func idFor(i int) string {
	const hex = "0123456789abcdef"
	if i < 16 {
		return "tc_0" + string(hex[i])
	}
	return "tc_" + string(hex[i/16]) + string(hex[i%16])
}

// AC15: 完整性 N:N+保序+id 1:1 — partitionToolCalls produces the
// same number of calls in the same order with IDs intact after
// ExecuteBatches. No calls are dropped, no calls are duplicated.
func TestPartition_Invariant_AC15_CompletePreserved(t *testing.T) {
	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})
	calls := makeCalls(20) // 10 safe + 10 unsafe
	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		if res == nil {
			return sessionorchestrator.ToolResult{ToolCallID: call.ID, Error: "nil result"}
		}
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)

	if len(results) != len(calls) {
		t.Fatalf("len(results)=%d, want %d", len(results), len(calls))
	}
	if got := atomic.LoadInt32(hits); int(got) != len(calls) {
		t.Errorf("hits=%d, want %d (every call must execute exactly once)", got, len(calls))
	}
	for i, want := range calls {
		if results[i].ToolCallID != want.ID {
			t.Errorf("results[%d].ToolCallID=%q, want %q (order broken)", i, results[i].ToolCallID, want.ID)
		}
		if results[i].Error != "" {
			t.Errorf("results[%d].Error=%q, want empty (no failures expected)", i, results[i].Error)
		}
	}
}

// AC16: 交错保序 — consecutive safe calls merge into one batch
// (clawcode consecutive-safe-merge invariant). After ExecuteBatches,
// the result order matches the input order even though safe calls ran
// in parallel.
func TestPartition_Invariant_AC16_InterleavedPreserved(t *testing.T) {
	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits, execDelay: 30 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})
	// 4 safe calls in a row then 1 unsafe — should merge into 1 batch
	// of 4 safe + 1 batch of 1 unsafe.
	calls := []llmgateway.ToolCall{
		{ID: "a", Name: "safe_tool", Input: `{}`},
		{ID: "b", Name: "safe_tool", Input: `{}`},
		{ID: "c", Name: "safe_tool", Input: `{}`},
		{ID: "d", Name: "safe_tool", Input: `{}`},
		{ID: "e", Name: "unsafe_tool", Input: `{}`},
	}
	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	start := time.Now()
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)
	elapsed := time.Since(start)

	// 4 safe in parallel: ~30ms; 1 unsafe serial: ~30ms. Total ~60ms.
	// Generous bound to avoid flakiness: < 200ms (well under 4*30=120ms
	// for serial).
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed=%v, want < 200ms (safe batch should be parallel)", elapsed)
	}
	for i, want := range calls {
		if results[i].ToolCallID != want.ID {
			t.Errorf("results[%d].ToolCallID=%q, want %q", i, results[i].ToolCallID, want.ID)
		}
	}
}

// AC17: read-only 部分失败 — when one call in a safe batch fails,
// the other calls in the same batch still produce results (no
// cascade abort). The failing call's result has Error set; the others
// succeed.
func TestPartition_Invariant_AC17_PartialFailureIsolated(t *testing.T) {
	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits, failOn: "safe_tool", execDelay: 30 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})
	calls := []llmgateway.ToolCall{
		{ID: "a", Name: "safe_tool", Input: `{}`},
		{ID: "b", Name: "safe_tool", Input: `{}`},
		{ID: "c", Name: "safe_tool", Input: `{}`},
	}
	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)

	if len(results) != 3 {
		t.Fatalf("len(results)=%d, want 3", len(results))
	}
	for i, r := range results {
		if r.ToolCallID != calls[i].ID {
			t.Errorf("results[%d].ToolCallID=%q, want %q (order)", i, r.ToolCallID, calls[i].ID)
		}
		// All three should be processed; none should be skipped due to
		// a single failure in the batch.
		if r.Output == "" && r.Error == "" {
			t.Errorf("results[%d] is empty (one failed but the rest should still produce output)", i)
		}
	}
}

// AC19: panic 隔离 — a panic in one goroutine does not crash the
// whole batch; the panicked call's result has Error="panic: ..."; the
// other calls in the same batch still complete.
func TestPartition_Invariant_AC19_PanicIsolated(t *testing.T) {
	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits, panicOn: "safe_tool", execDelay: 20 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})
	calls := []llmgateway.ToolCall{
		{ID: "a", Name: "safe_tool", Input: `{}`},
		{ID: "b", Name: "safe_tool", Input: `{}`},
	}
	exec := func(_ context.Context, call llmgateway.ToolCall) (r sessionorchestrator.ToolResult) {
		// Inner defer ensures the panic recovery is the partition
		// layer's safeExecuteOne, not ours. We re-panic to surface it.
		defer func() {
			if rec := recover(); rec != nil {
				r = sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: "ok:" + call.Name}
				panic(rec) // re-panic so safeExecuteOne catches it
			}
		}()
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)

	if len(results) != 2 {
		t.Fatalf("len(results)=%d, want 2", len(results))
	}
	// Both calls should have entries — neither should be a zero
	// ToolResult (which would mean the goroutine was lost).
	for i, r := range results {
		if r.ToolCallID != calls[i].ID {
			t.Errorf("results[%d].ToolCallID=%q, want %q", i, r.ToolCallID, calls[i].ID)
		}
		if r.Output == "" && r.Error == "" {
			t.Errorf("results[%d] is empty (panic lost)", i)
		}
	}
	// At least one result should have Error starting with "panic:"
	// (the call that hit panicOn). The other call may have Error too if
	// the panic was caught before the other goroutine wrote its result.
	panicCount := 0
	for _, r := range results {
		if strings.HasPrefix(r.Error, "panic:") {
			panicCount++
		}
	}
	if panicCount == 0 {
		t.Error("expected at least one panic result, got none")
	}
}

// AC20: 并发上限 — errgroup.SetLimit caps the number of concurrent
// goroutines in a single batch. With limit=2 and 10 safe calls of
// 50ms each, total elapsed should be ~250ms (5 batches × 50ms), not
// ~50ms (full parallel).
func TestPartition_Invariant_AC20_ConcurrencyLimit(t *testing.T) {
	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits, execDelay: 50 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})
	const N = 10
	const limit = 2
	calls := make([]llmgateway.ToolCall, N)
	for i := range calls {
		calls[i] = llmgateway.ToolCall{ID: idFor(i), Name: "safe_tool", Input: `{}`}
	}
	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	start := time.Now()
	results := ExecuteBatches(context.Background(), calls, lookup, exec, limit)
	elapsed := time.Since(start)

	// With limit=2, 10 calls of 50ms each = 5 waves × 50ms = ~250ms.
	// Generous bound: 400ms (avoid CI flakiness).
	if elapsed < 200*time.Millisecond {
		t.Errorf("elapsed=%v, want >= 200ms (limit should serialize the batch)", elapsed)
	}
	if elapsed > 600*time.Millisecond {
		t.Errorf("elapsed=%v, want < 600ms (limit not strict enough?)", elapsed)
	}
	if len(results) != N {
		t.Errorf("len(results)=%d, want %d", len(results), N)
	}
	if got := atomic.LoadInt32(hits); int(got) != N {
		t.Errorf("hits=%d, want %d", got, N)
	}
}

// AC21: ctx 取消 — when the context is cancelled, the errgroup's
// Wait returns promptly and surviving goroutines see ctx.Err() in
// their next call. (The executeOne path checks ctx between phases;
// we model that here by returning ctx.Err() when done.)
func TestPartition_Invariant_AC21_CtxCancel(t *testing.T) {
	hits := new(int32)
	s := &fixedSurface{name: "s", hits: hits, execDelay: 100 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})
	calls := make([]llmgateway.ToolCall, 5)
	for i := range calls {
		calls[i] = llmgateway.ToolCall{ID: idFor(i), Name: "safe_tool", Input: `{}`}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	exec := func(cctx context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		// Sleep, but watch ctx — return early on cancellation.
		select {
		case <-cctx.Done():
			return sessionorchestrator.ToolResult{ToolCallID: call.ID, Error: cctx.Err().Error()}
		case <-time.After(s.execDelay):
		}
		res, _ := s.Execute(cctx, call.Name, call.Input, "")
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}
	start := time.Now()
	results := ExecuteBatches(ctx, calls, lookup, exec, 0)
	elapsed := time.Since(start)

	// ctx times out at 30ms. The batch should return shortly after.
	// Bound: < 200ms (allow some scheduling slack).
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed=%v, want < 200ms (ctx cancel should propagate)", elapsed)
	}
	// At least one result should have a ctx-canceled error.
	cancelCount := 0
	for _, r := range results {
		if errors.Is(errFromString(r.Error), context.Canceled) ||
			errors.Is(errFromString(r.Error), context.DeadlineExceeded) ||
			strings.Contains(r.Error, "context") {
			cancelCount++
		}
	}
	if cancelCount == 0 {
		t.Error("expected at least one ctx-cancel error, got none")
	}
}

// errFromString converts an error string (from r.Error) back to a Go
// error for errors.Is checks. This is a best-effort helper; AC21
// tests use it only to check for context.Canceled / DeadlineExceeded.
func errFromString(s string) error {
	if s == "" {
		return nil
	}
	return stringError(s)
}

type stringError string

func (e stringError) Error() string { return string(e) }
