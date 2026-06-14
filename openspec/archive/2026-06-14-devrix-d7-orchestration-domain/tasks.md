# D7 Orchestration Domain — 任务分解（Review R1）

**Demand:** DM-20260613-001
**Status:** S3 Planning — 文档就绪，待二次评审，**不含代码**
**Last Updated:** 2026-06-14

---

## 任务与 DSAFT 映射

| Task ID | 描述 | Phase | DSAFT | T 测试点 | 优先级 |
|---------|------|-------|-------|----------|--------|
| T-D7-A01 | 编写 demand.md + review-r1.md + tasks.md | A | — | — | P0 |
| T-D7-A02 | 更新 d7-domain/spec/design/t-registry（R1 澄清） | A | D7 | — | P0 |
| T-D7-B01 | 创建 `internal/layers/d7/` 包骨架 + types | B | D7-IDENTITY | D7-IDENTITY-T01 | P0 |
| T-D7-B02 | 定义 contracts：D2Executor, D4Executor, SessionOrchestrator | B | D7-S2 | — | P0 |
| T-D7-B03 | orchestration/ → d7/ re-export 桥接 | B | D7-S3/S4 | D7-S3-T01…T10 | P0 |
| T-D7-B04 | 实现 `orchestration.d7_enabled` 配置解析 | B | D7-D1 | D7-D1-T01 | P0 |
| T-D7-C01 | ClassifyIntent 规则引擎 + command-first | C | D7-S5-A01 | D7-S5-T03, D7-S5-T06 | P0 |
| T-D7-C02 | ProcessMessage + RouteByIntent + FastPath proxy | C | D7-S2-A01 | D7-S2-T01, T02a, T02b | P0 |
| T-D7-C03 | OrchestratePath 路由至 PlanMode 或 Wave（矩阵） | C | D7-S2-A01-F03 | D7-S2-T03 | P0 |
| T-D7-D01 | HandleInterrupt：Wave→D4→Process→stopped→TaskCancel 链路 | D | D7-S2-A03 | D7-S2-T04, D7-S2-T05 | P0 |
| T-D7-D02 | D1 Gateway 切换至 D7（feature flag） | D | D1-S1 | D7-D1-T01 | P0 |
| T-D7-E01 | TaskManager 迁入 d7/workmodel（行为不变） | E | D7-S1-A02 | D7-S1-T01…T05 | P0 |
| T-D7-E02 | D2 loop 移除 delegate hooks / queue | E | D2 Thin | D7-THIN-T01…T04 | P0 |
| T-D7-E03 | BackgroundRun 标记 D7-S1 托管（适配层） | E | D7-S1 | D7-S1-T07 | P1 |
| T-D7-F01 | Migration 4 组合回归 + acceptance-report | F | — | 全 P0 T | P0 |
| T-D7-G01 | S5-P3 SynthesizeTaskGraph（v1.1） | G | D7-S5-A02 | D7-S5-T04 | P1 |
| T-D7-G02 | D7-S1 CreateWorkPlan + DAG 校验（v1.1） | G | D7-S1-A01 | D7-S1-T06 | P1 |

---

## Phase 依赖

```
A (文档) → B (骨架) → C (分类+入口) → D (中断+切换) → E (迁移+瘦身) → F (验收)
                                                              ↓
                                                         G (v1.1 增强)
```

**约束：** Phase D 与 E 应在同一 release 或相邻 release 完成，避免 `d7_enabled=true` 且 loop 仍含 hooks。

---

## 不在本 change 内

- S5-P4 auto_detect
- PlanTask ↔ BackgroundRun 存储合并
- D6 校验规则扩展
- `orchestration.task.store_dir` 第二配置源

---

## 评审关注项

1. Phase C 的 OrchestratePath 在 v1.0 无 SynthesizeTaskGraph 时，是否仅路由到 PlanMode？
2. T-D7-E03 是否与 DM-20260612-011 合并实施？
3. OQ-3：ClassifyIntent LLM 兜底是否推迟到 v1.1？
