# Demand: d3 LLM 网关 spec 精简

**Demand ID:** DM-20260630-006
**Change ID:** devrix-d3-spec-lite
**Priority:** P1
**Status:** S1_Requirement
**Date:** 2026-06-30
**Follows:** devrix-spec-lite-mode (DM-20260630-003, s7_archived), devrix-d2-spec-lite (DM-20260630-004, s7_archived), devrix-d1-spec-lite (DM-20260630-005, s7_archived)

---

## 1. 背景

lite-mode 规范（DM-20260630-003）已生效：spec.md ≤ 200 行 + CHANGELOG.md ≤ 300 行。d7/d2/d1 已完成精简（d7 spec.md 2622→195 / d2 1622→152 / d1 577→175）。d3 spec.md 已 **1060 行**，是当前最大 backlog 目标之一。

## 2. 问题陈述

| 维度 | 现状 | 期望 |
|------|------|------|
| d3 spec.md 行数 | 1060 | ≤ 200（精简设计契约） |
| d3 子文档数 | 12（d3-domain/a/f/t-registry/design/dsaft-architecture/observability-guide/span-registry/model-resolution-trace/terminal-state-guide/layer-delta 等） | 不动 |
| d3 Scenario 详细文本 | 90 个详细 Gherkin（行 ~85-1000） | 0 个（精简 1-2 canonical 范式） |
| d3 CHANGELOG | 无 | 新建 ≤ 300 行 |
| 跨 archive/ 90 Scenario 详细文本追溯 | 6 个 d3 archive 目录（v1/v2/sa-refine/v1.1/v2.0/dsaft-restructuring）累积 | 保留（不丢失） |

**痛点**：
- 违反已生效的 lite-mode 规范（spec.md 硬上限 200 行）
- 90 Scenario 详细文本对当前开发是噪音（已合入代码）
- d3 canonical S 已收敛（6 个承诺装置 + 1 CROSS），无需逐 S 详述
- 用户原始诉求：specs 域文档只放最新符合代码的设计，过程需求走 archive/

## 3. Acceptance Criteria（12 AC）

### AC1：spec.md ≤ 200 行
- `wc -l openspec/specs/d3-llm-gateway/spec.md` ≤ **200**
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
- 选 D3-S1 RouteModel happy 路径作为 canonical（最简单接口）
- 或 D3-S3 ProtectCall Breaker Open 路径（最有特色场景）

### AC4：CHANGELOG.md ≤ 300 行
- `wc -l openspec/specs/d3-llm-gateway/CHANGELOG.md` ≤ **300**
- 量化目标：60-100 行
- 列出最近 30 天 ≥ **6 条** d3 change

### AC5：d3 目录无子文件
- `ls openspec/specs/d3-llm-gateway/` 仅 d3 子文档（a/f/t/d-registry/design/dsaft-architecture/observability-guide/span-registry/model-resolution-trace/terminal-state-guide/layer-delta）+ spec.md + CHANGELOG.md
- 无 `spec-s{XX}.md` / `spec-{topic}.md` 等子文件

### AC6：不动 Go 代码
- `git diff --stat internal/` = **0**

### AC7：不动 d3 其他 12 个子文档
- `git diff --name-only -- openspec/specs/d3-llm-gateway/` 仅 `spec.md` + 新增 `CHANGELOG.md`
- 其他 d3 子文档（d3-domain / a-registry / f-registry / t-registry / dsaf-architecture / design / observability-guide / span-registry / model-resolution-trace / terminal-state-guide / layer-delta 等 11 个）0 diff

### AC8：90 Scenario 留 archive（distribution summary 留 CHANGELOG.md）
- 跨 archive/ 全局 grep `#### Scenario:` 总数 = **90**（0 丢失）
- d3 CHANGELOG.md 新增 "90 Scenario 分布表"（happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9，与 d1 同分布）

### AC9：规范升级对其他域（d4/d5/d6）立即生效，本 change 不强推
- `git diff --stat openspec/specs/d{4,5,6}-*/` = **0**
- 规范已生效（DM-20260630-003）

### AC10：`verify-archive.sh` 通过
- `./scripts/verify-archive.sh devrix-d3-spec-lite` PASS 或预期内 WARN（proposal.md 状态标记，与 d7/d2/d1 spec-lite 同 pattern）

### AC11：`openspec/demand-archive-index.md` 追加 DM-20260630-006 行
- 在主 Demand 表追加一行（DM-20260630-006 / devrix-d3-spec-lite / 2026-06-30 / PR #<number> / S7_Archived）
- 在归档路径表追加一行（devrix-d3-spec-lite → archive/2026-06-30-devrix-d3-spec-lite/）

### AC12：acceptance-report.md verdict: ACCEPTED
- 12 AC 全部满足（含已知 trade-off 文档化）
- d3 域异常备注：d3 canonical S = 6 (5 承诺 + 1 横切) + D3-X CROSS 跨域

## 4. 范围

### 4.1 包含

- `openspec/specs/d3-llm-gateway/spec.md` REWRITE（1060 → ≤ 200）
- `openspec/specs/d3-llm-gateway/CHANGELOG.md` NEW（≤ 300）
- `openspec/changes/devrix-d3-spec-lite/` S2-S5 docs（demand/proposal/design/tasks/acceptance-report + .openspec.yaml）

### 4.2 不包含

- d3 其他 12 个子文档（不动）
- 其他域 specs（不强推）
- Go 代码 / CI 配置 / 业务逻辑（0 行为变更）
- Backlog（Out of Scope）：
  - `devrix-d3-design-split`（d3 design.md 1042 行拆分）
  - `devrix-d3-tregistry-split`（d3 t-registry.md 296 行拆分）
  - `devrix-d4-spec-lite`（222 行 spec.md / 1064 行 design.md）
  - `devrix-verify-spec-links`（CI 工具）

## 5. 风险

| 风险 | 缓解 |
|------|------|
| 90 Scenario 详细文本丢失追溯 | CHANGELOG.md distribution summary + archive/ 全局 grep 校验 |
| d3-design.md 1042 行未精简 | Backlog 立项 `devrix-d3-design-split`；本 change 不动 |
| d3 canonical S = 6 个比 d2 多（S15-S20 4 个）+ d1 多（S13-S18 6 个）| 6 个 S 段简化表格展示 |
| d3 CROSS 跨域锚点（§9）不在 canonical 6 S 内 | 6 S + 1 CROSS = 7 元素，不算复杂度增量 |
| reviewer 担心 d3 spec 引用过多（d3-domain.md + dsaf-architecture.md + observability-guide.md + terminal-state-guide.md + model-resolution-trace.md + span-registry.md）| spec.md 顶部 SoT 引用段统一声明，与 d2/d1 同 pattern |

## 6. 关联引用

- devrix-spec-lite-mode (DM-20260630-003, s7_archived) — lite-mode 规范源头
- devrix-d2-spec-lite (DM-20260630-004, s7_archived) — lite-mode 推广第一站
- devrix-d1-spec-lite (DM-20260630-005, s7_archived) — lite-mode 推广第二站
- d3 6 个 archive 目录（d3-llm-gateway/v2/sa-refine/v1.1/v2.0/dsaft-restructuring）
