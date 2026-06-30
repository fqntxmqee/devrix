# Proposal: d2 上下文引擎 spec 精简

**Change ID:** devrix-d2-spec-lite
**Demand ID:** DM-20260630-004
**Status:** S2_Design
**Follows:** devrix-spec-lite-mode (DM-20260630-003, s7_archived)

## 1. Background

`devrix-spec-lite-mode` (DM-20260630-003) 已 S7_Archived (2026-06-30, PR #333+#334)，lite-mode 规范生效：spec.md ≤ 200 / CHANGELOG.md ≤ 300 / 其他 d{N} 子文档 ≤ 800。d7 已作为示范完成（spec.md 2622 → 195，CHANGELOG.md NEW 103 行）。

本 change 是 lite-mode 推广第一站。**d2 是 backlog 中最大的目标**（1622 行）+ 4 个候选中收益最高。

## 2. Problem Statement

d2 spec.md 累积了从 V1 (DM-20260607-001) 到 V8 (DM-20260628-001 layer subcontext phase 3) 全部 8 个 ADDED Requirements 段 + 96 个 Scenario 详细文本：

| 维度 | 现状 | 期望 |
|------|------|------|
| d2 spec.md 行数 | 1622 | ≤ 200 |
| Requirement 数 | 66 | spec.md 中 0（全部留 archive） |
| Scenario 数 | 96 | spec.md 中 1-2 canonical 范式 |
| 8 个 ADDED Requirements 段 | V2 (Autocompact) / V3 (PEV Plan) / V4 (Async + Snappy) / V5 (Harness Bootstrap) / V6 (QueryLoop) / V6-v2 (ExecutionFlow) / V7 (Harness Unification) / TOOL-SURFACE-1 / S16 | 全部留 archive/，spec.md 仅保留 canonical S15-S18 |
| 跨域漂移历史 | D2-D3 LLM Gateway / D2-D7 Turn / D2-D4 Delegate 8 段历史 | 简化为 1 段"边界契约"引用 d7-boundary.md |
| 检索体验 | 全文 grep / Ctrl+F | spec.md 顶部契约段 → CHANGELOG.md → archive/ |

## 3. Proposed Solution

### 3.1 总体策略

**复用 lite-mode 模式**（与 d7 spec-lite 同形）：

```
openspec/specs/d2-context-engine/
  spec.md         ≤ 200 行   当前符合代码的设计契约（v9.0.0）
  CHANGELOG.md    ≤ 300 行   时间线列表（28+ d2 change，最近 30 天）
  d2-domain.md    不变       v9.0.0 D2-Domain SoT（DM-20260629-002 收口）
  其他 11 个子文档 不变       a/t/f-registry / design / span-registry / dsaf / observability-guide / prompt-system-design / terminal-state-guide / layer-delta / d7-boundary
```

**过程需求**（66 Requirement / 96 Scenario 详细文本）**留在 archive/ 各 change 目录**，不进入 specs/。

### 3.2 拆分粒度决策

**方案对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. d7 lite-mode 复用（推荐） | 一致性；复用规范已验证模式；d7 已示范 | 仍需 1 跳到 archive/ |
| B. 按 8 个 V 阶段分片（spec-v2.md, spec-v3.md, ...） | 阶段清晰 | 子文件持续累积；specs 整体规模不降；与 lite-mode 反模式 |
| C. 维持 1622 行不拆分 | 0 改动成本 | 违反 lite-mode 规范硬上限（800/200） |

**选择:** A（复用 lite-mode）

**理由：**
- d7 已示范并归档，模式成熟
- 用户原始诉求 = specs 域文档只放最新符合代码的设计
- 96 个 Scenario 详细文本对当前开发是噪音（已合入代码，需求态已转为代码态）
- d2-domain.md v9.0.0 已提供 North Star + 物理路径 + 实现状态 SoT，spec.md 引用即可

### 3.3 d2 spec.md 新结构

```markdown
# Context Engine Specification

> 当前符合代码的设计契约（v9.0.0）。详细 Scenario 历史见 [CHANGELOG.md](CHANGELOG.md) 与 [archive/](../../archive/)。

## Overview
（3-5 段：d2 域职责 / Context Follower / Prepare-ToolRound-Persist 三原语 / 上下游接口 / 与 D7 Leader-Follower 关系）

## 核心设计原则
（5-8 条 bullet：会话级内存 / 7 步压缩 / 异步 Autocompact / Snappy 快照 / 工具注册中心化 / Deferred Complete / Turn 收口归 D7 / D2→D3 import 禁止）

## S 层职责（canonical S15-S18）
（表格：D2-S15 PrepareExecutionContext / D2-S17 PersistSessionState / D2-S18 EnforceExecutionPolicy；S19 拆解、S20 移除）

## DSAFT 结构
（表格：D=1, S=4, A=22, F=120, T=180 IMPLEMENTED）

## Scenarios
（表格：4 个 canonical S 状态）

## Architecture
（5 节点角色 + Leader/Follower 拓扑图 + 跨域边界 d7-boundary.md 引用）

## 关键 Scenario 范式
（1-2 canonical：S15 PrepareExecutionContext 或 S17 PersistSessionState）

## 关键链路口
（4-6 端到端：D7 Leader → D2 Prepare → D7 ToolRound → D2 Persist / D7→D3 LLM / D2→D4 Delegate / D2→D5 LRU / D2→D6 Verify）
```

行数目标：170-200 行。

### 3.4 d2 CHANGELOG.md 新结构

```markdown
# D2 Context Engine — Changelog

> 时间线列表。每个 change 一行 + 一句话摘要 + 链接到 archive/。

| Date | Change ID | 摘要 | 归档 |
|------|-----------|------|------|
| 2026-06-30 | devrix-d2-spec-lite | d2 spec.md 1622→200 lite-mode (12 AC) | [archive](../archive/2026-06-30-devrix-d2-spec-lite/) |
| 2026-06-29 | devrix-d2-dsaft-restructuring | v8.2→v9.0 8 PR/44 T/14 G 全 PASS | [archive](../archive/2026-06-29-devrix-d2-dsaft-restructuring/) |
| ... | ... | ... | ... |
| 2026-06-07 | devrix-context-engine | V1 上下文引擎基线 | [archive](../archive/2026-06-07-devrix-context-engine/) |

## 维护规则
- 每次归档追加 1 行（Date / Change ID / 摘要 / 链接）
- 30 天前条目折叠为 1 行 + 链接
- 不复制 Requirement / Scenario 文本（引用 archive/）
```

行数目标：80-150 行（28 条 d2 change 时间线，d2 历史比 d7 短）。

## 4. Success Metrics

| 指标 | 目标 | 测量 |
|------|------|------|
| d2 spec.md 行数 | ≤ 200 | `wc -l` |
| d2 CHANGELOG.md 行数 | ≤ 300 | `wc -l` |
| spec.md Scenario 范式数 | 1-2 个 | `grep -c "^#### Scenario:"` |
| CHANGELOG.md change 条目数 | ≥ 28（最近 30 天 d2 changes） | `grep -c "^|" CHANGELOG.md` |
| 96 个原 Scenario 全部留 archive | 96 = 96 | 跨 archive/ 全局 grep |
| 不动 Go 代码 | 0 diff | `git diff --stat internal/` |
| 不动 d2 其他 12 个子文档 | 0 diff | `git diff --stat openspec/specs/d2-context-engine/`（除 spec.md + 新增 CHANGELOG.md） |
| S5 验收 verdict | ACCEPTED | acceptance-report.md |

## 5. Implementation Plan

| 阶段 | 产出 | 门禁 |
|------|------|------|
| S3 设计 | design.md（六段式 + 新旧方案对比 + rollback） | S3-Gate 通过 |
| S4 实现 | d2 spec.md 精简（1622 → ≤ 200）+ d2 CHANGELOG.md + .openspec.yaml + demand/proposal/design/tasks.md | go vet / wc -l 检查 |
| S5 验收 | acceptance-report.md (verdict: ACCEPTED) + 96 Scenario 全部留 archive 验证 | AC1-AC12 全过 |
| S6-交付 | PR 合入 master (squash + auto-merge) | CI 全绿 |
| S6-归档 | mv changes/devrix-d2-spec-lite/ → archive/2026-06-30-devrix-d2-spec-lite/ + demand-archive-index.md 追加 + .openspec.yaml s7_archived + 新 PR | verify-archive.sh PASS |

**单 PR 一次到位**（按 devrix 模式 squash 合并）：本次 PR 包含 S1+S2+S3+S4+S5 全部 commit，最后 S6-归档独立 PR。

## 6. Risks & Mitigations

详见 `demand.md` §6。补充：

- **风险**: d2 spec.md 删 1422 行（V1-V7 详细文本），Reviewer 担心历史追溯
- **缓解**: d2 历史已存 21 个 archive 目录，CHANGELOG.md 按需引用（不复制 Scenario）
- **风险**: d2-domain.md v9.0.0 是 SoT，spec.md 简化是否与之冲突
- **缓解**: spec.md 显式引用 d2-domain.md 作为 SoT；canonical S 列表与 d2-domain.md §Canonical 价值流一致
- **风险**: TOOL-SURFACE-1 占 spec.md 600+ 行（1005-1595），是单段最大累积
- **缓解**: spec.md 仅保留 1 canonical 范式（Surface → Runner → 域 对照表简化为 3-4 行 + 引用 surface-design/）

## 7. Out of Scope

- d2 design.md / t-registry.md / 其他子文档拆分 — Backlog 单独立项
- d1/d3/d4/d5/d6 spec.md 同步精简 — 视需求后续，每个域独立 change 立项
- CI 工具 `verify-spec-links.sh` — 后续单独立项
- d2-domain.md 内容修订 — v9.0.0 已收口，本 change 不动
- d2 DSAFT 结构调整 — 已在 DM-20260629-002 S7_Archived

## 8. Reference

- `openspec/specs/d2-context-engine/spec.md`（1622 行原版，重写为精简版）
- `openspec/specs/d2-context-engine/d2-domain.md` v9.0.0（D2-Domain SoT，不变）
- `openspec/archive/2026-06-30-devrix-spec-lite-mode/`（lite-mode 规范归档，d7 示范）
- `openspec/archive/2026-06-29-devrix-d2-dsaft-restructuring/`（D2 v9.0.0 S7_Archived）