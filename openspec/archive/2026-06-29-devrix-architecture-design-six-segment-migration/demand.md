---
demand-id: DM-20260629-007
title: architecture-design.md 六段式规范升级 + 设计文档模板统一
priority: P1
status: S6_Archived (2026-06-29)
dsaft_domain: project
created: 2026-06-29
reporter: 2026-06-29 devrix-d7-taskcontract-unification (DM-20260629-006) S3 评审触发
related:
  - devrix-d7-taskcontract-unification（DM-20260629-006）— 第一个按新六段式落地的 Change
  - docs/methodology/detail-design-framework.md — 六段式定义源头（53 行）
  - 18+ 已归档 d{N}-dsaft-restructuring Change — 7 段模板历史
---

# architecture-design.md 六段式规范升级

## 1. 背景

2026-06-29 在评审 `devrix-d7-taskcontract-unification`（DM-20260629-006）design.md 时发现：**`openspec/specs/project/architecture-design.md` 自身存在矛盾**。

### 1.1 §1.2 vs §4 模板冲突

- **§1.2** 引用 `docs/methodology/detail-design-framework.md` 六段式（①架构目标 ②架构原则 ③业务流程 ④领域模型 ⑤核心链路图 ⑥接口/API 设计）
- **§4** 自己定义的 design.md 模板却是另一套（7 段轻量变更：Root Cause / Solution / Key Interfaces / Data Flow / File Manifest / Regression Risk / Rollback）
- 两套体系完全不重叠，§1.2 用 "应参照"（should）弱语义

### 1.2 §1.3 豁免口子

- "非架构级变更可跳过六段式" — 给了执行绕过的口子
- 18+ 已归档 Change（包括 devrix-d7-dsaft-restructuring / devrix-d1-ac-restructuring / devrix-d2-dsaft-restructuring / devrix-d3-dsaft-restructuring / devrix-d4-dsaft-restructuring 等）都按 7 段模板写
- 无人按六段式实际落地

### 1.3 §8 S3 checklist 缺口

- checklist 校验的是 §4 7 段模板（含 design.md 包含根因/方案/文件清单/回归风险 4 项）
- 漏了"六段式完整性"和"六段式非空"校验

## 2. 问题陈述

### 2.1 规范内部矛盾

`architecture-design.md` §1.2 与 §4 是**冲突的规范**：
- §1.2 要求复杂架构文档遵循 detail-design 六段式
- §4 提供的是 7 段模板（与六段式无关）
- §8 checklist 校验 §4 7 段模板

### 2.2 模板缺口

已实施的 18+ Change 普遍使用 7 段模板 → **新规范无人遵循**：
- 设计文档缺乏"架构原则"（设计原则 + 命名规范 + 代码风格）
- 设计文档缺乏"领域模型"（聚合根 + 限界上下文）
- 设计文档缺乏"核心链路图"（端到端 + SLA + 单点风险）

### 2.3 评审不闭环

S3 评审时无"六段式完整性"检查 → 评审者只能凭记忆判断 → 不同 reviewer 标准不一。

## 3. 验收标准

| ID | 标准 | 优先级 | 对应章节 |
|----|------|--------|----------|
| AC1 | `architecture-design.md §1.2` 改"应参照"为"**必须遵循**"，列出六段式标题与符号 | P0 | §1.2 |
| AC2 | `architecture-design.md §1.3` 删"轻量变更可跳过六段式"豁免，改"范围与详细度裁剪"（禁止旧式 7 段模板替代）| P0 | §1.3 |
| AC3 | `architecture-design.md §4` design.md 模板改为六段式（①-⑥）+ 附录 | P0 | §4 |
| AC4 | `architecture-design.md §8` S3 checklist 加"六段式完整性"+"六段式非空" 2 项校验 | P0 | §8 |
| AC5 | 已归档 Change（18+）**不追溯**（保持 7 段模板）| P1 | 决议 |
| AC6 | 进行中 Change 走自然过渡（不强制回填）| P1 | 决议 |
| AC7 | 第一个按新六段式落地的 Change 是 `devrix-d7-taskcontract-unification`（reference）| P1 | reference |

**总计 7 AC**：4 个规范修改 + 2 个决议 + 1 个 reference。

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `docs/methodology/detail-design-framework.md`（六段式定义源头，53 行）|
| 依赖 | `devrix-d7-taskcontract-unification`（DM-20260629-006）S3 评审触发 |
| 约束 | 项目级规范文件（master.md 子规范之一）|
| 约束 | 修改影响所有未来 Change 的 design.md 模板结构 |
| 约束 | 已归档 Change 不追溯（避免 18+ 历史 Change 重新归档）|
| 约束 | detail-design-framework.md 六段式标题符号（①-⑥）不可改名 |

## 5. 变更范围

### 修改

- `openspec/specs/project/architecture-design.md` 4 个段落（§1.2 / §1.3 / §4 / §8）

### 新增

- 本 Change 自身（DM-20260629-007）作为规范升级的归档凭证
- memory 记录：`devrix-architecture-design-six-segment-upgrade-2026-06-29.md`

### 不变更

- `docs/methodology/detail-design-framework.md`（模板源头，不变）
- 18+ 已归档 Change 的 design.md（不追溯）
- `master.md` 流程定义（不变）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 修改规范文件影响范围广 | 所有未来 Change 都需遵循六段式 | AC1 强制语义 + AC4 checklist 校验 |
| 已归档 Change 回填成本高 | 18+ 历史 design.md 需重写 | AC5 决议不追溯（0 成本）|
| 新作者学习曲线 | 不熟悉六段式 | detail-design-framework.md 53 行 + 本 Change design.md 作 reference |
| 进行中 Change 不合规 | S3 评审拒收 | AC6 决议自然过渡（不阻塞）|
| §1.2 / §1.3 / §4 内部一致性 | 三处修改需同步 | 单文件 4 段修改原子完成（已实施）|

## 7. 后续 Phase 关联

本 Change 范围窄（4 处段落修改 + 1 个 reference Change 记录），按 `architecture-design.md §1.3` "范围与详细度裁剪"属**小型 Change**（< 5 AC / < 1 PR）：

| 阶段 | 内容 | 状态 |
|------|------|------|
| S1 需求 | 本文件（demand.md）| ✅ 已创建 |
| S2 提案 | `proposal.md`（6 节 + 3 Decision）| 待写 |
| S3 设计 | `design.md`（按新六段式）+ `specs/project/spec.md` + `tasks.md` | 待写 |
| S4-S6 | **无 PR**（规范已直接修改，归档即可）| S6 归档 |

**工作量**：1-2 天（实际已大部分完成：本 Change 评审时已完成规范修改 + memory 记录 + 本 Change 设计文档）

## 8. 参考资料

- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md` — 升级记录
- `docs/methodology/detail-design-framework.md` — 六段式定义源头
- `openspec/specs/project/architecture-design.md` — 待升级规范文件
- `openspec/changes/devrix-d7-taskcontract-unification/` — 第一个按新六段式落地的 Change
- 前置归档：`openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/`（v6.0.x 收官，7 段模板历史）