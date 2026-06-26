# Demand: WorkItem × ContextGraph 分层透传

**Demand ID:** DM-20260626-020
**Created:** 2026-06-26
**Reporter:** 架构 Review（WorkItem Pipeline v0.6 后续）
**Priority:** P1
**Sprint:** d7-workitem-context-graph

---

## 1. 背景

`devrix-d7-workitem-pipeline-unification`（PR #243）已落地 per-WorkItem MUPS Pipeline + SpawnPolicy 规则裁决。WorkTree 有层级与 `BlockedBy`，但 **Context 仍为 session 单桶**：

- 同层 decompose 兄弟默认应隔离，无 `ContextScope` 绑定
- `BlockedBy` 依赖链未自动映射为 Wave `ContextUpstream`
- 子 → 父仅 `LastRound` 结构化 bubble，叙事透传无契约

## 2. 目标

在 WorkTree 之上引入正交 **ContextGraph**（设计 SoT：`workitem-context-graph-design.md` v0.1.0）：

- 每个 WorkItem 绑定 `ContextScope`（CG1）
- 默认隔离，显式 `ContextLinkRecord`（CG2）
- LLM 提案 Link/Bubble，规则引擎裁决（CG3，对称 SpawnPolicy）

## 3. 验收标准（Change 级）

| Phase | 验收 |
|-------|------|
| F1 | 契约类型 + Sibling taxonomy + CL/CB 规则单测 PASS |
| F2 | BubbleStructured 接入父 Observe |
| F3 | BlockedBy → upstream 物化 + feature flag |
| F4 | Plan LLM proposer 接线 |
| F5 | D2 sidechain 分区 `wi_<id>` |
| F6 | CLI `/task context show` + 集成测试 |

## 4. 关联

- 上游：`workitem-pipeline-unification-design.md` v0.6.0（G1–G5 + Phase A–E）
- 设计：`workitem-context-graph-design.md`
- 依赖：`D7_WORKITEM_PIPELINE=1`（F3+ 运行时验收）
