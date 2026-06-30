# Demand: d5 可观测性域 spec 精简（lite-mode 推广第五站）

**Demand ID:** DM-20260630-008
**Date:** 2026-06-30
**Priority:** P1
**Domain:** D5
**Source:** devrix-spec-lite-mode (DM-20260630-003) lite-mode 推广第五站

---

## 1. 背景

lite-mode 模式（spec.md ≤ 200 + CHANGELOG.md ≤ 300）已在 d7/d2/d1/d3/d4 五域成功落地（PR #333/#336/#338/#340/#342），6 个 change 全部 S7_Archived，0 Go 代码 diff 0 其他域 diff。

d5 可观测性域当前状态：
- **spec.md = 376 行**（超出 lite-mode 200 行上限 +88%）
- **无 CHANGELOG.md**
- 11 个 d5 子文档（d5-domain.md / d5-boundary.md / a-registry.md / f-registry.md / t-registry.md / span-registry.md / dsaf-architecture.md / observability-guide.md / coverage.md / terminal-state-guide.md / layer-delta.md / design.md）

## 2. 目标

将 d5 spec.md 从 376 行精简到 ≤ 200 行（lite-mode 标准），新增 CHANGELOG.md ≤ 300 行覆盖 d5 域 30 天变更时间线。

## 3. 验收标准（AC）

| ID | 标准 |
|----|------|
| AC1 | d5 spec.md ≤ 200 行 |
| AC2 | spec.md 含 8 段契约（Overview / 核心设计原则 / S 层职责 / DSAFT / Scenarios / Architecture / 关键 Scenario 范式 / 关键链路口） |
| AC3 | spec.md 含 1-2 canonical Gherkin 范式（候选：D5-S21 Tracer Start Operation Registry / D5-S22 OTLP SpanExporter / D5-S23 Coverage HealthCheck） |
| AC4 | d5 CHANGELOG.md ≤ 300 行 + ≥ 3 d5 change |
| AC5 | d5 目录无子文件 |
| AC6 | 0 Go 代码 diff |
| AC7 | d5 其他 12 子文档 0 diff |
| AC8 | 详细 Requirements 13 条迁 archive（1 行 reference） |
| AC9 | 规范升级对 d6/d7 生效，本 change 不强推 |
| AC10 | verify-archive.sh 通过 |
| AC11 | demand-archive-index.md 追加 DM-20260630-008 行 |
| AC12 | verdict: ACCEPTED |

## 4. 范围

**In Scope**：
- `openspec/specs/d5-observability/spec.md` REWRITE
- `openspec/specs/d5-observability/CHANGELOG.md` NEW
- 6 change docs（demand/proposal/design/tasks/acceptance-report/.openspec.yaml）

**Out of Scope**：
- d5 12 子文档任何修改（d5-domain.md / d5-boundary.md / a-registry.md / f-registry.md / t-registry.md / span-registry.md / dsaf-architecture.md / observability-guide.md / coverage.md / terminal-state-guide.md / layer-delta.md / design.md）
- Go 代码任何修改
- d1/d2/d3/d4/d6/d7 任何修改
- devrix-verify-spec-links（独立 backlog change）

## 5. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 13 条 Requirements 详细 Gherkin 迁 archive 可能丢失上下文 | 1 行 reference 指向 archive change 详细文本 |
| canonical Scenario 选错可能失去关键链路表达 | 候选 D5-S23 Coverage HealthCheck（运行时命中计数 + Registry 对账） |
| 376→200 压缩比 47% 比 d7 2622→195 难度低但比 d4 222→155 高 | 重点削 13 条 Requirements 详细文本 + V1.0-V1.9 版本里程碑表 + Revision History |

## 6. 复用模式

复用 d7/d2/d1/d3/d4 lite-mode 模式（DM-20260630-003/004/005/006/007）：
1. spec.md 顶部 SoT 引用 + 8 段结构
2. CHANGELOG.md 4 列表格
3. process requirement 详细文本迁 archive（1 行 reference）
4. 0 Go 代码 diff + 0 其他域 diff + 0 子文档 diff
5. 1 canonical Gherkin 范式
6. git mv changes/ → archive/<date>-<id>/

## 7. 下一步

- S2 proposal：方案 A 复用 lite-mode vs 方案 B 物理分片 vs 方案 C 仅做 CHANGELOG
- S3 design：六段式设计
- S4 实现：切 feat/devrix-d5-spec-lite + spec.md 重写 + CHANGELOG.md NEW
- S5 验收：12 AC 验证
- S6 交付：PR + auto-merge
- S6 归档：独立 PR