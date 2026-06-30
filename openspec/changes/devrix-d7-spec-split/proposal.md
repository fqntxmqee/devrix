# Proposal: 拆分 d7-orchestration/spec.md

**Change ID:** devrix-d7-spec-split
**Demand ID:** DM-20260630-002
**Status:** S2_Design

## 1. Background

`openspec/specs/d7-orchestration/spec.md` 自 2026-06-14 以来持续累积，已达 2622 行 / 63 Requirements / 174 Scenarios，是项目内最大单文件。同步在 2026-06-30 研发流程规范体检中确认：该规模超出项目级规范硬上限（800 行），且无任何规范约束 specs/ 文档规模。

详见 `demand.md` §1-§2。

## 2. Problem Statement

详见 `demand.md` §2。简言之：

1. 单 spec.md 超大 → S3-Gate 审查 / PR diff / AI Agent 上下文窗口体验恶化
2. `archiving.md §2.4` 旧规则鼓励累积 → 无拆分出口
3. 规范未约束 specs/ 文档规模 → 现状偏离项目"小而精"传统

## 3. Proposed Solution

### 3.1 总体策略

**主文件 + 子文件分层架构**，与 `architecture-design.md §6.4` 对齐：

```
openspec/specs/d7-orchestration/
  spec.md                 # 主文件：Overview / DSAFT 结构 / Scenarios 总览 / Architecture / Scenario Index
  spec-s01.md             # S1 Work Model
  spec-s03.md             # S3 Wave Scheduler
  spec-s04.md             # S4 Execution Flow Hub
  ...                     # 按需展开 ~14-18 个 S
```

主 spec.md 顶部新增 `## Scenario Index` 段，集中维护所有子文件链接。

### 3.2 拆分粒度决策

**选项对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 按 S 拆分（推荐） | 与 DSAFT 体系一致；归档按 S 分片时一一对应；粒度适中 | 横跨多 S 的 Requirement 需在主文件"参考说明" |
| B. 按 A 拆分 | 活动粒度更细（≤ 400 行/文件） | d7 大部分 S 内 A 数 ≤ 3，过度拆分；与 spec.md "Scenario 列表" 习惯不一致 |
| C. 按章节（Overview / Scenarios / Architecture） | 简单直接 | 违反 Gherkin 范式；不解决单文件超 800 行问题 |

**选择:** A（按 S 拆分）

**理由:**
- 与 `archiving.md §2.5` 新规则"按 S 分片合并"一一对应
- 18 个 S × 平均 145 行 ≈ 合理粒度（主 200 + 子 × 18 = 总规模不变）
- d7-orchestration/ 现有 17 文件已按主题切分（d7-domain / pipeline-architecture / observability-guide / terminal-state-guide 等），spec-s{XX}.md 与之风格统一
- 单 S 内 Scenario 数最大 9（S13-A47），约 200 行，无需进一步按 A 拆分

### 3.3 拆分范围（d7 18 个 S）

| S 文件 | 主题 | 估算 Requirement/Scenario |
|--------|------|---------------------------|
| spec-s01.md | S1 Work Model | 2 Req / 2 Sce |
| spec-s03.md | S3 Wave Scheduler | 1 Req / 4 Sce |
| spec-s04.md | S4 Execution Flow Hub | 1 Req / 4 Sce |
| spec-s05.md | S5 Plan Mode | 1 Req / 1 Sce |
| spec-s06.md | S6 Verify Promotion | (待盘点) |
| spec-s08.md | S8 Observation + Plan Data Contract | 2 Req / 9 Sce |
| spec-s09.md | S9 Execute Artifact + Channel | 2 Req / 9 Sce |
| spec-s11.md | S11 Learn 节点 | 5 Req / 22 Sce |
| spec-s12.md | S12 Observe-Learner 集成 | 3 Req / 11 Sce |
| spec-s13.md | S13 Verify-Auto-Close | 3 Req / 20 Sce |
| spec-s14.md | S14 Escape Engine | (待盘点) |
| spec-s15.md | S15 (待盘点) | (待盘点) |
| spec-s16.md | S16 (待盘点) | (待盘点) |
| spec-s18.md | S18 错误聚合 | (待盘点) |
| spec-s20.md | S20 TaskSpec 下行契约 | (待盘点) |
| spec-s21.md | S21 (待盘点) | (待盘点) |
| spec-s22.md | S22 (待盘点) | (待盘点) |

S3 阶段在 design.md §④ 中按实际 spec.md 内容逐 S 重新盘点，给出精确切分点（行号 + Requirement 范围）。

### 3.4 主 spec.md 结构（拆分后）

```markdown
# D7 Orchestration Domain Specification

> 主体 Gherkin 场景按 S 拆分到 spec-s{XX}.md 子文件；本文档保留 Overview / DSAFT 结构 / Scenarios 总览 / Architecture / Scenario Index。

## Overview
## DSAFT 结构
## Scenarios
## Architecture
## Scenario Index
- [S1 Work Model → spec-s01.md](spec-s01.md)
- [S3 Wave Scheduler → spec-s03.md](spec-s03.md)
- ...

## Revision History
| Date | Change |
|------|--------|
| 2026-06-30 | 拆分：原 2622 行 → 主 200 行 + spec-s{XX}.md × 14-18（DM-20260630-002） |
```

## 4. Success Metrics

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| 主 spec.md 行数 | ≤ 200（软上限）/ ≤ 800（硬上限） | `wc -l` |
| 子 spec-s{XX}.md 行数 | ≤ 600（软上限）/ ≤ 800（硬上限） | `wc -l` |
| Requirement / Scenario 文本复制数 | 0 | 文本 diff |
| Gherkin 语义变化 | 0 | diff spec.md（旧 vs 新）+ 各子文件 |
| 跨文件链接断链 | 0 | 后续 CI 加 verify-spec-links.sh |
| S3-Gate 一次通过 | 1 次 | review-design.md 检查清单 |
| S5 验收一次通过 | 1 次 | acceptance-report verdict |

## 5. Implementation Plan

| 阶段 | 产出 | 门禁 |
|------|------|------|
| S3 设计 | design.md（六段式 + 精确切分点表 + S3-Gate） | S3-Gate 通过 |
| S4 实现 | spec.md 主体重写 + spec-s{XX}.md × 14-18 | go vet / test-unit / 拆分后 wc -l 检查 |
| S5 验收 | acceptance-report.md（verdict: ACCEPTED） | verify-archive.sh PASS + AC1-AC10 全过 |
| S6-交付 | PR 合入 master（squash + auto-merge） | CI 全绿 |
| S6-归档 | change 移入 archive/ + 域文档同步（本 change 即域文档同步） | s7_archived |

**实施约束：**
- 单 PR 一次性提交所有 spec-s{XX}.md（避免分支长期在 review 状态）
- 拆分 commit 信息：`refactor(d7): split spec.md per S (DM-20260630-002)`
- 关联：`D7-S2` (本 change 涉及 d7 编排层文档治理场景)
- 拆分前 backup：`git tag d7-spec-pre-split-2026-06-30` 便于回滚

## 6. Risks & Mitigations

详见 `demand.md` §6。补充：

- **风险:** d7 spec.md 仍有增量（v7.0 之后可能继续追加）→ 拆分后单 S 仍可能累积
- **缓解:** 实施时同时在 S3-Gate 中明示"任何新增 Scenario 按 S 归属写到 spec-s{XX}.md，不再回写主 spec.md"作为 follow-up 约束

## 7. Out of Scope

- `design.md` 拆分（d7 当前 841 行）—— 列入 backlog `devrix-d7-design-split`
- `t-registry.md` 拆分（d7 当前 1133 行）—— 列入 backlog `devrix-d7-tregistry-split`
- d2 / d3 / d4 域文档拆分 —— 列入 backlog `devrix-d{2,3,4}-*-split`
- CI 工具 `verify-spec-links.sh` 开发 —— 后续单独立项
- D7 新功能开发 —— 与本 change 完全无关
- D6 / D1 / D5 域文档 —— 当前均 ≤ 800 行，无需拆分

## 8. Reference

- `openspec/specs/project/architecture-design.md §6.4`（本批次同步升级）
- `openspec/specs/project/archiving.md §2.5`（本批次同步升级）
- `openspec/specs/d7-orchestration/spec.md`（拆分对象，2622 行）
- `openspec/specs/d7-orchestration/d7-domain.md`（域内已有拆分范例可参考）
