# PR-B Consensus — cursor-agent (independent review), 2026-07-07

**Packet:** `reviews/pr-b-consensus-packet.md`
**Change:** DM-20260707-001 PR-B
**Reviewer:** cursor-agent

---

## WorkerWorkItem slot-less verification

The packet's claim is **correct**, with a nuance on line numbers.

`types.go:233-237` is `DefaultPoolCapacity` — lists only `cursor=1`, `claude_code=1`, `subagent=3` and **omits** `WorkerWorkItem`:

```go
var DefaultPoolCapacity = map[WorkerType]int{
    WorkerCursor:     1,
    WorkerClaudeCode: 1,
    WorkerSubAgent:   3,
}
```

The "tag-only, no slot" semantics are documented at `types.go:22-27`. `pool.Acquire` returns `("", false)` when the kind is absent from `caps` (`pool.go:53-55`), so `dispatchLoop` will skip a `WorkerWorkItem` node forever while it stays `StatePending` — **a livelock, not merely slow progress**.

---

## Q1 — `DAGExecutor` in `wavescheduler/dag_executor.go`

**ACCEPT.** Same package as `WaveScheduler` matches design §2.4, avoids import cycles, lets tests reach unexported scheduler/graph hooks (Q6). `sessionorchestrator` wiring belongs in PR-D.

## Q2 — `SortReadyNodes` hook on `TaskGraph`

**ADOPT-WITH-CHANGE.** Option B is the right minimal injection point: nil hook preserves today's lex order; non-nil hook runs inside `ReadyNodes()` before dispatch.

**Signature: in-place `func([]TaskNode)` is better than `func([]TaskNode) []TaskNode`.** `ReadyNodes()` already allocates a fresh slice each tick (`taskgraph.go:83-93`); sorting in-place with `sort.Slice` matches existing code, avoids an extra allocation/copy, and has no aliasing risk. A return-slice hook invites unnecessary heap churn and ambiguous ownership.

**Required doc on the field:** hook runs under `TaskGraph.mu` RLock; must not call `SetState` or any write path (deadlock).

**Packet inconsistency to resolve:** §2.1 says "do NOT modify `taskgraph.go`", but §5 deliverable #3 adds `SortReadyNodes` there. Treat §2.1 as "no behavioral rewrite"; the ~10 LOC hook is an allowed additive exception — state explicitly in the PR description.

## Q3 — `plan.WorkerHint` → `WorkerType` mapping

**ADOPT-WITH-CHANGE — route `workitem` to subagent + metadata; do not add sentinel 7214.**

Direct mapping `workitem` → `WorkerWorkItem` is **unsafe** (see verification above). Correct v1 behavior:

- `""` / `"subagent"` → `WorkerSubAgent`
- `"cursor"` → `WorkerCursor`
- `"claude_code"` → `WorkerClaudeCode`
- `"workitem"` → `WorkerSubAgent` + `TaskNode.Metadata["workitem_tag"] = true` (and/or `metadata["worker_hint"] = "workitem"`)

**Reject 7214 for `workitem`.** It is a known, valid hint in the 4-row table; failing the whole DAG at conversion is too strict. Reserve `7212` (`ErrDAGExecutorMissingSegment`) for conversion faults (segment ID not in `IntentSegmentSet`). For **unknown** hints: fallback to `WorkerSubAgent` + `slog.Error` with the offending value — defensive, not `errors.New` at `RunPlanDAG` entry (LLM hint variance is expected in v1).

## Q4 — Strict cancel-all on first child failure

**ADOPT-WITH-CHANGE.** Strict abort is correct for D7 multi-intent, but **`CancelAll` alone is insufficient.** `cancelWaveLocked` only cancels **running** handles (`scheduler.go:682-692`); **pending** nodes stay `StatePending`, so `AllTerminal()` remains false and `dispatchLoop` can spin until wave ctx is cancelled.

Executor error path must:
1. `CancelAll(sessionID)` on running siblings
2. Cancel the wave-scoped ctx passed to `Start` (or equivalent) so `dispatchLoop` exits
3. Explicitly mark remaining `StatePending` nodes `StateCancelled` (conversion-time loop or helper) before relying on emit teardown

Document caller contract: buffered emits may include `StateCancelled` rows with `Error` set; `RunPlanDAG` returns `ErrDAGExecutionFailed` or `ctx.Err()`.

## Q5 — `IsFinal=true` flag-only on last emit

**ADOPT-WITH-CHANGE.** Flag-only `IsFinal` on the last successful terminal emit is the right PR-B scope (no rollup logic).

**Yes — "channel closed without `IsFinal`" must be a documented contract surface.** On `ctx` cancel, reentry cancel, or child-error abort, the executor closes the channel **without** emitting `IsFinal=true`. PR-C/PR-E should treat `IsFinal` as "wave reached natural `AllTerminal` success path", and "closed ∧ ¬IsFinal" as abort/cancel. Add one paragraph to `DAGExecutor` / `RunPlanDAG` godoc and assert in `TestRunPlanDAG_CtxCancel_TerminatesAll` and `TestRunPlanDAG_DuplicateRun_ReentryCancelsPrior`.

**Clarify "last emit":** with parallel completions, "chronologically last" is nondeterministic; define tie-break (e.g. highest `EndedAt`, then lex `SegmentID`) in tests.

## Q6 — Tests in `package wavescheduler` (internal)

**ACCEPT.** Consistent with `scheduler_test.go` / `taskgraph_test.go`; priority-hook and emit-polling tests need internal access.

## Q7 — No executor-level dedup

**ACCEPT.** Dedup at `(sessionID, segmentID)` belongs in PR-C's IM adapter. Executor reentry cancel (constraint 7) covers the in-process duplicate-run case.

## Q8 — Coverage ≥ 80%

**ACCEPT.** Matches project standard; dispatch-loop timing races make 90%+ a flaky-tax tradeoff. Twelve focused tests on conversion, ordering, cap, error, and lifecycle paths are sufficient.

## Q9 — Sentinel codes 7210-7213

**ADOPT-WITH-CHANGE — use `sharederrors.SentinelError` in new `dag_executor_errors.go`; do not use `errWave` for these four.**

`wavescheduler/errors.go` intentionally uses local `waveError` for package self-containment (`errors.go:5-7`). Executor sentinels sit on the **same 72xx audit series** as `plan/dag_validator.go:176-220` and cross package boundaries for metrics/logs — they should mirror validator style (`sharederrors.WithCode("ORCH_DAG_EXECUTOR_*_721x", …)`). Surgical addition only; do not refactor existing `errWave` call sites in PR-B.

Codes 7210-7213 as listed are fine; no 7214 needed if Q3 adopts subagent routing for `workitem`.

## Q10 — PR-B scope cut (no `ItemPipelineRunner` wiring)

**ACCEPT.** Executor + isolated tests first; PR-D adds `Plan.DAG != nil` fork and LP-3 e2e. Minimal one-method interface keeps blast radius low.

---

## Additional risks beyond §7 (codex gaps)

| Risk | Sev | Notes |
|------|-----|-------|
| **Pending nodes survive `CancelAll`** | **High** | See Q4 — strict abort needs pending → `StateCancelled`, not only running cancel. §7 and codex Q4 miss this. |
| **§2.1 vs `taskgraph.go` hook** | Med | Hard "no modify" conflicts with `SortReadyNodes` deliverable; clarify additive-only exception. |
| **Unbuffered emit channel deadlock** | Med | §2.2.5 says caller consumes in another goroutine, but packet doesn't mandate buffered `chan SegmentEmit`. Recommend buffer ≥ node count or document "must start consumer before `RunPlanDAG` returns". |
| **`IsFinal` ordering ambiguity** | Low | Parallel completions make "last chronologically" flaky; needs deterministic rule in code + test. |
| **`depsSatisfied` ignores `StateFailed`/`StateCancelled` parents** | Low | `ReadyNodes` only treats `StateCompleted` as satisfied (`taskgraph.go:77-81`). After upstream fail+cancel, dependents correctly stay pending until executor marks them cancelled — reinforces Q4 pending-cancel requirement. |
| **Emit timestamps from poll vs `Artifact`** | Low | Codex caught this (#5 in their review); prefer `artifacts.ListForSession` / `WaitForCompletion` artifacts for `StartedAt`/`EndedAt`. |
| **`NewDAGExecutor(scheduler, deps)` redundancy** | Low | Codex caught (#2); executor should read pool/guard/runners from `scheduler` only. |

Codex's Q3 workitem deadlock, polling/`WaitForCompletion` race, reentry channel semantics, and `SentinelError` split are all valid and adopted here.

---

## Summary

| # | Verdict |
|---|---------|
| Q1 | ACCEPT |
| Q2 | ADOPT-WITH-CHANGE (in-place `func([]TaskNode)`; doc lock constraint; reconcile §2.1) |
| Q3 | ADOPT-WITH-CHANGE (workitem → subagent + metadata; no 7214) |
| Q4 | ADOPT-WITH-CHANGE (cancel ctx + pending nodes, not only `CancelAll`) |
| Q5 | ADOPT-WITH-CHANGE (document cancel-without-`IsFinal`; deterministic "last") |
| Q6 | ACCEPT |
| Q7 | ACCEPT |
| Q8 | ACCEPT |
| Q9 | ADOPT-WITH-CHANGE (`dag_executor_errors.go` + `SentinelError`) |
| Q10 | ACCEPT |
