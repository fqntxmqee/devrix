# Tasks: devrix-multiagent-isolation (S4 Implementation)

| ID | Task | Status | Notes |
|----|------|--------|-------|
| T1 | sessionview package (Fork, View, MergeToParent) | done | `internal/layers/multiagent/sessionview/` |
| T2 | factory.CreateWithView additive method | done | Wave Scheduler hook |
| T3 | agent.Impl view field + AttachSessionView + FinishedAt | done | backward compat preserved |
| T4 | agent.Fork path uses sessionview.Fork + D5 metric | done | hot path is allocation-free |
| T5 | agent.Join tool_call dedup | done | per-parent joinedToolIDs map |
| T6 | lifecycle captures tool_call events as messages | done | call_id in Metadata |
| T7 | D5 metric runtime.fork_session_view_total{policy} | done | local atomic + D5 mirror |
| T8 | D6 SessionIsolationProbe (deterministic) | done | registered in init() |
| T9 | L5-4-3-01 ~ 05 tests + -race -count=3 | done | all green |

## 5. L5 Test Mapping

See `design.md` §5.

## 6. Risks & Mitigations (from proposal §6)

| Risk | Outcome |
|------|---------|
| Join sort change breaks append-order tests | Mitigation: dedup is per-call_id, append order preserved. All existing `agent_test.go` tests pass unchanged. |
| Metadata COW big-Session copy | Snapshot is shared by default; only fork-time snapshot is copied (one byte slice). |
| ForkSessionView and Wave Scheduler alignment | Add `CreateWithView` additive method; existing call sites unaffected. |
| Direct `*types.Session` deprecation | Field is additive; no deprecation in this change. |
