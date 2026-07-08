# Tasks: D7 Phase 2 frame_delta production wiring

**Status:** S7_Archived (2026-07-08)
**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段)
**Status legend**: PLANNED / IMPLEMENTED

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| L5-MUPS-FD-7 | IMPLEMENTED (PR #467) | Phase 2 production wiring: `ItemPipelineRunner.Run()` 在 Observe phase 之前构造 `prevExecCtx = &WorkItemExecContext{Item: item, Tasks: r.Tasks}`;`observeWorkItem` 函数签名 +1 参数 `prevExecCtx *WorkItemExecContext` (插入在 tasks 与 proposer 之间);`mergeProposedObservations` 函数签名 +1 参数 (插入在 tm 与 prior 之间);line 257 `buildObserveSignalInput(sessionID, item, tm, nil)` 替换为 `prevExecCtx` 上游传参 → Round 2+ `BuildObservePriorDelta` 返回 non-zero `FrameDelta.PriorArtifactSummary` → `D7_Observe_PriorDelta_Inject` span 在 production 真实触发 | `internal/layers/orchestration/sessionorchestrator/observation_proposer.go::mergeProposedObservations` (line 246-276 + signature + param) + `internal/layers/orchestration/sessionorchestrator/item_observe.go::observeWorkItem` (signature +1 param) + `internal/layers/orchestration/sessionorchestrator/item_pipeline.go::Run()` (line ~256-260 prevExecCtx construction) |
| Round-2 unit test AC1 | IMPLEMENTED (PR #467) | `TestObserveWorkItem_NonFirstRoundPopulatesPriorArtifactSummary`: Round 2+ scenario with `wi.LastRound.ArtifactSummary` populated; capture-proposer records `ObserveSignalInput.PriorArtifactSummary` populated from prev round; truncates to 80 chars max | `internal/layers/orchestration/sessionorchestrator/observation_proposer_test.go::TestObserveWorkItem_NonFirstRoundPopulatesPriorArtifactSummary` |
| First-round invariant AC2 | IMPLEMENTED (PR #467) | `TestObserveWorkItem_FirstRoundLeavesPriorArtifactSummaryEmpty`: invariant gate — `prevExecCtx == nil` OR `prevExecCtx.Item.LastRound == nil` → `in.PriorArtifactSummary` stays empty (BuildObservePriorDelta zero-value branch + prior_delta_empty span fires) | `internal/layers/orchestration/sessionorchestrator/observation_proposer_test.go::TestObserveWorkItem_FirstRoundLeavesPriorArtifactSummaryEmpty` |

**Total**: 1 L5/T-point + 2 unit-test invariants, all IMPLEMENTED.

**Scope discipline**: production code ONLY — 0 testutil 修改 (与 sibling DM-20260706-001 严格隔离). 修复 sibling DM-20260706-001 S3-Gate codex review (2026-07-08 BLOCKED) 拆分的 Phase 2 production wiring gap。

**S7 Archive Note (2026-07-08)**: PR #467 squash merged; verify-archive.sh 待执行 (per acceptance-report.md)。
