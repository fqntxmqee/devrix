# Demand: d6 演化域 spec 精简（lite-mode 推广第六站）

**Demand ID:** DM-20260630-009
**Date:** 2026-06-30
**Priority:** P1
**Domain:** D6
**Source:** devrix-spec-lite-mode (DM-20260630-003) lite-mode 推广第六站

---

## 1. 背景

lite-mode 模式（spec.md ≤ 200 + CHANGELOG.md ≤ 300）已在 d7/d2/d1/d3/d4/d5 六域成功落地（PR #333/#336/#338/#340/#342/#344），全部 S7_Archived。

d6 演化域当前状态：
- **spec.md = 604 行**（超出 lite-mode 200 行上限 +202%）
- **无 CHANGELOG.md**
- 7 个 d6 子文档（d6-domain.md / a-registry.md / f-registry.md / t-registry.md / span-registry.md / layer-delta.md / design.md）

## 2. 目标

将 d6 spec.md 从 604 行精简到 ≤ 200 行（lite-mode 标准），新增 CHANGELOG.md ≤ 300 行覆盖 d6 域 30 天变更时间线。

## 3. 验收标准（AC）

| ID | 标准 |
|----|------|
| AC1 | d6 spec.md ≤ 200 行 |
| AC2 | spec.md 含 8 段契约 |
| AC3 | spec.md 含 1-2 canonical Gherkin 范式（候选：D6-S3 Tier Resolution ≥ 99% / D6-S3 Delta 回归 Red/Yellow/Green / D6-S4 Guard Observer Fork 捕获 / D6-S5 Invariant fail-closed log.Fatalf） |
| AC4 | d6 CHANGELOG.md ≤ 300 行 + ≥ 3 d6 change |
| AC5 | d6 目录无子文件 |
| AC6 | 0 Go 代码 diff |
| AC7 | d6 其他 7 子文档 0 diff |
| AC8 | 详细 Requirements 18 条迁 archive（1 行 reference） |
| AC9 | 规范升级对其他域生效，本 change 不强推 |
| AC10 | verify-archive.sh 通过 |
| AC11 | demand-archive-index.md 追加 DM-20260630-009 行 |
| AC12 | verdict: ACCEPTED |

## 4. 范围

**In Scope**：
- `openspec/specs/d6-evolution/spec.md` REWRITE
- `openspec/specs/d6-evolution/CHANGELOG.md` NEW
- 6 change docs

**Out of Scope**：
- d6 7 子文档任何修改
- Go 代码任何修改
- 其他域任何修改
- devrix-verify-spec-links（独立 backlog change）

## 5. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 18 条 Requirements 详细 Gherkin 迁 archive 可能丢失上下文 | 1 行 reference 指向 archive change 详细文本 |
| canonical Scenario 选错可能失去关键链路表达 | 候选 D6-S3 Tier Resolution ≥ 99%（v2.2.0 新增探针，跨 D3-D6 锚点） |
| 604→200 压缩比 67% 是 6 站最大压缩 | 重点削 18 条 Requirements 详细文本 + 10 类探针详细表 + Revision History + D6-S11/S12 韧性域新增需求 |

## 6. 复用模式

复用 d7/d2/d1/d3/d4/d5 lite-mode 模式（DM-20260630-003/004/005/006/007/008）：
1. spec.md 顶部 SoT 引用 + 8 段结构
2. CHANGELOG.md 4 列表格
3. process requirement 详细文本迁 archive
4. 0 Go 代码 diff + 0 其他域 diff + 0 子文档 diff
5. 1 canonical Gherkin 范式
6. git mv changes/ → archive/<date>-<id>/

## 7. 下一步

- S2 proposal：A vs B vs C
- S3 design：六段式
- S4 实现
- S5 验收
- S6 交付 + 归档