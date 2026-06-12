# Design: 多 Agent 会话隔离 — Join 合并与 Session 元数据隔离

**Change ID:** devrix-multiagent-isolation
**Demand ID:** DM-20260611-005
**Status:** S3_Design
**Priority:** P1

## 1. Goal

In v1.0 the Fork/Join pair shares the parent's `*types.Session` directly
(DM-012 QueryLoop v6 shipped fork-message isolation, but metadata + snapshot
are still shared). The shared pointer is a real race surface under `-race`,
and Join has no contract for ordering or tool_call id collapse.

This change ships a copy-on-write `SessionView`, a Join that dedupes
tool_call ids, and a D6 probe that blocks regressions in CI.

## 2. Architecture

### 2.1 `sessionview` package

New package `internal/layers/multiagent/sessionview` owns the COW view.

```go
type View struct {
    id, createdAt, model, budget (shared read-only)
    mu, metadata, snapshot       (isolated, RW-locked)
}

func Fork(parent *types.Session) *View
func (v *View) SetMetadata / GetMetadata / SetSnapshot / Snapshot / MergeToParent
```

Why a new package (not in `factory`): `agent` and `factory` import each
other transitively. Hoisting the view type out of the cycle lets both
layers reference it without import restrictions.

### 2.2 `factory.AgentFactory.CreateWithView`

Additive method on `AgentFactory`. The original `Create(ctx, cfg, session)`
is preserved; it auto-creates a view internally. `CreateWithView` binds
a caller-supplied view (Wave Scheduler fresh-policy path, tests).

`Create` keeps the exact same signature as v1.0 — no D4 contract change.

### 2.3 `agent.Impl` — Fork path

`Fork()` now:

1. Builds `childView := sessionview.Fork(a.session)` before Create.
2. Calls `a.creator.Create(ctx, childCfg, a.session)` as before.
3. Casts the returned agent to `*Impl` and calls `AttachSessionView(childView)`.
4. Increments D5 metric `runtime.fork_session_view_total{policy="cow"}`.

The hot path does not lock beyond the `sessionview.View` internal RWMutex.

### 2.4 `agent.Impl` — Join path

`Join()` now:

1. Collects `child.GetMessages()` and `result.Messages` (unchanged).
2. Calls `a.dedupToolCallMessages(msgs)` which:
   - Walks the slice under `a.mu`.
   - Collapses messages whose `Metadata["call_id"]` is non-empty and
     has already been seen in any previous Join on this parent
     (stored in `a.joinedToolIDs`).
3. Appends the deduped slice to `a.messageBuffer`.

Per-child ordering is preserved; the spec only requires per-call_id
uniqueness, not a global completion-time sort. The completion timestamp
is captured on `finishedAt` (v2.0 hook reserves global sort for the
`Policy` switch).

### 2.5 Lifecycle capture of `tool_call` events

The run loop in `agent/lifecycle.go` now appends a `Message` for every
`tool_call` event whose `Metadata["call_id"]` is non-empty. This gives
Join a stable string key for dedup without touching D2/D3 contracts
(EngineEvent contract is unchanged).

### 2.6 D6 probe `session_isolation`

Deterministic, no LLM call. Inputs:
- `fork_count`, `join_count`, `metadata_writes`, `isolation_violations`

Outputs:
- `isolation_rate = 1 - violations/writes`
- `join_consistency = 1 iff fork_count == join_count`
- `metric_ok = 1 iff local D5 counter >= fork_count`
- `score = mean of the three`

CI gate target: `score >= 0.95` for the production bucket.

### 2.7 D5 metric `runtime.fork_session_view_total`

`metrics/multiagent.go` provides a `PolicyCounter` (one Counter per
policy label) registered against the standard `*Registry`. The
`multiagent/observability` package owns a local atomic counter
(source of truth) and exposes a `SetD5Sink` hook so the D5 init path
can mirror bumps into the global registry without changing Fork's
hot-path code.

## 3. Files Touched

| File | Change |
|------|--------|
| `internal/layers/multiagent/sessionview/sessionview.go` | new |
| `internal/layers/multiagent/sessionview/sessionview_test.go` | new |
| `internal/layers/multiagent/observability/metrics.go` | new |
| `internal/layers/multiagent/observability/metrics_test.go` | new |
| `internal/layers/observability/metrics/multiagent.go` | new |
| `internal/layers/observability/metrics/multiagent_test.go` | new |
| `internal/layers/evolution/eval/session_isolation_probe.go` | new |
| `internal/layers/evolution/eval/session_isolation_probe_test.go` | new |
| `internal/layers/multiagent/agent/agent.go` | +View field, +FinishedAt, +Attach/SessionView, +New auto-fork view |
| `internal/layers/multiagent/agent/forkjoin.go` | Fork binds view; Join dedups; metric bump |
| `internal/layers/multiagent/agent/lifecycle.go` | capture tool_call as message |
| `internal/layers/multiagent/agent/forkjoin_isolation_test.go` | new (L5-4-3-01/02/03) |
| `internal/layers/multiagent/factory/factory.go` | +CreateWithView |
| `internal/layers/multiagent/factory/factory_test.go` | +CreateWithView test |
| `internal/shared/types/session.go` | +Metadata field (additive) |

## 4. Backward Compatibility

| Layer | Risk | Mitigation |
|-------|------|-----------|
| D2 ProcessOverlay | none | EngineEvent type unchanged |
| D3 gateway | none | `Create` signature unchanged |
| D4 delegate service | none | uses `factory.AgentFactory.Create` (unchanged) |
| D4 builtin agents | none | don't call `CreateWithView` |
| D4 existing tests | none | `TestAgent_should_fork_join_with_isolated_buffers` and friends pass without modification |
| DM-007 Wave Scheduler | additive | opt-in via `CreateWithView` |

## 5. L5 Test Mapping

| L5 ID | Test | File |
|-------|------|------|
| L5-4-3-01 | `TestJoin_should_dedup_tool_call_ids` | `agent/forkjoin_isolation_test.go` |
| L5-4-3-02 | `TestFork_metadata_writes_should_not_pollute_parent_session` | `agent/forkjoin_isolation_test.go` |
| L5-4-3-03 | `TestFork_concurrent_three_children_should_join_consistently` | `agent/forkjoin_isolation_test.go` |
| L5-4-3-04 | `TestFork_should_share_readonly_fields` + `TestAgentFactory_CreateWithView_should_bind_view_to_agent` | `sessionview/sessionview_test.go`, `factory/factory_test.go` |
| L5-4-3-05 | `TestSessionIsolationProbe_is_registered` + scoring tests | `eval/session_isolation_probe_test.go` |
