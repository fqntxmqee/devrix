---
demand-id: DM-20260630-003
title: specs 域文档轻量化（精简契约 + 轻量 changelog）
priority: P1
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-30
replaces: DM-20260630-002 (s1_cancelled)
---

# specs 域文档轻量化

## 1. 背景

PR #332（DM-20260630-001 拆分规范升级）于 2026-06-30 合入 master，将 `architecture-design.md` 升级至 v1.2.0（§6.4 specs 文档规模约束：硬上限 800 行 + 按 S 分片原则），将 `archiving.md` 升级至 v1.3.0（§2.5 spec.md 按 S 分片合并强制规则）。

在按新规范推进 d7-orchestration/spec.md 实际拆分时（S3-S5），发现方向偏离用户意图：

- 用户原始诉求是 **specs 域文档保持精简**，**过程需求迭代**走 `archive/` 目录
- PR #332 的"按 S 分片"思路**仍把 Scenario 留在 specs/**（虽然分文件，但 specs/ 整体规模没降）
- 174 个 Scenario 全部在 spec-s{XX}.md / spec-cross-cutting.md 中累积，仅解决"单文件超 800"问题，未解决"specs 整体过载"问题

原 `devrix-d7-spec-split` (DM-20260630-002) 已 s1_cancelled，本 change 替代。

## 2. 问题陈述

当前规范（PR #332 合入版）的问题：

| 问题 | 现状 | 期望 |
|------|------|------|
| specs/ 整体规模累积 | d7 仅 spec.md 就 2622 行，d2/d3/d4 各 1000+ | specs/ 域目录 ≤ 1 个 200 行 spec.md + 1 个 300 行 CHANGELOG.md |
| 过程需求与最新设计混杂 | spec.md 累积 63 Requirement / 174 Scenario | spec.md 只放当前符合代码的契约；过程需求在 `archive/<change>/specs/` |
| 索引入口膨胀 | 14 个 spec-s{XX}.md + 1 cross-cutting.md | 1 个 spec.md（含 CHANGELOG.md 链接） |
| 检索体验 | 读 Scenario 必须打开子文件 | spec.md 顶部 1-2 个关键 Scenario 范式示例，其余跳 CHANGELOG.md → archive/ |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `architecture-design.md §6.4` 改为"spec.md ≤ 200 行 / CHANGELOG.md ≤ 300 行 / 其他 d{N} 子文档 ≤ 800 行" | P0 |
| AC2 | `architecture-design.md §6.4` 删除"按 S 分片"硬要求，改为"specs 域 = 精简设计契约 + 轻量 changelog" | P0 |
| AC3 | `archiving.md §2.4` 改为"Scenario 留在 archive/<change>/specs/，不合并到域 spec.md" | P0 |
| AC4 | `archiving.md §2.5` 删除（按 S 分片合并强制规则废弃） | P0 |
| AC5 | `architecture-design.md` 升级到 v1.3.0，`archiving.md` 升级到 v1.4.0 | P0 |
| AC6 | 原 `devrix-d7-spec-split` 标 `s1_cancelled`（含 `cancelled_reason` + `replaced_by` 字段） | P0 |
| AC7 | `d7-orchestration/spec.md` 重写为精简设计契约（≤ 200 行）：Overview / DSAFT / 核心设计原则 / 关键链路口 / 1-2 个关键 Scenario 范式 | P0 |
| AC8 | 新建 `d7-orchestration/CHANGELOG.md`（≤ 300 行）：时间线列表（每个 change 一行）+ Requirement/Scenario 增删摘要 | P0 |
| AC9 | d7-orchestration/ 目录不含 14 个 spec-s{XX}.md / spec-cross-cutting.md（本 change 不创建这些） | P0 |
| AC10 | 不改 Go 代码 / 不改 d7 其他子文档（a-registry / t-registry / f-registry / d7-domain / d3-boundary 等） | P0 |
| AC11 | 规范升级对其他域（d1/d2/d3/d4/d5/d6）立即生效，本 change 不强推其他域拆分 | P1 |
| AC12 | `verify-archive.sh` 通过（本 change 走完 S6-归档） | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | PR #332 (DM-20260630-001 拆分规范升级) 已合入 master |
| 依赖 | 原 `devrix-d7-spec-split` (DM-20260630-002) s1_cancelled |
| 约束 | 不破坏 PR #332 已合入的 §6.4 / §2.5 既有条款（升级为新版本，保留可追溯性） |
| 约束 | `d7-orchestration/spec.md` 精简后必须含 Overview / DSAFT / 关键设计 / 链路口 4 段 |
| 约束 | `d7-orchestration/CHANGELOG.md` 不复制 spec.md 内容，只列 change-id 链接 + 一句话摘要 |
| 约束 | 174 个原 Scenario 留在 archive/<原 change>/specs/，不复制到 d7 CHANGELOG.md |
| 约束 | 不改 Go 代码 |
| 约束 | 不动 `openspec/specs/d{N}-*/` 其他域的 spec.md（本 change 仅示范 d7） |

## 5. 变更范围

### 新增

- `openspec/specs/d7-orchestration/CHANGELOG.md`（d7 域时间线摘要）
- `openspec/changes/devrix-spec-lite-mode/` S1+S2+S3+S4+S5+S6 六阶段文档
- `openspec/archive/2026-06-30-devrix-spec-lite-mode/`（S6-归档后）

### 修改

- `openspec/specs/project/architecture-design.md` v1.2.0 → **v1.3.0**（§6.4 改精简模式）
- `openspec/specs/project/archiving.md` v1.3.0 → **v1.4.0**（§2.5 删除，§2.4 改 changelog 模式）
- `openspec/specs/d7-orchestration/spec.md` 重写为精简设计契约（≤ 200 行）
- `openspec/changes/devrix-d7-spec-split/.openspec.yaml` `status: s1_cancelled` + 元数据

### 删除

- `openspec/changes/devrix-d7-spec-split/proposal.md` / `design.md` / `tasks.md` / `demand.md`（仅保留 .openspec.yaml 作为 s1_cancelled 标记）

### 不变更

- `openspec/specs/d7-orchestration/` 17 个其他子文档（a-registry / t-registry / d7-domain / 等）
- `openspec/specs/d{1..6}-*/spec.md`（其他域不强制改动）
- 任何 Go 代码 / CI 配置 / 业务逻辑
- `openspec/specs/project/master.md`（阶段路由不变）
- `openspec/specs/project/coding.md` / `git-workflow.md` / `testing.md` 等其他规范

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 174 个原 Scenario 失追溯 | 中 — reviewer 找不到历史场景 | 全部在 archive/<原 change>/specs/，通过 demand-archive-index.md + CHANGELOG.md 引用 |
| 其他域（d1/d2/d3/d4）规范不统一 | 低 — 仅示范 d7 | 规范升级对所有域生效，d1/d2/d3/d4 视需求后续拆分 |
| Reviewer 偏好"按 S 分片"而非"完全轻量化" | 中 | design.md §② 记录两种方案对比，决策证据完整 |
| 规范升级覆盖 PR #332 既有条款 | 低 | 升级为 v1.3.0 / v1.4.0，保留版本号变更历史 |
| 174 个原 Scenario 文本保留 17 个 archive 目录，需逐一检查 | 中 | S4 实施时只删 d7 spec.md 中累积部分，archive/ 历史不受影响 |
