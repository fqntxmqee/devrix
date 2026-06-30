# Delta: D7 Orchestration — Review Hardening

**Change ID:** `devrix-d2-d7-review-hardening`  
**Demand ID:** DM-20260630-013  
**Affects:** D7-S1, D7-S2, D7-S3, D7-S9, D7-S14, D7-S15, D7-S16

---

## ADDED

### Requirement: D7-S2-A80 PerInvocationEmit — ItemPipeline 并发隔离

`ItemPipelineRunner.Run` 与 `WorkItemExecutor.Execute` SHALL 通过 **per-invocation 参数**（非共享 struct 可变字段）传递 `Emit` 回调与 `UserContextPrepend`。全局单例 runner/executor 在多个 session 并发 Turn 时 MUST NOT 因字段覆写导致事件串线或 data race。

#### Scenario: Two sessions concurrent emit isolation
- GIVEN session A and session B both enter `RunSessionTurnLoop` concurrently
- WHEN each turn wires a distinct `emitFn` for its session channel
- THEN tool/progress events from session A MUST only arrive on session A's `out` channel
- AND `go test -race` on orchestration packages MUST report zero races on Emit path

#### Scenario: Custom WorkItemExecutor receives emit via opts
- GIVEN a test stub implementing `WorkItemExecutor` (not `*DefaultWorkItemExecutor`)
- WHEN `ItemPipelineRunner.Run` is called with `RunOpts.Emit` set
- THEN the stub MUST receive the emit callback without type assertion to concrete type

---

### Requirement: D7-S3-A84 OnReleaseOnce — WorkerPool hook 不累积

`WaveScheduler` SHALL register at most **one** `OnRelease` hook per `WorkerPool` lifetime。`Start()` 与 `dispatchLoop()` MUST NOT append additional hooks on each wave。

#### Scenario: Repeated Start does not grow hook count
- GIVEN a `WaveScheduler` with default pool
- WHEN `Start()` is called 100 times for the same or different sessions (with cancel between)
- THEN the internal `OnRelease` hook slice length MUST remain 1
- AND each slot `Release` MUST trigger at most one wakeup signal

---

### Requirement: D7-S2-A81 EnsureGoal Error Observability

`SessionOrchestrator.ProcessMessage` SHALL NOT silently discard `TaskManager.Tree().EnsureGoal` errors (`_, _ =` forbidden on production path)。

#### Scenario: EnsureGoal failure is logged
- GIVEN `EnsureGoal` returns an error (e.g. disk persist failure)
- WHEN `ProcessMessage` processes a non-skip intent
- THEN structured `slog.Warn` MUST include `session_id` and `err`
- AND the orchestrator SHOULD short-circuit or surface degraded behavior (not proceed as if goal exists)

---

### Requirement: D7-S2-A82 AwaitRunningChildren Feedback

`RunSessionTurnLoop` SHALL inspect `ResolveAwaiter.AwaitRunningChildren` return value and emit user-visible or structured feedback when children fail to reach terminal within budget.

#### Scenario: Await timeout surfaces feedback
- GIVEN a parent WorkItem with children stuck `InProgress`
- WHEN `AwaitRunningChildren` returns non-empty status indicating timeout or error
- THEN the session loop MUST emit a warning/error `EngineEvent` or increment await metric
- AND MUST NOT spin indefinitely without user-visible signal

---

### Requirement: D7-S2-A83 TurnState Handle Lifecycle

`TurnState.EndTurn` SHALL remove completed handles from `handles` map (or equivalent purge on session close) to prevent unbounded memory growth in long-running processes.

#### Scenario: EndTurn purges handle
- GIVEN `BeginTurn` succeeded for `sessionID`
- WHEN `EndTurn(sessionID)` is called
- THEN `handles[sessionID]` MUST be deleted or marked reclaimable
- AND a subsequent `WaitTurn` before next `BeginTurn` MUST return immediately (no stale channel)

---

### Requirement: D7-S2-A84 ItemPipeline Phase-Tree Consistency

`ItemPipelineRunner` SHALL NOT ignore `SetRoundPhase` errors. Phase transition failures MUST be logged with `session_id`, `work_item_id`, `phase`, and `err`; span SHOULD record `phase_write_failed=true`.

#### Scenario: SetRoundPhase failure is observable
- GIVEN `SetRoundPhase` returns error (e.g. invalid transition)
- WHEN ItemPipeline advances Observe→Plan→Execute
- THEN `slog.Warn` MUST be emitted
- AND Jaeger span for the round MUST NOT claim success phase if tree write failed

---

### Requirement: D7-S15-A42 Resolve Rollup Error Propagation

`workmodel.resolve` rollup paths (`SetUncertainty`, `UpdateStatus`, `SetNeedsRollup`, `ReopenForRollup`) on parent reevaluation SHALL log failures; P0 rollup gate paths SHOULD propagate error to caller.

#### Scenario: Parent status update failure logged
- GIVEN child WorkItem reaches terminal state
- WHEN `ReevaluateParentAfterChild` calls `UpdateStatus` and receives error
- THEN structured warn MUST include parent_id and child_id
- AND parent MUST NOT appear `Completed` if status write failed

---

### Requirement: D7-S16-A77 ChildDownlink Schema ExpectedReturn

`ChildDownlink` fallback ExpectedReturn MUST use machine-readable schema tag (`<deliverable_schema>…</deliverable_schema>` or `DefaultChildExpectedReturn`), NOT natural-language `"Deliverable aligned with directive: " + directive`.

#### Scenario: Fallback uses schema tag
- GIVEN child spec with empty ExpectedReturn
- WHEN ChildDownlink builds downlink payload
- THEN ExpectedReturn MUST contain `<deliverable_schema>` tag
- AND MUST NOT contain raw directive prose as the sole return contract

---

### Requirement: D7-S14-A48 Escape Arbitrator Resource Safety

`LLMArbitrator` timeout paths SHALL use `time.NewTimer` with `Stop`, cancelable context for `Generate`, and MUST NOT leak goroutines when parent ctx or timeout fires first.

#### Scenario: LLM timeout does not leak goroutine
- GIVEN `invokeLLMWithTimeout` with 1ms timeout and blocking Generate
- WHEN timeout fires before Generate returns
- THEN the call MUST return within bounded time
- AND repeated invocations MUST NOT grow goroutine count without bound (leak test)

---

### Requirement: D7-S9-A33 MUPS Execute Early Cancel

`mups/execute` parallel exploration (`ExplorationChannel`, `ScenarioChannel`) SHALL honor `ctx.Done()` and exit `wg.Wait` early, marking in-flight artifacts as cancelled/unknown rather than blocking until all probes complete.

#### Scenario: Context cancel stops wait
- GIVEN parallel exploration with 5 steps and slow runners
- WHEN parent `ctx` is cancelled after 1 step completes
- THEN the channel MUST return within bounded time
- AND incomplete steps MUST NOT block slot release indefinitely

---

### Requirement: D7-S2-A85 Turn Ingress Serialization Semantics

`TurnState` ingress contract SHALL document and implement: either (a) `BeginTurn` synchronously in `ProcessMessage` after `WaitTurn`, or (b) explicit queue/reject when second message arrives before first `BeginTurn`. Current "reject on second BeginTurn" behavior is acceptable if documented; silent `WaitTurn` no-op when no handle is NOT sufficient for queue semantics.

#### Scenario: Second message while turn in flight
- GIVEN session has active turn (BeginTurn without EndTurn)
- WHEN second `ProcessMessage` arrives
- THEN caller MUST receive `TurnInProgressError` or equivalent user-visible "busy" response
- AND MUST NOT start a second `RunSessionTurnLoop` goroutine for same session

---

## MODIFIED

### Requirement: D7-S1 WorkTree SetStore Concurrency

`WorkTree.SetStore` MUST hold `mu.Lock()` during store replacement, OR documentation MUST state "bootstrap-only, before any concurrent access" with CI guard.

#### Scenario: SetStore under lock
- GIVEN concurrent `Get`/`EnsureGoal` on WorkTree
- WHEN `SetStore` is called (if allowed at runtime)
- THEN operation MUST be synchronized with other tree mutations
- OR static analysis / lint MUST fail if `SetStore` appears outside bootstrap

---

### Requirement: D7-S14 Escape Tactical Prompt Externalization

Escape `LLMArbitrator` JSON format instructions MUST NOT be hardcoded Chinese prose in Go source (`arbitrator.go`). MUST use i18n registry or `format_hints` schema tag pattern consistent with DM-20260630-012.

#### Scenario: No tactical Chinese in arbitrator Go source
- GIVEN `grep` scan of `escape/arbitrator.go` production strings
- WHEN checking against orchestration-no-tactical-hardcoding rule
- THEN multi-line Chinese JSON template MUST NOT appear in const or string literal
- AND schema hint MUST be loadable from i18n key

---

### Requirement: D7-S16 StrategicPlan Appendix Externalization (P2)

`strategic_plan_proposer.go` proposer appendix strings (`strategicPlanAppendixZH` etc.) SHOULD migrate to i18n; Go retains JSON schema one-liner + `ValidateStrategicPlan` invariants only.

---

## REMOVED

(None)

---

## L5 Test Points (register in t-registry at S4)

| ID | Description | Priority |
|----|-------------|----------|
| D7-S2-A80-T01 | PerInvocationEmit two-session race test | P0 |
| D7-S2-A80-T02 | Custom executor emit via opts | P1 |
| D7-S3-A84-T01 | OnRelease hook count invariant | P0 |
| D7-S2-A81-T01 | EnsureGoal failure slog | P1 |
| D7-S2-A82-T01 | AwaitRunningChildren warning emit | P1 |
| D7-S2-A83-T01 | EndTurn handle purge | P1 |
| D7-S2-A84-T01 | SetRoundPhase failure span attr | P1 |
| D7-S15-A42-T01 | Resolve parent update warn | P1 |
| D7-S16-A77-T01 | ChildDownlink schema fallback | P1 |
| D7-S14-A48-T01 | Arbitrator timeout no goroutine leak | P1 |
| D7-S9-A33-T01 | MUPS execute ctx cancel early exit | P2 |
| D7-S2-A85-T01 | Turn in progress reject second message | P2 |
| D7-S1-A80-T01 | SetStore mutex or bootstrap-only lint | P2 |
| D7-S14-A49-T01 | Arbitrator no tactical Chinese scan | P2 |
| D7-S16-A78-T01 | StrategicPlan appendix i18n | P2 |
