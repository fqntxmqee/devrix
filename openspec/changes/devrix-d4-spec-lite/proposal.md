# Proposal: d4 多智能体域 spec 精简

**Change ID:** devrix-d4-spec-lite
**Demand ID:** DM-20260630-007
**Status:** S2_Proposal
**Follows:** devrix-spec-lite-mode (DM-20260630-003), devrix-d2-spec-lite (DM-20260630-004), devrix-d1-spec-lite (DM-20260630-005), devrix-d3-spec-lite (DM-20260630-006)

---

## 1. Background

`devrix-spec-lite-mode` (DM-20260630-003) 已 S7_Archived，lite-mode 规范生效：spec.md ≤ 200 / CHANGELOG.md ≤ 300。d7/d2/d1/d3 已完成精简（d7 2622→195 / d2 1622→152 / d1 577→175 / d3 1060→149）。

d4 spec.md 当前 **222 行**（略超 200），含 1 个 process requirement 段（Sub-Agent Mode Field，54 行）。d4 canonical 6 S（ProvisionAgent/RunAgentLoop/IsolateAndMerge/ExecuteWorker/InvokeExternalAgent/ConfigureAgents）已稳定收敛，Legacy S1-S10 冻结追溯。

## 2. Problem Statement

| 维度 | 现状 | 期望 |
|------|------|------|
| d4 spec.md 行数 | **222** | ≤ 200 |
| Sub-Agent Mode Field Requirement 段 | 54 行（process requirement） | 0（合并到 archive/） |
| d4 canonical Requirement | 6 段（每段 1 句 + 1 canonical Gherkin） | 0（合并到 §Scenarios 表） |
| d4 CHANGELOG.md | **无** | ≤ 300 |

**痛点**：
- 违反已生效的 lite-mode 规范（spec.md 硬上限 200 行）
- Sub-Agent Mode Field process requirement（54 行）对当前开发是噪音
- d4 canonical 6 S 已稳定收敛

## 3. Proposed Solution

### 3.1 方案对比

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A. 复用 d7/d2/d1/d3 lite-mode 模式（推荐）** | 一致性；4 站验证成熟 | Sub-Agent Mode Field 需迁出 |
| B. 按 S 分片（spec-s{11..16}.md） | S 层清晰 | 子文件持续累积；与 lite-mode 反模式 |
| C. 维持 222 行不拆分 | 0 改动 | 违反 lite-mode 硬上限 |

**选择**：A

**理由**：4 站验证成熟，d4 与 d1/d2/d3 canonical S 同构。

### 3.2 spec.md 8 段结构

1. Overview — D4 Delegation Execution Follower
2. 核心设计原则 — 7-8 条
3. S 层职责（canonical 6）+ Legacy S1-S10 双轨
4. DSAFT 结构
5. Scenarios（6 canonical S 状态表）
6. Architecture（Hub-Spoke 编排归 D7）
7. 关键 Scenario 范式
8. 关键链路口

### 3.3 CHANGELOG.md 时间线

- 2026-06-30 devrix-d4-spec-lite（本 change）
- 2026-06-29 devrix-d4-dsaft-restructuring
- 2026-06-15 devrix-d4-sa-refine
- 2026-06-08 devrix-multi-agent

**总计**：4 条 d4 change

## 4. AC 总结

12 AC（详见 demand.md §3）：AC1 spec.md ≤ 200 / AC2 8 段 / AC3 1-2 canonical / AC4 CHANGELOG.md ≤ 300 + ≥ 3 d4 change / AC5 0 子文件 / AC6 0 Go diff / AC7 11 d4 子文档 0 diff / AC8 Sub-Agent Mode Field 迁 archive / AC9 规范立即生效 / AC10 verify-archive / AC11 demand-archive-index / AC12 verdict

## 5. 关联引用

- devrix-spec-lite-mode (DM-20260630-003, s7_archived)
- devrix-d2/d1/d3-spec-lite (s7_archived)
- d4 3 archive：v1 / sa-refine / dsaft-restructuring
- d4 域 SoT: `d4-multi-agent/d4-domain.md` v2.2.0 (DM-20260629-004 收口，未触碰)
- d4 D7 边界: `d4-multi-agent/d7-boundary.md` (未触碰)
