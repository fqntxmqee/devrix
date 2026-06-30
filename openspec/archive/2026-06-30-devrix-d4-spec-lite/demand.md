# Demand: d4 多智能体域 spec 精简

**Demand ID:** DM-20260630-007
**Change ID:** devrix-d4-spec-lite
**Priority:** P1
**Status:** S1_Requirement
**Date:** 2026-06-30
**Follows:** devrix-spec-lite-mode (DM-20260630-003, s7_archived), devrix-d2-spec-lite (DM-20260630-004), devrix-d1-spec-lite (DM-20260630-005), devrix-d3-spec-lite (DM-20260630-006)

---

## 1. 背景

lite-mode 规范（DM-20260630-003）已生效：spec.md ≤ 200 行 + CHANGELOG.md ≤ 300 行。d7/d2/d1/d3 已完成精简（d7 2622→195 / d2 1622→152 / d1 577→175 / d3 1060→149）。d4 spec.md 当前 **222 行**（略超 200），且含有 devrix-context-budget-phase-b 的 54 行 process requirement（start line 148）属过程需求，应当走 archive。

## 2. 问题陈述

| 维度 | 现状 | 期望 |
|------|------|------|
| d4 spec.md 行数 | 222 | ≤ 200 |
| d4 process requirement 段 | 1 个（54 行，context-budget-phase-b） | 0（合并到 archive/） |
| d4 canonical Requirement | 6 个（每个 1 句 + 1 canonical Gherkin） | 同上 |
| d4 CHANGELOG | 无 | 新建 ≤ 300 行 |
| 跨 archive/ 追溯 | 3 个 d4 archive（v1/sa-refine/dsaft-restructuring） | 保留 |

**痛点**：
- 违反已生效的 lite-mode 规范（spec.md 硬上限 200 行）
- Sub-Agent Mode Field process requirement（54 行）对当前开发是噪音
- d4 canonical 6 S 已稳定收敛（S11-S16），无需长 Requirement 段
- d4-domain.md v2.2.0 已 S7_Archived 收口，spec.md 仅作精简契约

## 3. Acceptance Criteria（12 AC）

### AC1：spec.md ≤ 200 行
- `wc -l openspec/specs/d4-multi-agent/spec.md` ≤ **200**
- 量化目标：170-200 行

### AC2：spec.md 含 8 段契约
- 段 1: Overview
- 段 2: 核心设计原则
- 段 3: S 层职责（canonical 6 S）
- 段 4: DSAFT 结构
- 段 5: Scenarios（6 canonical S 状态表）
- 段 6: Architecture
- 段 7: 关键 Scenario 范式（1-2 canonical）
- 段 8: 关键链路口

### AC3：spec.md 含 1-2 canonical Gherkin 范式
- 选 D4-S14 ExecuteWorker happy 路径作为 canonical（Worker fork→run→join，最常见接口）
- 或 D4-S13 IsolateAndMerge COW 隔离（最有特色场景）

### AC4：CHANGELOG.md ≤ 300 行
- `wc -l openspec/specs/d4-multi-agent/CHANGELOG.md` ≤ **300**
- 量化目标：30-60 行
- 列出最近 30 天 ≥ **3 条** d4 change（d4 archive 仅有 3 个，覆盖即可）

### AC5：d4 目录无子文件
- `ls openspec/specs/d4-multi-agent/` 仅 d4 子文档（d4-domain / a/f/t-registry / design / d7-boundary / span-registry / dsaf-architecture / observability-guide / terminal-state-guide / layer-delta）+ spec.md + CHANGELOG.md
- 无 `spec-s{XX}.md` / `spec-{topic}.md` 等子文件

### AC6：不动 Go 代码
- `git diff --stat internal/` = **0**

### AC7：不动 d4 其他 11 个子文档
- `git diff --name-only -- openspec/specs/d4-multi-agent/` 仅 `spec.md` + 新增 `CHANGELOG.md`
- 其他 d4 子文档（d4-domain / d7-boundary / a-registry / f-registry / t-registry / span-registry / dsaf-architecture / observability-guide / terminal-state-guide / design / layer-delta 等 11 个）0 diff

### AC8：Sub-Agent Mode Field requirement 留 archive
- 跨 archive/ 全局 grep `Sub-Agent Mode Field` / `mode enum` 在原 spec.md 命中 → 0（已迁出）
- 新 spec.md 仅 1 行 reference："mode field 详细 Gherkin 在 archive/2026-06-20-devrix-context-budget-phase-b/"

### AC9：规范升级对其他域（d5/d6）立即生效，本 change 不强推
- `git diff --stat openspec/specs/d{5,6}-*/` = **0**
- 规范已生效（DM-20260630-003）

### AC10：`verify-archive.sh` 通过
- `./scripts/verify-archive.sh devrix-d4-spec-lite` PASS 或预期内 WARN（proposal.md 状态标记 + specs/*/spec.md 不存在 lite-mode caveat）

### AC11：`openspec/demand-archive-index.md` 追加 DM-20260630-007 行
- 在主 Demand 表追加一行
- 在归档路径表追加一行

### AC12：acceptance-report.md verdict: ACCEPTED
- 12 AC 全部满足
- d4 域备注：d4 canonical S = 6 (S11 ProvisionAgent / S12 RunAgentLoop / S13 IsolateAndMerge / S14 ExecuteWorker / S15 InvokeExternalAgent / S16 ConfigureAgents) + Legacy S1-S10 双轨

## 4. 范围

### 4.1 包含

- `openspec/specs/d4-multi-agent/spec.md` REWRITE（222 → ≤ 200）
- `openspec/specs/d4-multi-agent/CHANGELOG.md` NEW（≤ 300）
- `openspec/changes/devrix-d4-spec-lite/` S2-S5 docs

### 4.2 不包含

- d4 其他 11 个子文档（不动）
- 其他域 specs（不强推）
- Go 代码 / CI 配置 / 业务逻辑
- Backlog（Out of Scope）：
  - `devrix-d4-design-split`（d4 design.md 1064 行）
  - `devrix-d5-spec-lite`（d5 spec.md 376 行）
  - `devrix-d6-spec-lite`（d6 spec.md 604 行）
  - `devrix-verify-spec-links`（CI 工具）

## 5. 风险

| 风险 | 缓解 |
|------|------|
| Sub-Agent Mode Field requirement detail 丢失 | 1 行 reference 指向 archive/2026-06-20-devrix-context-budget-phase-b/ |
| d4 design.md 1064 行未精简 | Backlog 立项 `devrix-d4-design-split`；本 change 不动 |
| d4 Legacy S1-S10 双轨表格过长 | 表格精简；指向 archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md |

## 6. 关联引用

- devrix-spec-lite-mode (DM-20260630-003, s7_archived)
- devrix-d2-spec-lite (DM-20260630-004, s7_archived)
- devrix-d1-spec-lite (DM-20260630-005, s7_archived)
- devrix-d3-spec-lite (DM-20260630-006, s7_archived)
- d4 3 archive：v1 / sa-refine / dsaft-restructuring
