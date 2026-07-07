# PR-B Consensus Packet — WaveScheduler DAG Executor (4-worker parallel run)

**Date:** 2026-07-07
**PR:** PR-B (DM-20260707-001, 7-PR split, step 3/7)
**Author:** Claude (Sonnet 4.6)
**Reviewers:** codex (MiniMax-M3), cursor-agent

---

## 1. PROBLEM

DM-20260707-001 PR-A1 + PR-A2 (PR #451, #452) shipped:
- **PR-A1**: IntentSegment + PlanDAG grammar + validateDAG (no runtime).
- **PR-A2**: SegmenterDispatcher (LLM/Rule/Dispatcher fallback chain) that
  produces IntentSegmentSet at Observe time.

Now **PR-B** ships the **DAG executor runtime**: given a `plan.PlanDAG`
(one PlanNode per IntentSegment, edges = dependencies), Execute node must
run the nodes in topological order with a 4-worker parallel pool.

The existing `WaveScheduler` (wavescheduler/scheduler.go) already provides:
- 4-slot `WorkerPool` (cursor=1, claude_code=1, subagent=3, workitem=tag)
- `TaskGraph` with `ReadyNodes()` (Kahn-style ready set, deterministic order)
- `ConflictGuard` (atomic AllowAndRegister)
- `dispatchLoop` (ticker + wakeupCh)
- `Start(ctx, sessionID, graph)` + `WaitForCompletion(ctx, sessionID)`

PR-B = a thin **adapter** that converts `plan.PlanDAG` → `wavescheduler.TaskGraph`
and exposes a streaming emit API (`<-chan SegmentEmit`).

Tasks per `openspec/changes/devrix-d7-multi-intent-observation-decompose/tasks.md`:
- **T18**: `DAGExecutor` interface { `RunPlanDAG(ctx, plan PlanDAG) (<-chan SegmentEmit, error)` }
- **T19**: 4 worker pool + channel queue + priority heap over ready nodes
- **T20**: initial ready set = nodes with no incoming edges; new ready set after each completion
- **T21**: error propagation — any child error → cancel unstarted siblings + drain emit

---

## 2. CONSTRAINTS

### 2.1 Hard constraints
1. **No WaveScheduler rewrite.** PR-B is an adapter; do NOT modify
   `wavescheduler/scheduler.go`, `taskgraph.go`, or `pool.go`. The existing
   4-slot pool is the hard cap. (v6.0.x maintenance phase per memory.)
2. **PlanDAG validation already done.** PR-A1's `validateDAG` enforces
   no cycles, no duplicates, no dangling edges, ≤ MaxDAGNodes (default 10),
   ≤ MaxFanOut (default 8). Executor MUST NOT re-validate.
3. **PlanDAG has Priority map.** `PlanDAG.Priorities map[string]int` is
   per-node priority. Executor MUST sort ready nodes by priority desc
   (T19). Ties broken by node ID lex order for determinism.
4. **Hard cap = 4 workers** (subagent pool size). PlanDAG.MaxParallelism
   is **informational only** (v1 ignored per design §2.3).
5. **Stream emit semantics**: `<-chan SegmentEmit` per design §2.4. Each
   child completion produces one SegmentEmit; the **last** completion
   (when all nodes reach terminal state) emits `IsFinal=true` for the
   parent rollup hook.
6. **Error propagation**: any child error → cancel running siblings +
   close emit channel (caller observes the partial emits already buffered
   + ctx.Err() or `ErrDAGExecutionFailed`).
7. **Idempotency of Start()**: re-running RunPlanDAG for the same
   `sessionID` cancels the prior run (mirrors WaveScheduler.Start reentry).

### 2.2 Soft constraints
1. Adapter ≤ 250 LOC; tests ≤ 300 LOC.
2. Coverage ≥ 80% on `dag_executor.go`.
3. New sentinels use `ORCH_DAG_*_72xx` range (7200-7205 already used by
   `dag_validator.go`); new ones for executor: 7210-7213.
4. No LLM calls inside the executor; it only translates + drives the
   existing WaveScheduler.
5. `RunPlanDAG` is synchronous-blocking (returns after the wave is
   terminal or ctx cancelled); the `<-chan SegmentEmit` is the streaming
   surface — caller must consume in a separate goroutine.

---

## 3. SCOPE

### 3.1 NEW file: `wavescheduler/dag_executor.go` (~250 LOC)

```go
package wavescheduler

import (
    "github.com/devrix/devrix/internal/layers/orchestration/plan"
)

type SegmentEmit struct {
    SessionID  string
    PlanDAGID  string                // for idempotency
    SegmentID  string                // PlanNode.ID (== PlanNode.SegmentID for v1)
    WorkerType plan.WorkerHint       // for downstream metrics
    IsFinal    bool                  // true on the parent rollup emit
    StartedAt  time.Time
    EndedAt    time.Time
    Summary    string                // worker's output (mirror Artifact.Summary)
    ExitCode   int
    Error      string                // empty on success
}

type DAGExecutor interface {
    RunPlanDAG(ctx context.Context, sessionID, planDAGID string,
               dag *plan.PlanDAG, segSet *interfaces.IntentSegmentSet) (
        <-chan SegmentEmit, error)
}

type dagExecutor struct {
    scheduler *WaveScheduler
    pool      *WorkerPool
    guard     *ConflictGuard
    resolver  ContextResolverIface
    artifacts *ArtifactStore
    runners   map[WorkerType]WorkerRunner
    obsBridge *observability.Bridge
}

func NewDAGExecutor(scheduler *WaveScheduler, deps SchedulerDeps) DAGExecutor {
    return &dagExecutor{scheduler: scheduler, ...}
}

func (d *dagExecutor) RunPlanDAG(ctx, sessionID, planDAGID string,
                                  dag *plan.PlanDAG, segSet *ifaces.IntentSegmentSet) (
    <-chan SegmentEmit, error) {
    // 1. Convert plan.PlanDAG → wavescheduler.TaskGraph
    // 2. Override TaskGraph's ready-set ordering with PlanDAG.Priorities
    //    (we hook into dispatchLoop via a small wrapper)
    // 3. scheduler.Start(ctx, sessionID, taskGraph)
    // 4. Spawn a goroutine that:
    //    - reads state.graph.State(id) on each tick
    //    - on terminal: emit SegmentEmit
    //    - on AllTerminal: emit IsFinal=true, close channel
    //    - on any error: cancel siblings, drain, close
    // 5. Return the channel
}
```

### 3.2 Conversion: `plan.PlanDAG` → `wavescheduler.TaskGraph`

For each `plan.PlanNode`:
- `TaskNode.ID` = `PlanNode.ID`
- `TaskNode.Directive` = lookup `PlanNode.SegmentID` in `IntentSegmentSet.Segments[*].Text`
- `TaskNode.WorkerType` = map `PlanNode.WorkerHint` → enum (see Q3)
- `TaskNode.DependsOn` = derived from `PlanDAG.Edges` (incoming edges to this node)
- `TaskNode.ContextPolicy` = `ContextFresh` (v1: no segment data flow)
- `TaskNode.Metadata["priority"]` = `PlanDAG.Priorities[id]` (or 50 if absent)
- `TaskNode.Metadata["plan_dag_id"]` = `planDAGID`
- `TaskNode.Metadata["segment_id"]` = `PlanNode.SegmentID`

### 3.3 NEW file: `wavescheduler/dag_executor_test.go` (~300 LOC)

Test surface (~12 tests):
- `TestRunPlanDAG_HappyPath_3Parallel`: 3 independent nodes → 3 emits, IsFinal on last
- `TestRunPlanDAG_TopologicalOrder`: 3-node chain (a→b→c) → emits in order a,b,c
- `TestRunPlanDAG_PriorityOrder`: 3 ready nodes with priorities {a:10, b:90, c:50} → b, c, a
- `TestRunPlanDAG_RespectsHardCap4`: 6 ready nodes → only 4 dispatched at once
- `TestRunPlanDAG_ChildError_CancelsSiblings`: 1 of 3 children errors → other 2 cancelled, channel closed
- `TestRunPlanDAG_CtxCancel_TerminatesAll`: ctx cancel mid-flight → all workers stop
- `TestRunPlanDAG_DuplicateRun_ReentryCancelsPrior`: RunPlanDAG twice for same sessionID → first cancelled
- `TestRunPlanDAG_NilDAG_Errors`: nil *plan.PlanDAG → ErrDAGExecutorNilDAG
- `TestRunPlanDAG_NilIntentSegmentSet_Errors`: nil *IntentSegmentSet → ErrDAGExecutorNilSegmentSet
- `TestRunPlanDAG_Conversion_PreservesDirective`: PlanNode.SegmentID=seg_a, segment text="查 plan" → TaskNode.Directive="查 plan"
- `TestRunPlanDAG_PriorityDefaults_50`: priority absent in map → Metadata["priority"]=50
- `TestRunPlanDAG_Conversion_WorkerHintMapping`: PlanNode.WorkerHint="subagent" → TaskNode.WorkerType=WorkerSubAgent

### 3.4 NEW sentinels (extend `wavescheduler/errors.go` or new file `dag_executor.go`)

```go
// ORCH_DAG_EXECUTOR_NIL_DAG_7210
ErrDAGExecutorNilDAG = errors.New(...)
// ORCH_DAG_EXECUTOR_NIL_SEGSET_7211
ErrDAGExecutorNilSegmentSet = errors.New(...)
// ORCH_DAG_EXECUTOR_MISSING_SEGMENT_7212 — PlanNode.SegmentID not in IntentSegmentSet
ErrDAGExecutorMissingSegment = errors.New(...)
// ORCH_DAG_EXECUTOR_EXECUTION_FAILED_7213
ErrDAGExecutionFailed = errors.New(...)
```

Wrap helpers in `dag_executor.go`.

### 3.5 NOT in scope (deferred to PR-C, PR-D, PR-E)
- **PR-C**: streaming emit → IM adapter (idempotency key `(sessionID, segmentID)` + dedup table)
- **PR-D**: ItemPipelineRunner.Run() detection of `Plan.DAG != nil` → call DAGExecutor
- **PR-E**: Learn per-segment + ParentEvidence aggregator
- **PR-F**: Plan 26-scenario coverage (P12-P17 Parse Reject handling)

### 3.6 Integration with ItemPipelineRunner (PR-D, NOT in this PR)

PR-B only ships the executor. Wiring ItemPipelineRunner to detect
`Plan.DAG != nil` and call `DAGExecutor.RunPlanDAG` is **PR-D** (DM-20260707-001 §5.7.1
decision 1). PR-B ships the runtime + tests in isolation; the
`ItemPipelineRunner` integration comes in PR-D with the e2e LP-3 test.

**This is a deliberate scope cut**: PR-B reduces blast radius and lets
codex/cursor review the executor without the ItemPipelineRunner wiring
noise. The executor is usable from tests directly; PR-D adds production wiring.

---

## 4. OPEN QUESTIONS

### Q1. Where does `DAGExecutor` live?
**(A)** `wavescheduler/dag_executor.go` (per design §2.4) — same package as WaveScheduler
**(B)** `wavescheduler/executor/` subpackage
**(C)** `sessionorchestrator/dag_executor.go` (where the wiring lives)

→ **Recommendation: (A)** — adapter in same package as the WaveScheduler it
drives; no import cycle; the wire-through ItemPipelineRunner is PR-D's job.

### Q2. How is priority ordering injected into WaveScheduler?
WaveScheduler.dispatchLoop sorts `ReadyNodes()` lex by ID. To respect
PlanDAG.Priorities we need to override. Options:
**(A)** Wrap the TaskGraph with a custom `ReadyNodes()` (interface
       override — bigger refactor)
**(B)** Add a `SortReadyNodes func([]TaskNode) []TaskNode` hook to TaskGraph
       (small additive change, no behavior change for existing callers)
**(C)** Sort in DAGExecutor BEFORE calling scheduler.Start by reordering
       nodes such that high-priority nodes have lower IDs (hacky, breaks
       ID-based semantics)

→ **Recommendation: (B)** — minimal additive change to TaskGraph, nil
hook = existing lex order. PR-B adds the hook + sorts by priority desc
(ties by ID asc).

### Q3. PlanNode.WorkerHint → TaskNode.WorkerType mapping
**(A)** String passthrough: hint="subagent" → WorkerType=WorkerSubAgent; empty → WorkerSubAgent
**(B)** New enum `plan.WorkerHint` (string) with explicit mapping table
**(C)** All PlanNodes default to WorkerSubAgent regardless of hint (v1 simplification)

→ **Recommendation: (B)** with a 4-row mapping table:
- "" or "subagent" → WorkerSubAgent
- "cursor" → WorkerCursor
- "claude_code" → WorkerClaudeCode
- "workitem" → WorkerWorkItem (tag-only, no slot)

Unknown hint → fallback to WorkerSubAgent + slog.Warn (defensive).

### Q4. Error semantics when a child fails
**(A)** Cancel all running siblings, drain emit channel, return ctx.Err()
       from WaitForCompletion
**(B)** Cancel running siblings, drain emits with Error=field set on
       cancelled ones, close channel
**(C)** Let siblings complete (partial-failure-tolerant); close channel
       after all nodes terminal regardless

→ **Recommendation: (A)** — strict: any error in any child aborts the
wave. Mirrors WaveScheduler's existing semantic (`CancelAll`). Caller
sees ctx.Canceled + already-buffered emits.

### Q5. Where does the parent rollup hook live?
**(A)** PR-B: `IsFinal=true` is set on the LAST emit (the chronologically
       last child to complete). No actual rollup logic.
**(B)** PR-B: skip IsFinal entirely; parent rollup is PR-C's job
**(C)** PR-B: implement a basic rollup that joins all segment summaries
       into one final summary

→ **Recommendation: (A)** — flag-only, no rollup logic. PR-C will wire the
actual rollup into the IM adapter. Keeps PR-B focused on the runtime.

### Q6. Test placement: same package or external?
**(A)** `wavescheduler/dag_executor_test.go` (internal `package wavescheduler`)
**(B)** `wavescheduler/dag_executor_external_test.go` (`package wavescheduler_test`)

→ **Recommendation: (A)** — internal access to `dispatchLoop` internals is
useful for the priority override test.

### Q7. Idempotency / dedup at the executor level?
**(A)** No dedup; PR-C's IM adapter owns the dedup table
**(B)** Executor maintains an in-memory `(sessionID, segmentID)` dedup,
       drops duplicate emits (defense-in-depth)

→ **Recommendation: (A)** — single responsibility. Executor emits; PR-C's
streaming layer dedups. Avoids two sources of truth.

### Q8. Coverage target for `dag_executor.go`?
**(A)** ≥ 80% (devrix/testing.md standard)
**(B)** ≥ 90% (matches PR-A1 strict target)
**(C)** ≥ 95% (pure-function-friendly file)

→ **Recommendation: (A)** — 80% is the project standard; the executor has
~10 functions all unit-testable. The `dispatchLoop` integration is
inherently hard to fully cover (race timing) — accept ~80%.

---

## 5. DELIVERABLES

1. `wavescheduler/dag_executor.go` (~250 LOC)
2. `wavescheduler/dag_executor_test.go` (~300 LOC, 12 tests)
3. `wavescheduler/taskgraph.go` +5 LOC: `SortReadyNodes` field on TaskGraph
4. `wavescheduler/errors.go` +4 sentinels + 4 wrap helpers
5. Tests: 12 new unit tests, race-clean
6. Coverage ≥ 80% on `dag_executor.go`
7. `go vet` clean, `go build ./...` clean
8. `go test -race ./internal/layers/orchestration/wavescheduler/...` passes

---

## 6. TEST MATRIX

| Scenario | Input | Expected | Test |
|----------|-------|----------|------|
| 3 parallel, no deps | 3-node PlanDAG, no edges | 3 emits, IsFinal on last | TestRunPlanDAG_HappyPath_3Parallel |
| Linear chain | a→b→c | emit order: a, b, c | TestRunPlanDAG_TopologicalOrder |
| Priority ordering | 3 ready, prios {a:10, b:90, c:50} | emit order: b, c, a | TestRunPlanDAG_PriorityOrder |
| Hard cap 4 | 6 ready nodes | 4 dispatched at once | TestRunPlanDAG_RespectsHardCap4 |
| Child errors | 1 of 3 errors | siblings cancelled, channel closed | TestRunPlanDAG_ChildError_CancelsSiblings |
| Ctx cancel | mid-flight cancel | all workers stop | TestRunPlanDAG_CtxCancel_TerminatesAll |
| Reentry | RunPlanDAG twice same session | first cancelled | TestRunPlanDAG_DuplicateRun_ReentryCancelsPrior |
| nil DAG | nil *PlanDAG | ErrDAGExecutorNilDAG | TestRunPlanDAG_NilDAG_Errors |
| nil segSet | nil *IntentSegmentSet | ErrDAGExecutorNilSegmentSet | TestRunPlanDAG_NilIntentSegmentSet_Errors |
| Conversion directive | seg_a text="查 plan" | TaskNode.Directive="查 plan" | TestRunPlanDAG_Conversion_PreservesDirective |
| Priority default | absent in map | TaskNode.Metadata["priority"]=50 | TestRunPlanDAG_PriorityDefaults_50 |
| WorkerHint mapping | hint="subagent" | WorkerType=WorkerSubAgent | TestRunPlanDAG_Conversion_WorkerHintMapping |

---

## 7. RISK REGISTER

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Priority hook breaks existing WaveScheduler callers | Med | nil hook = lex order (default); regression test on existing scheduler_test.go |
| SortReadyNodes introduces race with concurrent SetState | Med | Add RWMutex; the sort happens inside dispatchLoop under existing graph.mu |
| 4-worker cap is too tight for `MaxParallelism=8` PlanDAGs | Low | MaxParallelism is informational; v1 ignores per design §2.3 |
| PR-D wiring may need different interface | Low | Interface is minimal (1 method); easy to extend |
| Child panic in worker not caught by executor | Med | WorkerPool's existing panic recovery (via dispatchOne) covers it; no change needed |

---

## 8. CONSENSUS QUESTIONS (for codex/cursor)

Please respond with **ACCEPT / ADOPT-WITH-CHANGE / REJECT** for each:

1. **Q1**: DAGExecutor in `wavescheduler/dag_executor.go` (same package as WaveScheduler)?
2. **Q2**: Add `SortReadyNodes` hook to TaskGraph for priority override?
3. **Q3**: `plan.WorkerHint` → `WorkerType` mapping (4-row table; unknown → subagent)?
4. **Q4**: Any child error cancels running siblings + drains emit + closes channel?
5. **Q5**: `IsFinal=true` flag-only on last emit (no rollup logic in PR-B)?
6. **Q6**: Tests in `package wavescheduler` (internal)?
7. **Q7**: No executor-level dedup; PR-C's streaming layer owns it?
8. **Q8**: Coverage ≥ 80% (project standard)?
9. **Sentinel codes 7210-7213** for executor errors?
10. **PR-B scope cut**: no ItemPipelineRunner wiring (deferred to PR-D)?

---

## 9. REVIEW DELIVERABLES

After review, expected output:
- `reviews/pr-b-codex-consensus-2026-07-07.md` — codex's responses
- (Optional) `reviews/pr-b-cursor-consensus-2026-07-07.md` — cursor's responses
- Implementation follows adopted consensus
