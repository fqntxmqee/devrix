---
demand-id: DM-20260630-002
title: 拆分 d7-orchestration/spec.md 至规范上限
priority: P1
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-30
---

# 拆分 d7-orchestration/spec.md 至规范上限

## 1. 背景

2026-06-30 研发流程规范体检发现：`openspec/specs/d7-orchestration/spec.md` 已累积至 **2622 行 / 63 Requirements / 174 Scenarios**，远超项目级规范硬上限（800 行）。其他三个超大域文档：

- d2-context-engine/spec.md: 1622 行
- d7-orchestration/t-registry.md: 1133 行
- d3-llm-gateway/spec.md: 1060 行 / design.md: 1042 行
- d4-multi-agent/design.md: 1064 行

同步新增的 `architecture-design.md §6.4` 规定 specs/ 文档硬上限 800 行、`archiving.md §2.5` 要求 spec.md 按 S 分片合并。本 change 负责把 d7-spec.md 落进新规范框架。

## 2. 问题陈述

单 spec.md 2622 行导致：

- S3-Gate 设计审查 diff 难（PR review 痛苦）
- AI Agent 上下文窗口爆（master.md §2.2 路由表加载入口）
- `archiving.md §2.4` 旧规则"将 changes 下的 Scenario 合并到域 spec.md"是累积指令，无拆分出口，越合越大
- 当前 d7-orchestration/ 已有 17 个文件（含 d7-domain / d3-boundary / pipeline-architecture / observability-guide 等），但 spec.md / design.md / t-registry.md 仍是黑洞

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | d7-orchestration/spec.md 主文件 ≤ 200 行（仅含 Overview / DSAFT 结构 / Scenarios 总览 / Architecture / 索引） | P0 |
| AC2 | d7-orchestration/spec.md 主文件严格 ≤ 800 行（硬上限） | P0 |
| AC3 | 按 S 拆分的子文件 `spec-s{XX}.md` 每个 ≤ 600 行（软上限）/ 800 行（硬上限） | P0 |
| AC4 | 主 spec.md 顶部含"## Scenario Index"段，列出所有子文件链接 | P0 |
| AC5 | DSAFT ID 全局唯一，跨文件用 T 层注释（`<!-- T: D7-S8-A15-T01 -->`）追溯 | P0 |
| AC6 | 拆分后所有 Requirement / Scenario 文本 0 复制（仅引用） | P0 |
| AC7 | S3-Gate 通过（design.md 含拆分粒度决策 + 主文件 ≤ 800 行验证） | P0 |
| AC8 | S5 验收 verdict: ACCEPTED（CI 全绿 + verify-archive.sh PASS） | P0 |
| AC9 | `archiving.md §2.5` 新规则首次走通（场景演练：mock 一个 S2 改动归档） | P1 |
| AC10 | 文档末尾追加 Revision History 段，记录拆分前/后规模 | P1 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `devrix-architecture-design-six-segment-migration` 已合入（2026-06-29）—— 提供六段式骨架 |
| 依赖 | `architecture-design.md v1.2.0` §6.4 文档规模约束（本批次同步升级） |
| 依赖 | `archiving.md v1.3.0` §2.5 按 S 分片合并规则（本批次同步升级） |
| 约束 | 不改 Requirement / Scenario 文本（仅位置移动 + 索引维护） |
| 约束 | 不改 T 层 ID 编号（D7-S8-A15-T01 等保持原样） |
| 约束 | 不改 Gherkin 语义（场景措辞、Given/When/Then 不动） |
| 约束 | 拆分不破坏 git blame 实用性（按 S 整块移动，行号变动可接受） |

## 5. 变更范围

### 新增

- `openspec/specs/d7-orchestration/spec-s{XX}.md`（按 S 拆分的子文件，约 14-18 个）
- `openspec/specs/d7-orchestration/spec.md` 重写为索引 + Overview/Architecture 主体

### 修改

- `openspec/specs/d7-orchestration/spec.md` 主体结构（不再平铺所有 Requirement/Scenario）

### 不变更

- 任何 Requirement / Scenario 文本
- 任何 T 层 ID 编号
- 任何 Gherkin 语义
- `openspec/specs/d7-orchestration/design.md`（1133 行）—— 列入后续 backlog，本 change 不处理
- `openspec/specs/d7-orchestration/t-registry.md`（1133 行）—— 列入后续 backlog，本 change 不处理
- d2 / d3 / d4 域文档拆分 —— 列入后续 backlog

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 拆分后跨文件引用断链 | 中 — 阅读时跳链体验下降 | 主 spec.md 顶部 Scenario Index 段集中维护所有链接；CI 加 `verify-spec-links.sh`（后续） |
| git blame 历史性下降 | 低 — 整块移动导致旧 blame 失效 | 文件头 Revision History 记录拆分时间点；按 S 整块移动避免行级碎片 |
| S 边界判定争议 | 中 — 部分 Requirement 横跨 S（如 D7-S8-A15 Observation） | 拆分时按"主 S"归属；横跨 S 的内容在子文件头部用 "相关 S" 段指引 |
| 子文件仍然超 800 行 | 低 — 单 S 内 Scenario 不会太多 | 评估：d7 最大 S 约 9 Req / 9 Sce（约 150-200 行），无次级拆分需求 |
| reviewer 对拆分粒度有分歧 | 中 — 可能被要求按 A 拆分而非按 S | S3-Gate 决策记录在 design.md 附录；如有变更再起 follow-up change |
