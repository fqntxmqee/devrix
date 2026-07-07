# PR-B Consensus Review — codex (MiniMax-M3), 2026-07-07

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Reviewer:** codex exec (MiniMax-M3)
**Packet:** `reviews/pr-b-consensus-packet.md`
**Tokens:** 44,386
**Status:** Single-reviewer consensus (cursor quota lockout until 2026-07-20)

---

## Q1 — DAGExecutor location (`wavescheduler/dag_executor.go`)

**ACCEPT.** Design §2.4, `proposal.md:90`, and `tasks.md:100` all converge on `internal/layers/orchestration/wavescheduler/dag_executor.go`. The executor is a thin adapter that drives `WaveScheduler.dispatchLoop`, `WorkerPool`, and `TaskGraph.ReadyNodes` — putting it in the same package eliminates import gymnastics and lets tests (Q6) poke at unexported helpers.

No change.

## Q2 — `SortReadyNodes` hook on TaskGraph

**ACCEPT-WITH-CHANGE.** Option B is the right shape: `SortReadyNodes func([]TaskNode) []TaskNode` on `TaskGraph` with `nil` = existing lex order. Purely additive — existing tests stay green because `ReadyNodes()` falls back to inline `sort.Slice` when the hook is nil.

Two implementation details:

1. **Run the hook under the RLock with explicit doc.** `ReadyNodes()` holds `g.mu.RLock()` while computing. If the hook reads `g.nodes`/`g.states` it would re-RLock (allowed), but the hook MUST NOT call `g.SetState` (write lock from same goroutine = deadlock). Codify in the field doc:
   > *"SortReadyNodes is invoked under TaskGraph.mu read lock; the hook MUST NOT call SetState, Mutate, or any write method on TaskGraph."*

2. **Signature: in-place vs copy.** Recommend `func([]TaskNode)` (in-place `sort.Slice` over the same backing array) since `ReadyNodes()` already allocates a fresh slice per call — no aliasing risk.

## Q3 — `plan.WorkerHint` → `WorkerType` mapping (4-row table)

**ACCEPT-WITH-CHANGE.** The 4-row mapping is correct, but the `WorkerWorkItem` row is a footgun.

`wavescheduler/types.go:233-237` defines `WorkerWorkItem` as **tag-only, no slot**. Routing a `WorkerHint="workitem"` to `WorkerWorkItem` would enqueue a node that can never acquire a slot — the dispatchLoop will spin and `TestRunPlanDAG_RespectsHardCap4` will hang on any DAG containing a workitem-hinted node.

**Recommended fix (option b)**: route workitem hints to `WorkerSubAgent` and stamp `TaskNode.Metadata["workitem_tag"]=true` so downstream metrics/observability still see the hint lineage. More forgiving and matches v1 "informational" stance on hints.

**Unknown hints**: bump from `slog.Warn` to `slog.Error` (an LLM emitting a bogus hint is a contract violation, not a soft anomaly).

## Q4 — Error semantics: strict cancel-all on first child failure

**ACCEPT.** Strict semantics (option A) are the right call for D7 multi-intent. Existing `WaveScheduler.CancelAll` (`scheduler.go:669`) implements the cancellation primitive; executor calls it and lets `WaitForCompletion` (`scheduler.go:697`) unwind. Caller-visible contract: receives already-buffered emits on the channel, then either `ctx.Err()` (cancel) or `ErrDAGExecutionFailed` (child error). Document in the `DAGExecutor` interface doc comment so PR-C knows what to gate on.

No change.

## Q5 — `IsFinal=true` flag-only on last emit

**ACCEPT.** Option A keeps PR-B at ~250 LOC and matches T19/T20/T21. Flag is a contract surface for PR-C (IM adapter final-card emit) and PR-E (Learn parent rollup); both can gate on `IsFinal` without needing the actual rollup logic.

**One follow-up**: clarify cancel semantics. If `ctx.Done()` fires mid-wave, executor MUST still close the channel, but `IsFinal=true` should NOT be emitted (wave never reached AllTerminal). Caller treats "channel closed without IsFinal" as cancel/abort signal — symmetric with cancel-all. Add one-liner to `RunPlanDAG` doc.

## Q6 — Tests in `package wavescheduler` (internal)

**ACCEPT.** Q2 sort-hook test and priority-override test (`TestRunPlanDAG_PriorityOrder`) need unexported access. Existing `scheduler_test.go` / `taskgraph_test.go` are internal-package; `dag_executor_test.go` joins the same package.

No change.

## Q7 — No executor-level dedup; PR-C owns it

**ACCEPT.** Single responsibility. Executor-side reentry cancellation (constraint 7) already prevents the only in-process duplicate path. Remaining duplicate risk is at IM adapter (network retries, partial-card failures) — PR-C's table.

No change.

## Q8 — Coverage ≥ 80%

**ACCEPT.** Project standard per `devrix/testing.md`. DispatchLoop tick-polling goroutine is inherently timing-sensitive — chasing 90%+ buys flaky tests, not safety. 12-test surface covers all happy/error/conversion/lifecycle paths; 80% is right target.

No change.

## Q9 — Sentinel codes 7210-7213 for executor errors

**ACCEPT-WITH-CHANGE.** Codes are consistent with `plan/dag_validator.go:178-218` (`ORCH_DAG_EMPTY_7200` … `ORCH_DAG_CYCLE_7205`).

**Package-boundary inconsistency the packet glosses over:**
- `plan/dag_validator.go` uses `internal/shared/errors.SentinelError`
- `wavescheduler/errors.go` uses its own private `waveError` struct — no `Code:`, no `errors.Is` chain

**Recommended fix**: define the four executor sentinels as `*sharederrors.SentinelError` wrap helpers (mirror `dag_validator.go:176-220` style) in a new file `wavescheduler/dag_executor_errors.go`. This:
- (a) preserves AGENTS.md mandate ("使用 `internal/shared/errors/` SentinelError 模式")
- (b) keeps audit-log consistency with validator's 7200-7205 series
- (c) surgical addition — does not touch existing `errWave` style used elsewhere in wavescheduler. PR-B does not refactor existing wavescheduler error style.

## Q10 — PR-B scope cut (no ItemPipelineRunner wiring)

**ACCEPT.** PR-D wiring belongs in PR-D. ItemPipelineRunner.Run() detection of `Plan.DAG != nil` (`tasks.md:119, T22`) is a one-method fork that's only meaningful with PR-B's executor present, but the e2e LP-3 needs PR-B's tests green first. Shipping executor + tests in isolation lets PR-B review focus on runtime correctness. `d7.dag_executor.enabled=false` default flag means no production behavior change at PR-B merge.

No change.

---

## New risks beyond §7 RISK REGISTER

1. **`WorkerHint="workitem"` routing deadlocks (Med).** Already flagged in Q3 — `WorkerWorkItem` has no slot (`types.go:233-237`). Mitigation: see Q3 recommendation (route to subagent + stamp workitem_tag metadata).

2. **DAGExecutor constructor signature redundancy (Low).** §3.1 shows `NewDAGExecutor(scheduler *WaveScheduler, deps SchedulerDeps)` but `NewWaveScheduler` already stores `pool`, `guard`, `resolver`, `artifacts`, `runners`, `obsBridge` from `deps`. Executor can read all off `s.pool`, `s.guard`, etc. Recommend `NewDAGExecutor(scheduler *WaveScheduler) DAGExecutor` — drop second arg.

3. **Polling-goroutine vs `WaitForCompletion` teardown race (Med).** §3.1 step 4 has executor spawn goroutine that polls `state.graph.State(id)`. Meanwhile `WaitForCompletion` (`scheduler.go:697-722`) deletes `s.waves[sessionID]` after `<-state.doneCh`. If polling goroutine holds `state` via closure and reads `state.graph` after map delete, no race on graph itself (GC keeps alive), but `state.cancel` and `state.handles` could be inconsistent. Mitigation: polling goroutine should capture `state.graph` (stable `*TaskGraph`) and `state.doneCh` at start, never re-read `s.waves`.

4. **No `slog` audit trail for executor lifecycle (Low).** Validator logs each reject (implied by audit-log comment at `dag_validator.go:173`); executor should similarly log `slog.Info("dag_executor.run_start", sessionID, planDAGID, node_count)` and `slog.Info("dag_executor.run_done", sessionID, ...)` with cancellation reason. Reuse WaveScheduler's existing spans via `startOrchSpan` (in-package).

5. **`SegmentEmit.StartedAt` / `EndedAt` source-of-truth (Low).** Struct has both timestamps but proposed executor polls `state.graph.State(id)` rather than receiving terminal `Artifact` directly. `Artifact` carries worker's actual start/end time (`scheduler.go:536-552` `completeTask`). If polling latency exceeds worker's actual end time by >10ms, `EndedAt` will be wrong. Mitigation: read `Artifact.StartedAt`/`EndedAt` from `s.artifacts.ListForSession(sessionID)` (returned by `WaitForCompletion`) at emit time, not from poll timestamp. Test `TestRunPlanDAG_HappyPath_3Parallel` should assert `EndedAt >= StartedAt`.

6. **`RunPlanDAG` returning channel while blocking — contract clarity (Medium).** §2.2.5 says "synchronous-blocking (returns after wave terminal or ctx cancelled); channel is streaming surface — caller must consume in separate goroutine." Fine but non-obvious. Recommend: add doc comment "the returned channel is closed when RunPlanDAG returns; do not block on the channel before spawning your consumer goroutine, or you will deadlock." Out of scope for PR-B; flag for PR-D when wiring ItemPipelineRunner.

7. **Constraint 7 reentry semantics leak the prior channel (Low).** When `RunPlanDAG(sessionID=X)` is called twice, the prior call's `<-chan SegmentEmit` is never explicitly closed — second `scheduler.Start` cancels prior wave, but prior executor goroutine may still be in mid-poll. Caller of first invocation could observe `ctx.Err()` from `WaitForCompletion` (internal) while their channel never gets `IsFinal=true` emit. Test `TestRunPlanDAG_DuplicateRun_ReentryCancelsPrior` should assert "first channel is closed" not "first channel receives IsFinal=true".

---

## Adopted (summary)

- Q1: ACCEPT — `wavescheduler/dag_executor.go`
- Q2: ACCEPT-WITH-CHANGE — `SortReadyNodes` hook, doc-comment locks + in-place sort signature `func([]TaskNode)`
- Q3: ACCEPT-WITH-CHANGE — workitem hint route-to-subagent + `metadata["workitem_tag"]=true`; unknown → `slog.Error`
- Q4: ACCEPT — strict cancel-all
- Q5: ACCEPT — flag-only; document cancel-without-IsFinal
- Q6: ACCEPT — internal `package wavescheduler`
- Q7: ACCEPT — no executor-side dedup
- Q8: ACCEPT — 80% coverage
- Q9: ACCEPT-WITH-CHANGE — 4 sentinels as `sharederrors.SentinelError` wrap helpers in `wavescheduler/dag_executor_errors.go`
- Q10: ACCEPT — scope cut, no ItemPipelineRunner wiring

## Cursor status

Cursor-agent quota lockout continues from PR-A1 (resets 2026-07-20). Single-reviewer consensus. Once cursor resumes, follow-up review focused on:
- Priority-hook determinism (Q2)
- Goroutine-teardown race (new risk #3)

Neither blocks PR-B start.

---

## Implementation TODOs (adopted)

- [ ] `wavescheduler/dag_executor.go` — `RunPlanDAG` + spawn consumer goroutine + emit on terminal/cancel/final
- [ ] `wavescheduler/taskgraph.go` + ~10 LOC — `SortReadyNodes func([]TaskNode)` field on `TaskGraph` + invoke before `sort.Slice` if non-nil (with explicit "MUST NOT call write methods" doc)
- [ ] `wavescheduler/dag_executor_errors.go` (NEW) — 4 `sharederrors.SentinelError` wrap helpers (7210-7213)
- [ ] `wavescheduler/dag_executor.go` — `NewDAGExecutor(scheduler *WaveScheduler) DAGExecutor` (drop redundant `deps` arg)
- [ ] `wavescheduler/dag_executor.go` — explicit `WorkerHint="workitem"` handling: route-to-subagent + `metadata["workitem_tag"]=true`
- [ ] `wavescheduler/dag_executor.go` — unknown hint → `slog.Error` (not slog.Warn) with offending hint in fields
- [ ] `wavescheduler/dag_executor_test.go` — 12 tests per §3.3; add `TestRunPlanDAG_PollingGoroutine_TimestampFromArtifact` (new risk #5) and `TestRunPlanDAG_DuplicateRun_FirstChannelClosed` (new risk #7)
- [ ] `wavescheduler/dag_executor.go` — header doc comment naming relationship to `TaskGraph`, `WaveScheduler`, and dedup boundary with PR-C
- [ ] `wavescheduler/dag_executor.go` — `slog.Info("dag_executor.run_start"/"run_done", ...)` lifecycle audit (new risk #4)
- [ ] Regression: `go test -race ./internal/layers/orchestration/wavescheduler/...` (existing 8 test files stay green)
