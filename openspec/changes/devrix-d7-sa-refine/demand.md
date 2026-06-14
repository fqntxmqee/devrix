---
demand-id: DM-20260614-008
title: D7 Orchestration — S 层价值流重构与 S2 入口上移
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-14
---

# D7 Orchestration — S 层价值流重构与 S2 入口上移

## 1. 背景

### 1.1 D7 根本目标（领域 North Star）

**「编排」= 决定做什么、按什么顺序做、谁来做、做得怎么样了。**

D7 作为横向协调层，编排 D2（LLM↔Tool 执行原语）与 D4（Agent 委托原语），并通过 D1 向用户发布进度信号。

### 1.2 现状问题

| 问题 | 根因 |
|------|------|
| D7-S2 Session Orchestrator **0 T 实现**，但已是 D1 实际路由目标 | S 切法按模块（coordinator/wave/flow）而非用户价值流 |
| D7-S1 Task 写模型在 **D2** `contextengine/tasks/` | S 跨域边界漂移 |
| A-registry 中 `d7/` 路径大量不存在 | 规格与实现脱节 |
| D7-S5 ClassifyIntent 部分在 coordinator，部分在 D2 | A 层归属不清 |

### 1.3 博弈论视角

**S2 入口确定性 > S5 决策准确性**。当前 S2 全 PLANNED 是结构性风险：
- D1 路由依赖 S2 ProcessMessage，但无生产验证锚点
- 开发者局部最优（先做 wave/flow）≠ 用户全局最优（S2 端到端可验收）

---

## 2. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | S2 主入口 ProcessMessage 无 T 锚点 | 生产不可验证 |
| P2 | Task 写模型跨 D2/D7 边界 | 职责不清，变更互相影响 |
| P3 | A 层归属混乱（classifier 部分在 D2） | 无法独立演进 |
| P4 | F-registry `d7/` 路径大量不存在 | 规格与实现脱节 |

---

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | D7-S2 ProcessMessage 入口有 P0 T 锚点（端到端或契约测试） | P0 |
| AC2 | D7-S5 ClassifyIntent 路由决策在 coordinator 内，不依赖 D2 | P0 |
| AC3 | D7-S1 Task 模型归属 D7（可通过指标或 stub 验证） | P1 |
| AC4 | S 切法按用户价值流（S2/S3/S4/S5），非按模块 | P0 |
| AC5 | Legacy 双轨：旧 A/F 冻结追溯，新 A/F Canonical | P0 |

---

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `dsaft-methodology.md`、`dsaft-refactoring-playbook.md` |
| 依赖 | D7 v2.2.0 现有注册表（`specs/d7-orchestration/`） |
| 约束 | **切法 A**：S 按用户价值流（S2=会话入口、S5=决策规划） |
| 约束 | v1.0 registry-only；不改 Go 代码 |

---

## 5. 变更范围

### 新增

- D7 新 S 切法（S2=会话编排、S5=决策规划）含 Legacy 双轨
- `devrix-d7-sa-refine/` change 包（S1 demand + S2 proposal）
- D7-S2 ProcessMessage T 锚点草案（PLANNED 标注）

### 修改

- `openspec/specs/d7-orchestration/a-registry.md`（A 层重编号 + Legacy 追溯）
- `openspec/specs/d7-orchestration/f-registry.md`（F 层清理不存在路径）
- `openspec/specs/d7-orchestration/t-registry.md`（T 层 S2 补充）

### 不变更

- v1.0 Go 代码
- 已 IMPLEMENTED 的 T（44 条）
- D7-S3 Wave / D7-S4 Flow（已稳定）

---

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 双轨 S 表增加认知负担 | 中 | 明确 SoT = Canonical；Legacy 仅追溯 |
| S2 实现状态与 spec 不符 | 高 | v1.0 仅 registry；T 锚定 PLANNED |
| 跨 D2 Task 模型迁移 | 中 | v1.1 再动；v1.0 保持现状 |

---

## 7. S 切法候选

### 切法 A（推荐）：按用户价值流

| S | 价值流 | 核心 A |
|---|--------|--------|
| D7-S2 | 会话编排入口 | ProcessMessage、HandleInterrupt |
| D7-S3 | Wave 调度 | ScheduleWave、GuardConflict |
| D7-S4 | 执行流 | PublishFlowEvent、NotifyGateway |
| D7-S5 | 决策规划 | ClassifyIntent、SynthesizeTaskGraph |

### 切法 B（现状）：按模块

| S | 模块 | 问题 |
|---|------|------|
| S2 | coordinator | 含 S2+S5，边界不清 |
| S3 | wave | ok |
| S4 | flow/workplan/imsink | 4 个子模块合并 S4 |
| S5 | plan | 规划从 S2 拆分，但 Task 模型仍在 D2 |

**推荐切法 A**，与 D1 切法 A 原则一致：S 表达用户价值流，非技术模块。
