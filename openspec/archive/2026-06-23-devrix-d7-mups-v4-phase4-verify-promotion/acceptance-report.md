# Acceptance Report — DM-20260623-002 (Phase 4 Verify 节点升格)

**Change ID:** `devrix-d7-mups-v4-phase4-verify-promotion`
**Demand ID:** DM-20260623-002
**PR Scope:** PR-D1 + PR-D2 + PR-D3 + PR-D4 (Verify 节点 8 P0 T 点)
**Acceptance Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 4 Verify 节点升格
**Status:** ✅ S5_Accepted

---

## 1. 验收范围

本报告验收 Phase 4 Verify 节点升格（PR-D1..PR-D4）的实现质量与设计一致性。

| 维度 | 范围 |
|------|------|
| **代码变更** | 4 PR / 10 新文件 / 1 修改文件 / +1133/-0；Verify 节点 4 类核心契约落地 |
| **测试变更** | 50 tests / 0 race detector warnings / go vet clean |
| **文档变更** | spec.md v4.3.0→v4.4.0 (D7-S10-A32/A33/A34/A35 Requirement) + t-registry.md v3.11.0→v3.12.0 (T147→155, P0 114→122) |
| **G8-1 修复** | VerifyWithRetry parse failure → INDETERMINATE (NOT error, NOT FAIL) |
| **不做的事** | Phase 5 Learn 节点不动 / 与 docs/error-handling.md 不交叉 / 不引入新 LLM 模型 |

## 2. 验收标准达成

### 2.1 P0 验收（AC1-AC12）

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC1** | VerdictKind 4 态 typed enum + String/Parse/Marshal/Unmarshal + 零值兼容 | ✅ PASS | D7-S10-A32-T01 IMPLEMENTED；`internal/shared/types/verdict_test.go` 10/10 PASS |
| **AC2** | AggregationStrategy 4 策略 + AggregateVerdicts 函数 + 边界（空/单/同质） | ✅ PASS | D7-S10-A32-T02 IMPLEMENTED；`workmodel/aggregate_verdicts_test.go` 20/20 PASS |
| **AC3** | orchtypes type alias `type VerdictKind = types.VerdictKind` 避免 import cycle | ✅ PASS | `orchtypes/uncertainty_coord.go` 重新导出 4 const aliases |
| **AC4** | VerdictToExitReason 4 Verdict → 4 ExitReason 映射 + SystemAnomaly 覆盖 | ✅ PASS | D7-S10-A33-T03 IMPLEMENTED；`turn/verdict_to_exit_reason_test.go` 14/14 PASS |
| **AC5** | ExitReason 8 → 14 扩展 + 6 新 enum + ParseExitReason + AllExitReasons | ✅ PASS | `turn/orchestrator.go` 14 ExitReason + ParseExitReason 函数 |
| **AC6** | VerifyWithRetry parse failure → INDETERMINATE（G8-1 P0-3 修复） | ✅ PASS | D7-S10-A33-T04 IMPLEMENTED；`workmodel/verify_with_retry_test.go` 10/10 PASS |
| **AC7** | DefaultMaxParseRetries = 3 + RetryCount 记录尝试次数 | ✅ PASS | `workmodel/verify_with_retry.go:60` const + struct field |
| **AC8** | Evidence struct 5 字段 + Validate + NewEvidence 必填 fail-fast | ✅ PASS | D7-S10-A34-T05 IMPLEMENTED；`workmodel/evidence_test.go` 10/10 PASS |
| **AC9** | EvidenceExtractor interface 2 方法 + LLM + Stub 实现 | ✅ PASS | D7-S10-A34-T06 IMPLEMENTED；`workmodel/evidence_extractor_test.go` 11/11 PASS |
| **AC10** | SystemAnomalyAggregator 阈值触发 + RecordCatSystem + Reset | ✅ PASS | D7-S10-A35-T07 IMPLEMENTED；`workmodel/system_anomaly_test.go` 9/9 PASS |
| **AC11** | AnomalyCategory interface + observationAdapter 避免 import cycle | ✅ PASS | `workmodel/system_anomaly.go` + `orchtypes/system_anomaly_wiring.go` 跨包 adapter 模式 |
| **AC12** | ObserveNode wiring BuildUncertaintyCoordFromReport + SystemAnomaly 强制 Value=0.95 | ✅ PASS | D7-S10-A35-T08 IMPLEMENTED；`orchtypes/system_anomaly_wiring_test.go` 9/9 PASS |

### 2.2 测试与质量

| 项 | 目标 | 实际 | 状态 |
|----|------|------|------|
| 单元测试 PASS | 100% | 50/50 PASS (含 race) | ✅ PASS |
| 新增 P0 T | 8 | 8 (D7-S10-A32-T01/T02 + A33-T03/T04 + A34-T05/T06 + A35-T07/T08) | ✅ PASS |
| go vet | 0 issue | 0 issue | ✅ PASS |
| go build | 0 error | 0 error | ✅ PASS |
| go test -race | 0 warning | 0 warning (21 个 orchestration 包) | ✅ PASS |
| layer-lint | 0 violation | 0 violation | ✅ PASS |
| import cycle | 0 cycle | AnomalyCategory interface + observationAdapter 模式避免 | ✅ PASS |

### 2.3 跨域一致性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 与 `shared/types.VerdictKind` 复用 | ✅ PASS | 跨域类型上提 shared/types（Phase 3 PR-C1 precedent）+ orchtypes type alias |
| 与 Phase 2 PR-A1 UncertaintyCoord.FromVerifier 兼容 | ✅ PASS | FromVerifierTyped(verdict, confidence, reason, systemAnomaly) 扩展为可选 4 参数；Phase 2 调用方零修改 |
| 与 Phase 2 PR-A1 UncertaintyReport.Anomalies 兼容 | ✅ PASS | EvaluateSystemAnomaly 直接消费 UncertaintyReport.Anomalies（CatSystem + ObsDeviation subset） |
| 与 Phase 2 PR-B1 Plan 反向追溯链路 | ✅ PASS | SourceObservationIDs 链路由 UncertaintyReport.Anomalies → Plan → Verify（PR-D4 入口预留） |
| 与 Phase 3 PR-C1 Artifact.SourcePlanID 兼容 | ✅ PASS | VerifierOutput.SourceID 字段添加（PR-D3）；Phase 3 调用方零修改 |
| ExitReason 向后兼容 | ✅ PASS | 8 既有 const 字符串不变（natural/max_turns/aborted_*/repeated_tool/tool_failure/token_diminishing）+ 6 新增 |
| SentinelError 模式 | ✅ PASS | sharederrors.WithCode 模式与 Phase 1/2/3 一致；ORCH_COORD_VERDICT_7004 复用 |

### 2.4 SystemAnomaly 行为正确性

| 场景 | 触发条件 | UncertaintyCoord.Value | ExitReason |
|------|---------|------------------------|------------|
| 3+ CatSystem ObsDeviation (≥ Threshold 3) AND ratio ≥ 0.5 | ✅ true | 0.95 (override) | system_anomaly |
| 3 CatBusiness + 1 CatSystem (33% ratio) | ❌ false | VerdictPass baseline | natural |
| 2 anomalies (below threshold) | ❌ false | VerdictPass baseline | natural |
| 4 anomalies, half CatSystem (50% boundary) | ✅ true (boundary inclusive) | 0.95 (override) | system_anomaly |
| Empty report | ❌ false | VerdictPass baseline | natural |

## 3. 实施质量

### 3.1 PR 信息

| PR | URL | 文件数 | 代码行数 | 风险等级 |
|----|-----|--------|----------|---------|
| PR-D1 | [#170](https://github.com/fqntxmqee/devrix/pull/170) | 4 (verdict.go/verdict_test.go/aggregate_verdicts.go/aggregate_verdicts_test.go + 1 modified uncertainty_coord.go) | +97 +145 +346 +339 / -0 | Low |
| PR-D2 | [#171](https://github.com/fqntxmqee/devrix/pull/171) | 4 NEW (orchestrator 改 + verdict_to_exit_reason + verify_with_retry + uncertainty_coord 改) | +92 +50 +96 +13 / -0 | Low |
| PR-D3 | [#172](https://github.com/fqntxmqee/devrix/pull/172) | 3 NEW (evidence + evidence_extractor + verify_with_retry 改) | +93 +153 / +10 | Low |
| PR-D4 | [#173](https://github.com/fqntxmqee/devrix/pull/173) | 4 NEW (system_anomaly + system_anomaly_wiring + 2 test) | +130 +76 / +180 +99 | Low |
| **汇总** | 4 PR / 10 NEW files / 3 MODIFIED | **+1133/-0** (含 50 tests) | **Low** |

### 3.2 关键修复

**G8-1 P0-3 修复**：`ParseVerifierOutputWithRetry(raw, maxRetries)` 在 3 次 parse 失败后返回 `VerifierOutput{ParsedKind: VerdictIndeterminate, Confidence: 0, RetryCount: 3}`，**不返回 error**。这样 verifier 暂时性网络抖动或输出格式异常被分类为 INDETERMINATE（高不确定性需人工复核），而不是硬错误或 FAIL。

修复前：parse failure → error → orchestrator 误判为 FAIL
修复后：parse failure → INDETERMINATE → ExitReasonVerifierAbstain → 人工复核

### 3.3 跨域类型上提

Phase 4 PR-D1 把 `VerdictKind` 从内联字符串 switch 升级为 typed enum，并上提到 `shared/types/verdict.go`（与 Phase 3 PR-C1 SideEffectStatus precedent 一致）。`orchtypes/uncertainty_coord.go` 通过 type alias `type VerdictKind = types.VerdictKind` 重新导出，避免 import cycle。

## 4. 后续依赖

Phase 4 闭环后，Phase 5 Learn 节点（PR-E1..E5）的前置依赖全部就位：
- VerdictKind 4 态（PR-D1）：Learn 节点学习算法输入
- Verdict + Evidence 数据契约（PR-D2/PR-D3）：Learn 节点正向反馈信号
- SystemAnomaly 触发（PR-D4）：Learn 节点信誉先验更新触发器

## 5. 关联

- **前置依赖**：`devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001) PR-A1 + `devrix-d7-mups-v4-phase2-plan` (DM-20260623-001-PRB1) PR-B1 + `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C1 + `devrix-d7-mups-v4-phase3-channels` (DM-20260625-001-PRC2) PR-C2
- **后续依赖**：Phase 5 Learn 节点（PR-E1..E5）强依赖本 PR 的 VerdictKind + Evidence + SystemAnomaly 数据契约
- **设计稿**：`openspec/changes/devrix-d7-mups-v4-phase4-verify-promotion/design.md`
- **方法论**：doc 35 §三.4 (Verify 节点方法论) + doc 17 (L2 verifier) + doc 18 (L1 ExitReason)
- **OpenSpec 归档**：`openspec/archive/2026-06-23-devrix-d7-mups-v4-phase4-verify-promotion/`