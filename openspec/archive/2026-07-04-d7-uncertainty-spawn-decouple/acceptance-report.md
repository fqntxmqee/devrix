---
demand-id: DM-20260704-001
change-id: d7-uncertainty-spawn-decouple
title: D7 MUPS 不确定性驱动 Spawn — 验收报告
executor: Cursor Agent (S4-Gate 自检)
environment: local CI (unit tests)
date: 2026-07-04
verdict: ACCEPTED
---

# 验收报告：D7 MUPS 不确定性驱动 Spawn（Deliverable 解耦）

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260704-001 |
| Change ID | d7-uncertainty-spawn-decouple |
| 执行人 | Cursor Agent |
| 测试环境 | local — `go test` workmodel + sessionorchestrator |
| 执行日期 | 2026-07-04 |
| 总体结论 | **ACCEPTED**（P0 全绿；P1 单测全绿；staging 手工项记 SKIP 待部署后补验） |

## 2. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
| L5-D7-U-01 | Partial + 高证据 + U 低 → RollupSynth 非 inline 耗尽 | P0 | PASS | `evidence_progress_test.go::TestSpawnPolicyEvaluator_CCU1_inlineNotEscalateWithEvidence`, `spawn_apply_rollup_test.go` |
| L5-D7-U-02 | U 高时 strategic `single` 被拒绝 | P0 | PASS | `strategic_plan_proposer_test.go::TestApplySingleModeUncertaintyGate_rejectsHighU` |
| L5-D7-U-03 | spawnRationale 区分 CC-1.2 vs R7 | P1 | PASS | `evidence_progress_test.go::TestSpawnRationale_CC12_notR7` |
| L5-D7-U-04 | Session complete salvage | P1 | PASS | `deliverable_format_test.go::TestExtractSessionDeliverable_SalvageFromWorkItemArtifact` |
| L5-D7-U-05 | alias registry + fence extract + verify JSON-body + U damp | P1 | PASS | `deliverable_findings_parse_test.go`, `deliverable_contract_verify_test.go`, `uncertainty_unified_test.go` |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过 |
|--------|------|------|------|------|
| P0 | 2 | 2 | 0 | 0 |
| P1 | 3 | 3 | 0 | 0 |

### 手动验收（T-ACC-2/3）

| ID | 内容 | 状态 | 说明 |
|----|------|------|------|
| T-ACC-2 | staging 飞书大 scope 探索 → decompose/rollup | SKIP | 待 PR 合入 staging 后补验 |
| T-ACC-3 | sess 类路径读文件+格式失败 → 有总结 | SKIP | 依赖 staging 部署；单测路径已覆盖 salvage + rollup synth |

## 3. 测试执行

```text
go test ./internal/layers/orchestration/workmodel/... -count=1       → PASS
go test ./internal/layers/orchestration/sessionorchestrator/... -count=1 → PASS
```

## 4. 领域文档同步（S5 → S6 门禁）

| 文件路径 | 变更摘要 | 已更新 |
|----------|----------|--------|
| `openspec/specs/d7-orchestration/spec.md` | 范式 4 CC-U 精简契约 | ✅ |
| `openspec/specs/d7-orchestration/uncertainty-spawn-contract.md` | CC-U1～U6 完整 delta | ✅ |
| `openspec/specs/d7-orchestration/pipeline-architecture.md` | §4.1 CC-U 交叉引用 | ✅ |
| `openspec/specs/d7-orchestration/t-registry.md` | D7-U-T01～T05 | ✅ |
| `openspec/specs/d7-orchestration/CHANGELOG.md` | 时间线条目 | ✅ |
| `openspec/demand-archive-index.md` | S7 归档条目 | 待 S7（合入后） |

## 5. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| staging 未跑 T-ACC-2/3 | 生产行为与单测偏差 | PR 合入后 staging 飞书补验 sess 类指令 |
| Rollup synth 增 latency | 单 WI 多一轮 synth | `MaxRollupRetries` + 证据双条件门控 |

## 6. 结论

Phase 1–5 实现与 P0/P1 自动化验收通过。Deliverable gate 已降级为呈现/提取层；Spawn 主信号回归 UncertaintyMean + 拓扑 + 证据进度。准许进入 **S6 交付**（PR 合入 `master`）；S7 归档在合入后执行。
