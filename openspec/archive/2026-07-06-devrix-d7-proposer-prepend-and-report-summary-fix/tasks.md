# Tasks: D7 Proposer UserContextPrepend + ReportSummary 信息密度修复

**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md`
**Status legend**: PLANNED / IMPLEMENTED

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A121-T01 | IMPLEMENTED | `uncertaintyReportSignature(anomalyCount)` 签名扩展为 `(report UncertaintyReport)`,扫描 `report.Observations`,序列化 ObsUncertainty.Question + ObsDeviation.Statement,strength ≥ 0.7 阈值过滤;intent=<kind> 永远保留作为第一段(向后兼容) | `internal/layers/orchestration/sessionorchestrator/deliverable_execute.go::uncertaintyReportSummary` + `deliverable_execute_test.go::TestUncertaintyReportSummary_*` (5 tests) |
| D7-S5-A121-T02 | IMPLEMENTED | `LLMObservationProposer.ProposeObservations` msgs 改为走 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)`(PR #449 Fix B 第 1 处) | `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go:55` + `llm_observation_proposer_test.go::TestLLMObservationProposer_RoutesUserContextPrepend` |
| D7-S5-A121-T03 | IMPLEMENTED | `LLMStrategicPlanProposer.ProposeStrategicPlan` msgs 改为走 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)`(PR #449 Fix B 第 2 处) | `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:403` + `strategic_plan_proposer_usercontext_test.go::TestLLMStrategicPlanProposer_RoutesUserContextPrepend` |
| D7-S5-A121-T04 | IMPLEMENTED | `LLMIntentSegmenter.SegmentRequest` 新增 `UserContextPrepend map[string]string` 字段(forward-compatible nil 零值) + `Segment()` msgs 走 `messagesForLLMInvoke(msgs, req.UserContextPrepend)`(PR #460 跨域 debt 修复) | `internal/layers/orchestration/sessionorchestrator/intent_segmenter.go` SegmentRequest + Segment() + `intent_segmenter_llm_test.go::TestLLMIntentSegmenter_RoutesUserContextPrepend` |
| D7-S5-A121-T05 | IMPLEMENTED | `scripts/check-d7-d3-prepend-boundary.sh` CI guard:扫描 `internal/layers/orchestration/sessionorchestrator/` 内所有 `LLMInvoker.InvokeStream` / `InvokeNonStream` 调用点,验证前 30 行已调 `messagesForLLMInvoke`,allow-list `semantic_verifier_default.go`(PR #450 CI defense) | `scripts/check-d7-d3-prepend-boundary.sh` (NEW) + 6/6 InvokeStream call sites 通过 + 1 allow-list |

**Total**: 5 T-points, all IMPLEMENTED. PR #449 (T01-T03) + PR #450 (T05) + PR #460 (T04 跨域 debt).

## 跨域 wire 一致性矩阵

| Change | New InvokeStream call site | 是否走 messagesForLLMInvoke | CI guard | 跨域 debt |
|--------|--------------------------|-----------------------------|----------|-----------|
| DM-20260706-008 PR #449 | Observe (LLMObservationProposer) + Plan (StrategicPlanProposer) | ✅ 通过 PR #449 修复 | N/A(PR #449 之前没 guard) | — |
| DM-20260706-008 PR #450 | (无新 call site;Execute / Turn 早已 wired) | ✅ 全部通过 | ✅ CI guard 新建 | — |
| DM-20260707-001 PR #452 | IntentSegmenter (PR-A2 新增 5th call site) | ❌ 初期绕过 | ❌ CI guard FAIL | ✅ PR #460 修复 |

## 文件清单(实际归档)

| 文件 | 来源 | 状态 |
|------|------|------|
| `internal/layers/orchestration/sessionorchestrator/deliverable_execute.go` | PR #449 | MODIFIED |
| `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go` | PR #449 | MODIFIED |
| `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go` | PR #449 | MODIFIED |
| `internal/layers/orchestration/sessionorchestrator/intent_segmenter.go` | PR #460 | MODIFIED (forward-compatible UserContextPrepend 字段 + messagesForLLMInvoke wrap) |
| `internal/layers/orchestration/sessionorchestrator/deliverable_execute_test.go` | PR #449 | MODIFIED(+5 tests) |
| `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer_test.go` | PR #449 | MODIFIED(+1 test) |
| `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer_usercontext_test.go` | PR #449 | NEW(+1 test) |
| `internal/layers/orchestration/sessionorchestrator/intent_segmenter_llm_test.go` | PR #460 | MODIFIED(+1 test TestLLMIntentSegmenter_RoutesUserContextPrepend) |
| `scripts/check-d7-d3-prepend-boundary.sh` | PR #450 | NEW |