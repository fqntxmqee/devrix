# Tasks: D7 MUPS Frame Delta Phase 1+2 spans 端到端触发 (testutil_only)

**Status:** S7_Archived (2026-07-08)
**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段)
**Status legend**: PLANNED / IMPLEMENTED

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| L5-MUPS-FD-6 | IMPLEMENTED (PR #466) | testutil Phase 1+2+3 span 触发链: `SequenceLLMStub.FrameDeltaInject` callback (per-Stream-call `func(idx int) FrameDelta`) + `LastFrameDelta atomic.Pointer[interfaces.FrameDelta]` (most-recent-wins) + `SeedPriorExecContext(t, stack, sessionID, workItemID, artifactSummary)` helper (mutates `wi.LastRound.ArtifactSummary` via `stack.TaskManager.Tree().ApplyPipelineRound` + t.Cleanup restore) + `FormatConvergenceRate` util (strconv %.4f) + 5-cycle e2e `TestIntegration_D7FrameDelta_Phase1And2SpanTrigger` (20 stub responses × obs_uncertainty JSON 旁路 observational_answer fast-path) + AC4 testutil-only 文档化 | `tests/testutil/d7_frame_delta_helpers_test.go::TestSequenceLLMStub_FrameDeltaInjectCallback` + `tests/testutil/d7_frame_delta_helpers_test.go::TestFormatConvergenceRate` + `tests/integration/d7/d7_frame_delta_e2e_test.go::TestIntegration_D7FrameDelta_Phase1And2SpanTrigger` + `tests/integration/d7/d7_frame_delta_e2e_test.go::TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` |
| AC4 callback invariance | IMPLEMENTED (PR #466) | `FrameDeltaInject` callback fires on every Stream call; `LastFrameDelta` holds the most-recent-wins value via atomic.Pointer (concurrent Stream goroutine-safe); "testutil only" doc invariant on `SequenceLLMStub` | `tests/testutil/d7_frame_delta_helpers_test.go::TestSequenceLLMStub_FrameDeltaInjectCallback` + `tests/testutil/d7_llm_stub.go` docstring |

**Total**: 2 L5/T-points (L5-MUPS-FD-6 + AC4 sub-invariant), all IMPLEMENTED.

**Scope discipline**: testutil-only — 0 production code 修改 (与 sibling DM-20260706-004 严格隔离). Phase 1+2 e2e baseline ≥ 1 span 在 Phase 2 production wiring (sibling) 合入后提升至 ≥ 5 spans。

**S7 Archive Note (2026-07-08)**: PR #466 squash merged; verify-archive.sh 待执行 (per acceptance-report.md)。
