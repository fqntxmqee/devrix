---
demand-id: DM-20260630-005
title: d1 通信层 spec 精简（lite-mode 推广第二站）
priority: P1
status: S1_Proposal
dsaft_domain: communication
created: 2026-06-30
follows: DM-20260630-003 (lite-mode 范式) + DM-20260630-004 (d2 推广第一站, s7_archived)
---

# d1 通信层 spec 精简

## 1. 背景

`devrix-spec-lite-mode` (DM-20260630-003) S7_Archived (PR #333+#334) + `devrix-d2-spec-lite` (DM-20260630-004) S7_Archived (PR #336+#337) 已完成，lite-mode 模式在 d7/d2 域验证成熟：
- spec.md ≤ 200 / CHANGELOG.md ≤ 300 / 其他 d{N} 子文档 ≤ 800
- 过程需求 Scenario 详细文本留在 archive/<change>/specs/

d1 spec.md 当前 **577 行**（含 90 个 Scenario 详细文本，由 DM-20260629-005 PR-6 gherkin-restructuring 落地 18 缩写 bullet → 90 展开），是 backlog 4 个候选中**最小目标**（仅次于 d4 spec.md 222 行），lite-mode 推广收益高 + 风险低。

## 2. 问题陈述

| 问题 | 现状 | 期望 |
|------|------|------|
| d1 spec.md 行数 | **577** | ≤ 200（精简设计契约） |
| d1 Scenario 详细文本 | 90 个 `#### Scenario:` 块全驻 spec.md | spec.md 顶部 1-2 canonical 范式；详细文本留 archive/ |
| d1 历史追溯 | 全文 grep spec.md | spec.md → CHANGELOG.md → archive/<change>/specs/ 3 跳 |
| d1 跨域漂移 | 切法 A 信号分层 + Boundary Debt + 影子编排 | 简化为 1 段"边界契约"引用 d7-boundary.md |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `d1-communication/spec.md` 重写为精简设计契约（≤ 200 行） | P0 |
| AC2 | spec.md 含：Overview / 核心设计原则 / S 层职责（canonical S13-S18）/ DSAFT 结构 / Scenarios / Architecture / 关键 Scenario 范式 / 关键链路口 8 段 | P0 |
| AC3 | spec.md 含 1-2 canonical Gherkin 范式（按 S13 CaptureUserIntent 选一） | P0 |
| AC4 | 新建 `d1-communication/CHANGELOG.md`（≤ 300 行）：时间线列表（≥ 6 条 d1 change，最近 30 天） | P0 |
| AC5 | d1-communication/ 目录不含 `spec-s{XX}.md` / `spec-{topic}.md` 等子文件 | P0 |
| AC6 | 不改 Go 代码（`git diff --stat internal/` = 0） | P0 |
| AC7 | 不改 d1 其他 12 个子文档（d1-domain / a-registry / t-registry / f-registry / design / d7-boundary / span-registry / dsaf-architecture / observability-guide / terminal-state-guide / layer-delta / feishu-task-planning-verification） | P0 |
| AC8 | 90 个 Scenario 全部留 archive | P0 |
| AC9 | 规范升级对其他域（d3/d4/d5/d6）立即生效，本 change 不强推 | P1 |
| AC10 | `verify-archive.sh` 通过（本 change 走完 S6-归档） | P0 |
| AC11 | `openspec/demand-archive-index.md` 追加 DM-20260630-005 行 | P0 |
| AC12 | acceptance-report.md verdict: ACCEPTED | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260630-003 S7_Archived（lite-mode 规范生效） |
| 依赖 | DM-20260630-004 S7_Archived（d2 推广验证完成） |
| 依赖 | DM-20260629-005 S7_Archived（d1 AC-restructuring 收口：spec v6.0.0 + d7-boundary.md NEW） |
| 约束 | 不破坏 d1-domain.md v1.2.0（DM-20260629-005 收口版） |
| 约束 | 90 个 Scenario 留 archive（d1 6 个 archive 目录已覆盖） |
| 约束 | 不复制 Scenario 文本到 CHANGELOG.md（仅引用 archive/） |
| 约束 | 不改 Go 代码 |
| 约束 | 不动 `openspec/specs/d{2,3,4,5,6}-*/`（d2 已 done，其他域不强推） |
| 约束 | 不动 `openspec/specs/project/*`（规范已生效） |

## 5. 变更范围

### 新增

- `openspec/specs/d1-communication/CHANGELOG.md`（d1 域时间线）
- `openspec/changes/devrix-d1-spec-lite/` S1-S5 六阶段文档
- `openspec/archive/2026-06-30-devrix-d1-spec-lite/`（S6-归档后）

### 修改

- `openspec/specs/d1-communication/spec.md` 重写为精简设计契约（577 → ≤ 200 行）

### 不变更

- `openspec/specs/d1-communication/` 12 个其他子文档
- `openspec/specs/d{2,3,4,5,6}-*/spec.md`（d2 已 done，其他域不强推）
- 任何 Go 代码 / CI 配置 / 业务逻辑
- `openspec/specs/project/*` 规范文件
- `openspec/specs/d1-communication/d1-domain.md` v1.2.0

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 90 个 Scenario 失追溯 | 中 — reviewer 找不到历史场景 | 全部在 d1 6 个 archive/ 目录（devrix-d1-ac-restructuring 含 gherkin-restructuring 完整 90），CHANGELOG.md 按需引用 |
| d1-domain.md v1.2.0 与 spec.md 冲突 | 低 — 双层文档职责清晰 | spec.md 显式引用 d1-domain.md v1.2.0 作为 SoT + d7-boundary.md 作为跨域契约 |
| 90 个 Scenario 中含 happy/sad/boundary/concurrent/timeout 5 类平衡 | 低 — 已 PR-6 #4 编排 | 1 canonical 选 happy 路径（入站飞书消息持久化成功） |
| 切法 A 信号分层博弈论内容丢失 | 中 — 学术性强 | spec.md 1 段"信号分层" + ValueFlow 引用 d1-domain.md §North Star |
| 6 个 ADDED Requirements 段（V1..V6 + 切法 A + Boundary Debt）累积 470+ 行 | 中 — 历史过程需求 | CHANGELOG.md 按需精简（> 30 天折叠）；不复制 Scenario 文本 |