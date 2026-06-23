# T-Registry Delta: D7 MUPS v4.3 Phase 4 — Verify 节点升格

**Change ID:** `devrix-d7-mups-v4-phase4-verify-promotion`
**Target:** `openspec/specs/d7-orchestration/t-registry.md` v3.11.0 → v3.12.0
**Created:** 2026-06-23

---

## ADDED Test Points（8 P0 T 点）

### D7-S10-A32: Verify 节点数据契约（G3-1）

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S10-A32-T01 | VerdictKind 4 态 typed enum + String/Parse/Marshal/Unmarshal（空字符串零值兼容） | D7-S10-A32-F01 | `internal/shared/types/verdict_test.go` | P0 |
| D7-S10-A32-T02 | AggregationStrategy 4 策略 + AggregateVerdicts 函数边界（空/单元素/同质）+ 4 策略实现（Weak/Strong/Majority/Threshold） | D7-S10-A32-F02 | `internal/layers/orchestration/workmodel/aggregate_verdicts_test.go` | P0 |

### D7-S10-A33: VerdictToExitReason + G8-1 修复

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S10-A33-T03 | VerdictToExitReason 4 Verdict → 4 ExitReason 映射 + SystemAnomaly 覆盖 + 14 ExitReason 8→14 扩展（向后兼容） | D7-S10-A33-F01 | `internal/layers/orchestration/turn/verdict_to_exit_reason_test.go` | P0 |
| D7-S10-A33-T04 | VerifyWithRetry parse failure → INDETERMINATE（G8-1 P0-3 修复，3 次重试 + 单次成功 + 全失败 边界） | D7-S10-A33-F02 | `internal/layers/orchestration/workmodel/verify_with_retry_test.go` | P0 |

### D7-S10-A34: Evidence + EvidenceExtractor

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S10-A34-T05 | Evidence struct 5 字段（Reason/Confidence/Counterexample/SourceRef/ExtractedAt）+ Validate + NewEvidence（必填字段 fail-fast） | D7-S10-A34-F01 | `internal/layers/orchestration/workmodel/evidence_test.go` | P0 |
| D7-S10-A34-T06 | EvidenceExtractor interface 2 方法（Extract + Validate）+ LLM 实现 + Stub 实现 | D7-S10-A34-F02 | `internal/layers/orchestration/workmodel/evidence_extractor_test.go` | P0 |

### D7-S10-A35: SystemAnomaly 异常聚合 + ObserveNode wiring

| T ID | 描述 | 归属 A/F | Test 位置 | Priority |
|------|------|----------|-----------|----------|
| D7-S10-A35-T07 | SystemAnomalyAggregator 阈值触发（AnomaliesCount ≥ Threshold + CatSystem/AnomaliesCount ≥ Ratio）+ RecordCatSystem + Reset | D7-S10-A35-F01 | `internal/layers/orchestration/workmodel/system_anomaly_test.go` | P0 |
| D7-S10-A35-T08 | ObserveNode wiring SystemAnomaly → FromVerifier + BuildUncertaintyCoordFromReport 集成（SystemAnomaly 强制 Value=0.95） | D7-S10-A35-F02 | `internal/layers/orchestration/workmodel/uncertainty_system_anomaly_test.go` | P0 |

---

## Statistics Delta

| 指标 | v3.11.0 | v3.12.0 (Phase 4) | 增量 |
|------|---------|-------------------|------|
| Total T | 147 | 155 | +8 |
| IMPLEMENTED | 147 | 155 | +8 |
| PLANNED | 0 | 0 | 0 |
| P0 | 114 | 122 | +8 |
| Scenarios D7-S10 | 0 | 4 | +4 |

---

## Revision History

| Version | Date | Change | PR |
|---------|------|--------|-----|
| v3.12.0 | 2026-06-23 | Phase 4 Verify 节点升格：8 P0 T 点 IMPLEMENTED (D7-S10-A32-T01/T02 + A33-T03/T04 + A34-T05/T06 + A35-T07/T08) | #172/#173/#174/#175 + #176 (S6 archive) |

---

## 关联

- 前置：Phase 3 PR-C1/C2 (Artifact + Channel)
- 后续：Phase 5 Learn 节点（PR-E1..E5）
- 设计稿：openspec/changes/devrix-d7-mups-v4-phase4-verify-promotion/{proposal,design,tasks}.md