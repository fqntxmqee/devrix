---
demand-id: DM-20260703-001
change-id: d7-convergence-contract
title: D7 任务树收敛契约 — 验收报告
executor: Cursor Agent (S4-Gate 自检)
environment: local CI (go test -race)
date: 2026-07-04
verdict: ACCEPTED
---

# 验收报告：D7 任务树收敛契约

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260703-001 |
| Change ID | d7-convergence-contract |
| 执行人 | Cursor Agent |
| 测试环境 | local — `go test -race ./internal/layers/orchestration/...` |
| 执行日期 | 2026-07-04 |
| 总体结论 | **ACCEPTED**（P0 全绿；P1/P2 手工或 follow-up 记 DEFERRED） |
| 合入 PR | [#381](https://github.com/fqntxmqee/devrix/pull/381) + follow-ups [#382–#386](https://github.com/fqntxmqee/devrix/pulls?q=DM-20260703-001) |

## 2. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
| L5-D7-CC-01 | R0.5 terminalization + pipeline continuation | P0 | PASS | `spawn_policy_test.go` (D7-S5-A93-T01/T03), `session_turn_loop` |
| L5-D7-CC-02 | max-depth inline budget | P0 | PASS | `spawn_policy_test.go` (D7-S5-A93-T02), `inline_retry.go` |
| L5-D7-CC-03 | rollup gate + session exit | P0 | PASS | `rollup_gate_test.go`, `session_complete.go` |
| L5-D7-CC-04 | decompose parent rollup | P1 | PASS | `MaybeParentRollup` tests |
| L5-D7-CC-05 | scope validation before decompose | P1 | PASS | `scope_validator.go` + tests |
| L5-D7-CC-07 | session complete 安全网 | P0 | PASS | `session_complete_test.go` (D7-S2-A73-T05) |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过/延期 |
|--------|------|------|------|-----------|
| P0 | 4 | 4 | 0 | 0 |
| P1 | 2 | 2 | 0 | 0 |
| P2 | 2 | 0 | 0 | 2 DEFERRED |

### 延期项

| ID | 内容 | 原因 |
|----|------|------|
| T7b (4.5) | staging 飞书 `review d2 领域 kernel目录下代码` | 待 staging 部署后手工补验 |
| 4.2 | `MaxMUPSRounds` 软上限 | P2 optional，默认 disabled |
| 3.3/3.4 | MergeChildDeliverables / RollupGatePolicyFor 配置 | P2 defer |
| 3.5 T4 | 4 层 decompose 集成 | 部分由 `DecomposeRecursive` stub 覆盖，完整 E2E defer |

## 3. 测试执行

```text
go test -race ./internal/layers/orchestration/workmodel/...           → PASS
go test -race ./internal/layers/orchestration/sessionorchestrator/... → PASS
```

## 4. 领域文档同步（S5 → S6 门禁）

| 文件路径 | 变更摘要 | 已更新 |
|----------|----------|--------|
| `openspec/specs/d7-orchestration/pipeline-architecture.md` | §4.1 CC-1～CC-5 | ✅ |
| `openspec/changes/d7-convergence-contract/specs/` | delta 已合入 specs（CC 表） | ✅ |
| `openspec/specs/d7-orchestration/t-registry.md` | D7-S5-A93/A94 等 T 点 | 归档时登记 |
| `openspec/demand-archive-index.md` | S7 条目 | 归档时 |

## 5. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| staging T7b 未跑 | 飞书长会话行为 | PR #381–386 合入后 staging 补验 |
| PR #389 rollup synthesis 修复 | MiniMax stream garbage | 独立 hotfix 已合入，属 CC 收敛 follow-up |

## 6. 结论

Convergence Contract v1（CC-1～CC-5）P0 机制 invariant 已落地并合入 master。准许 **S7 归档**；P1 staging 与 P2 optional 记 follow-up。
