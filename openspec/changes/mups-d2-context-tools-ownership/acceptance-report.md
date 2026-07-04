---
demand-id: DM-20260704-001
title: MUPS 上下文与工具决策归 D2 — 验收报告
executor: Agent S4-Gate
environment: local dev (go test -race)
date: 2026-07-04
verdict: ACCEPTED
---

# 验收报告：MUPS 上下文与工具决策归 D2

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260704-001 |
| Change ID | mups-d2-context-tools-ownership |
| 执行人 | Agent S4-Gate |
| 测试环境 | local dev |
| 执行日期 | 2026-07-04 |
| 总体结论 | **ACCEPTED** |

## 2. L5 测试点验证结果

| L5 ID | 描述 | 优先级 | 状态 | 证据 |
|-------|------|--------|------|------|
| L5-D2-MUPS-01 | Observe 节点 MaterializeForMUPS，Tools 空 + obs appendix | P0 | PASS | `mups_materializer_test.go`, `llm_observation_proposer_test.go` |
| L5-D2-MUPS-02 | Plan 节点 phase prompt + D7 无 appendix 常量 | P0 | PASS | `phase_prompts_test.go`, `strategic_plan_proposer.go` |
| L5-D2-MUPS-03 | Execute 7 步 filter pipeline + MaterializeForMUPS | P0 | PASS | `filter_pipeline_test.go`, `workitem_executor_test.go` |
| L5-D2-MUPS-04 | D7 不 import enforce/tools/filter | P0 | PASS | `d7_no_tool_filter_test.go` |
| L5-D2-MUPS-05 | ToolRound channel router + PromptPressure | P1 | PASS | `enforce/toolround/channel_router_test.go` |

### 统计

| 优先级 | 总数 | 通过 | 失败 | 跳过 |
|--------|------|------|------|------|
| P0 | 17 | 17 | 0 | 0 |
| P1 | 2 | 2 | 0 | 0 |

## 3. T 点执行结果（19 新 T）

全部 IMPLEMENTED。单元测试：

```text
go test -race ./internal/layers/contextengine/...     PASS
go test -race ./internal/layers/orchestration/sessionorchestrator/...  PASS
go test -race ./internal/lint/layer/...               PASS
go test -race ./internal/...                          PASS
```

## 4. 领域文档同步（S5 门禁）

| 文件路径 | 变更摘要 | 已更新 |
|----------|----------|--------|
| `openspec/specs/d2-context-engine/d7-boundary.md` | 新增 §10 MUPS D2 上下文归属 | ✅ |
| `openspec/specs/d2-context-engine/t-registry.md` | D2-S15-A90..A92 + D2-S18-A90 (14 T) | ✅ |
| `openspec/specs/d7-orchestration/t-registry.md` | D7-S2-A90..A91 (5 T) | ✅ |
| `openspec/changes/mups-d2-context-tools-ownership/design.md` | 架构 SoT（change 包内） | ✅ N/A |

## 5. 遗留风险

| 风险 | 影响 | 规避方案 |
|------|------|---------|
| D7 `mups/execute/toolchannel/` 仍保留（D2 已复制核心逻辑） | 双份代码可能漂移 | S7 归档时 thin wrapper 或删除 D7 副本 |
| B.4.4 并行 old/new router diff 未实现 | P1 观测缺口 | D2 toolround 默认 ModeShadow；后续 release 切 ModeEnforce |
| FastPath RunTurn 仍走 PrepareForTurn，未复用 filter pipeline | Turn 与 MUPS 工具集可能不一致 | follow-up change 共享 filter_pipeline 内部函数 |

## 6. 结论

DM-20260704-001 全部 P0 测试点通过，19 新 T 点已实现并登记。D2 通过 `MaterializeForMUPS` 统一负责 MUPS LLM 节点的 context + tools 决策，D7 边界 lint 生效。验收 **ACCEPTED**，可进入 S6 合入与 S7 归档。
