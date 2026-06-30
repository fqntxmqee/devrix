---
demand-id: DM-20260630-004
title: d2 上下文引擎 spec 精简（lite-mode 推广）
priority: P1
status: S1_Proposal
dsaft_domain: context-engine
created: 2026-06-30
follows: DM-20260630-003 (devrix-spec-lite-mode, s7_archived)
---

# d2 上下文引擎 spec 精简

## 1. 背景

`devrix-spec-lite-mode` (DM-20260630-003) 已 S7_Archived (2026-06-30, PR #333+#334 联动)，规范升级：
- `architecture-design.md` v1.3.0 §6.4 — spec.md ≤ 200 行 / CHANGELOG.md ≤ 300 行
- `archiving.md` v1.4.0 §2.4 — Scenario 留在 archive/<change>/specs/，不合并到域 spec.md

d7 已作为示范完成（d7 spec.md 2622 → 195，CHANGELOG.md NEW 103 行）。本 change 是 **lite-mode 推广第一站**：d2 是 backlog 中**最大的目标**（1622 行）+ 4 个 backlog 候选中收益最高。

## 2. 问题陈述

| 问题 | 现状 | 期望 |
|------|------|------|
| d2 spec.md 行数 | **1622** | ≤ 200（精简设计契约） |
| d2 Requirement 详细文本 | 66 Requirement / 96 Scenario 全部驻 spec.md | spec.md 顶部 1-2 canonical 范式；详细文本随 change 留在 archive/ |
| d2 历史追溯 | 必须全文 grep spec.md | spec.md → CHANGELOG.md → archive/<change>/specs/ 3 跳 |
| d2 跨域漂移历史 | 8 个 ADDED Requirements 段（V1/V2/V3/V4/V5/V6/V6-v2/V7/TOOL-SURFACE-1/S16）累计 1590 行 | CHANGELOG.md 时间线 + archive/ 历史 |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `d2-context-engine/spec.md` 重写为精简设计契约（≤ 200 行） | P0 |
| AC2 | spec.md 包含：Overview / DSAFT 结构 / 核心设计原则 / S 层职责 / Scenarios / Architecture / 关键 Scenario 范式 / 关键链路口 | P0 |
| AC3 | spec.md 含 1-2 个 canonical Gherkin 范式（按 S15 Prepare / S17 Persist 选一） | P0 |
| AC4 | 新建 `d2-context-engine/CHANGELOG.md`（≤ 300 行）：时间线列表（≥ 28 条 d2 change，最近 30 天） | P0 |
| AC5 | d2-context-engine/ 目录不含 `spec-s{XX}.md` / `spec-{topic}.md` 等子文件 | P0 |
| AC6 | 不改 Go 代码（`git diff --stat internal/` = 0） | P0 |
| AC7 | 不改 d2 其他子文档（a-registry / t-registry / f-registry / d2-domain / d7-boundary / design / span-registry / dsaf / observability-guide / prompt-system-design / terminal-state-guide / layer-delta） | P0 |
| AC8 | 66 个 Requirement + 96 个 Scenario 全部留 archive（`grep -r "^#### Scenario:" archive/ | wc -l` = 96） | P0 |
| AC9 | 规范升级对其他域（d1/d3/d4/d5/d6）立即生效，本 change 不强推 | P1 |
| AC10 | `verify-archive.sh` 通过（本 change 走完 S6-归档） | P0 |
| AC11 | `openspec/demand-archive-index.md` 追加 DM-20260630-004 行 | P0 |
| AC12 | acceptance-report.md verdict: ACCEPTED | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260630-003 已 S7_Archived（lite-mode 规范生效） |
| 依赖 | d7 spec.md 195 行已合入 master（PR #333+#334 squash auto-merge） |
| 约束 | 不破坏 d2-domain.md v9.0.0（DM-20260629-002 收口版） |
| 约束 | 96 个 Scenario 留 archive（与 d7 spec-lite 同模式） |
| 约束 | 不复制 Requirement 文本到 CHANGELOG.md（仅引用 archive/） |
| 约束 | 不改 Go 代码 |
| 约束 | 不动 `openspec/specs/d{1,3,4,5,6}-*/`（其他域不强推） |
| 约束 | 不动 `openspec/specs/project/*`（规范已生效，无需重升版） |

## 5. 变更范围

### 新增

- `openspec/specs/d2-context-engine/CHANGELOG.md`（d2 域时间线）
- `openspec/changes/devrix-d2-spec-lite/` S1-S5 六阶段文档
- `openspec/archive/2026-06-30-devrix-d2-spec-lite/`（S6-归档后）

### 修改

- `openspec/specs/d2-context-engine/spec.md` 重写为精简设计契约（1622 → ≤ 200 行）

### 删除

- 66 个 Requirement 详细文本 + 96 个 Scenario 详细文本（保留在原 archive/，不在本 change 删除）

### 不变更

- `openspec/specs/d2-context-engine/` 12 个其他子文档
- `openspec/specs/d{1,3,4,5,6}-*/spec.md`（其他域不强推）
- 任何 Go 代码 / CI 配置 / 业务逻辑
- `openspec/specs/project/*` 规范文件（lite-mode 已生效）
- `openspec/specs/d2-context-engine/d2-domain.md` v9.0.0（D2-Domain SoT 不变）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 96 个 Scenario 失追溯 | 中 — reviewer 找不到历史场景 | 全部在 archive/<原 change>/specs/，通过 demand-archive-index.md + CHANGELOG.md 引用 |
| d2-domain.md v9.0.0 已被 DM-20260629-002 收口，spec.md 重写是否冲突 | 低 — d2-domain 是 SoT，spec.md 是契约 | d2-domain.md 不动；spec.md 引用 d2-domain.md 作为 SoT |
| d2 spec.md 与 TOOL-SURFACE-1/S16 复杂链路 | 中 — TOOL-SURFACE-1 占 600+ 行 | spec.md 仅保留 1-2 canonical 范式（如 Materialize 路径） |
| d2 Spec 与 d7 spec 维护负担 | 低 — lite-mode 通用 | d7 spec.md 已示范，d2 复用同模式 |
| 96 个 Scenario 中部分不在 archive/ | 低 — d2 历史 21 个 archive 目录全覆盖 | 实施时只删 d2 spec.md 中累积部分，archive/ 历史已存在 0 触碰 |