# Proposal: specs 域文档轻量化

**Change ID:** devrix-spec-lite-mode
**Demand ID:** DM-20260630-003
**Status:** S2_Design
**Replaces:** devrix-d7-spec-split (DM-20260630-002, s1_cancelled)

## 1. Background

PR #332 (2026-06-30 09:01 UTC MERGED) 合入了 `architecture-design.md v1.2.0` §6.4 文档规模约束 + `archiving.md v1.3.0` §2.5 spec.md 按 S 分片合并规则。该方案基于"按 S 分片解决单文件超 800 行"思路。

在按此方案推进 d7-orchestration spec.md 实际拆分时（S3-S5），发现方向偏离用户原始意图：

- 用户希望 `specs/` 域文档 = **精简设计契约**（最新符合代码）
- 详细 Scenario / Requirement 历史走 `archive/<change>/specs/`
- `specs/` 域目录最多放一个**轻量级 changelog**

详见 `demand.md` §1-§2。

## 2. Problem Statement

PR #332 的"按 S 分片"思路未解决根本问题：

| 维度 | PR #332 思路（按 S 分片） | 本 change 思路（精简契约） |
|------|--------------------------|---------------------------|
| d7 spec.md 行数 | 183 ✓ | ≤ 200 ✓（更精简） |
| d7 整体 specs 规模 | 183 + 13×329 + 666 = 5000+ 行 | 150 + 300 = 450 行 |
| Scenario 归宿 | specs/d7/spec-s{XX}.md | archive/<change>/specs/ |
| 检索路径 | 读主 spec.md → 跳 14 个子文件 | 读主 spec.md → 跳 CHANGELOG.md → 跳 archive/<change>/specs/ |
| 维护负担 | 14 个子文件持续同步 | 1 个 spec.md + 1 个 CHANGELOG.md |

**结论**：PR #332 解决了"单文件超 800"症状，但 specs 整体规模仍会持续累积。本 change 转向"精简模式"，把累积彻底关在 archive/ 门外。

## 3. Proposed Solution

### 3.1 总体策略

**specs 域 = 精简设计契约 + 轻量 changelog**，与 PR #332 思路正交：

```
openspec/specs/<domain>/
  spec.md         ≤ 200 行   当前符合代码的设计契约（Overview / DSAFT / 关键设计 / 链路口 / 1-2 关键 Scenario 范式）
  CHANGELOG.md    ≤ 300 行   时间线列表（每个 change 一行 + 一句话摘要 + 链接到 archive/）
  <其他子文档>    ≤ 800 行   a-registry.md / t-registry.md / f-registry.md / design.md 等
```

**过程需求**（63 Requirement / 174 Scenario 详细文本）**不进入 specs/，留在 archive/ 各 change 目录**：

```
openspec/archive/<YYYY-MM-DD>-<change-id>/
  spec.md         完整 Gherkin Scenario 集合（按需展开）
  ...
```

### 3.2 拆分粒度决策

**方案对比：**

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. PR #332 "按 S 分片"（已实施） | 解决单文件超 800 行；与 DSAFT 一致 | specs 整体规模仍累积；Scenario 散落 14 文件 |
| B. **本 change "完全轻量化"**（推荐） | specs 整体规模受控；Scenario 集中在 archive/ | 主 spec.md 引用需双跳（spec.md → CHANGELOG.md → archive） |
| C. 混合：spec.md 极简 + spec-{S}.md 摘要 | 折中 | 双轨维护负担更重 |

**选择:** B（完全轻量化）

**理由:**
- 用户明确指示"specs 域文档只放最新符合代码的设计"
- 174 个 Scenario 详细文本对当前开发是噪音（已合入代码，需求态已转为代码态）
- CHANGELOG.md + archive/ 双跳可接受（devbrain / Obsidian 等工具链都是这种模式）
- 维护负担最低（spec.md 只在架构级变更时更新；CHANGELOG.md 每次归档时追加 1 行）

### 3.3 d7 spec.md 新结构

```markdown
# D7 Orchestration Domain Specification

> 当前符合代码的设计契约。详细 Scenario 历史见 [CHANGELOG.md](CHANGELOG.md) 与 [archive/](../../archive/)。

## Overview
（3-5 段：d7 域职责 / 5 节点管道 / v7.0 TaskContract / 上下游接口）

## DSAFT 结构
（表格：D/S/A/F/T 当前计数 + canonical 5 节点）

## 核心设计原则
（5-8 条 bullet：纯函数 / 不可变 / Pessimistic Commit 5 类触发 / 4 候选规则 / CoW VersionChain / 5 节点 / Trace 树）

## 关键链路口
（4-6 端到端路径：User → D1 → D7 → D2 → D3 → D4 → D5 / 跨域消费 / Trace 树）

## 关键 Scenario 范式
（1-2 个典型 Gherkin 示例：S16-A75 Observe LLM via D2 then D3 作为 canonical 范式）
```

行数目标：150-180 行。

### 3.4 d7 CHANGELOG.md 新结构

```markdown
# D7 Orchestration — Changelog

> 时间线列表。每个 change 一行 + 一句话摘要 + 链接到 archive/。
> spec.md 详细 Scenario 演进看 archive/ 各 change。

| Date | Change ID | 摘要 | 归档 |
|------|-----------|------|------|
| 2026-06-30 | devrix-d7-observe-unified-llm-path | S16-A75 Observe LLM D2→D3 (4 Req/4 T) | [archive](../archive/2026-06-30-devrix-d7-observe-unified-llm-path/) |
| ... | ... | ... | ... |

## 总览
- 当前活跃 Requirement 数：见 a-registry.md / t-registry.md
- 历史 Scenario 详细文本：archive/<change>/specs/
```

行数目标：200-280 行。

## 4. Success Metrics

| 指标 | 目标 | 测量 |
|------|------|------|
| d7 spec.md 行数 | ≤ 200 | `wc -l` |
| d7 CHANGELOG.md 行数 | ≤ 300 | `wc -l` |
| d7 specs 目录总行数 | ≤ 200 + 300 + 其他子文档 = 500 行内 | `du -k` + `wc -l` 合计 |
| spec.md Scenario 范式数 | 1-2 个 | `grep -c "^#### Scenario:"` |
| CHANGELOG.md change 条目数 | ≥ 10（最近 30 天） | `grep -c "^|" CHANGELOG.md` |
| 174 个原 Scenario 全部留 archive | 174 = 174 | 跨 archive/ 全局 grep |
| 不动 Go 代码 | 0 diff | `git diff --stat internal/` |
| 不动 d7 其他子文档 | 0 diff | `git diff --stat openspec/specs/d7-orchestration/`（除 spec.md + 新增 CHANGELOG.md） |
| S5 验收 verdict | ACCEPTED | acceptance-report.md |

## 5. Implementation Plan

| 阶段 | 产出 | 门禁 |
|------|------|------|
| S3 设计 | design.md（六段式 + 新旧方案对比 + rollback） | S3-Gate 通过 |
| S4 实现 | architecture-design.md v1.3.0 + archiving.md v1.4.0 + d7 spec.md 精简 + d7 CHANGELOG.md + 改 devrix-d7-spec-split/.openspec.yaml s1_cancelled | go vet / test-unit / 规范版本号自检 |
| S5 验收 | acceptance-report.md (verdict: ACCEPTED) + 174 Scenario 全部留 archive 验证 | AC1-AC12 全过 |
| S6-交付 | PR 合入 master (squash + auto-merge) | CI 全绿 |
| S6-归档 | mv changes/devrix-spec-lite-mode/ → archive/2026-06-30-devrix-spec-lite-mode/ + demand-archive-index.md 追加 + .openspec.yaml s7_archived + 新 PR | verify-archive.sh PASS |

**单 PR 一次到位**（按 devrix 模式 squash 合并）：本次 PR 包含 S1+S2+S3+S4+S5 全部 commit，最后 S6-归档独立 PR。

## 6. Risks & Mitigations

详见 `demand.md` §6。补充：

- **风险**: 174 个原 Scenario 留 archive 后，可能导致某些 d7 子文档（如 d7-requirements-clarifications.md）需要同步——但本 change 不动 d7 子文档
- **缓解**: 实施时只改 spec.md 主文件，其他 17 个 d7 子文档保持原状；CHANGELOG.md 通过 change-id 链接到 archive/，不复制任何 Scenario 文本

## 7. Out of Scope

- 174 个原 Scenario 的重新整理（按主题或 S 维度归类）—— 留在原 archive/ 位置
- d1/d2/d3/d4/d5/d6 域文档的同步精简 —— 仅示范 d7，其他域视需求后续
- CI 工具 `verify-spec-links.sh` —— 后续单独立项
- d7 design.md (841 行) 拆分 —— Backlog `devrix-d7-design-split`
- d7 t-registry.md (1133 行) 拆分 —— Backlog `devrix-d7-tregistry-split`

## 8. Reference

- `openspec/specs/project/architecture-design.md v1.2.0`（PR #332 合入版，本 change 升级到 v1.3.0）
- `openspec/specs/project/archiving.md v1.3.0`（PR #332 合入版，本 change 升级到 v1.4.0）
- `openspec/changes/devrix-d7-spec-split/`（s1_cancelled 标记）
- `openspec/specs/d7-orchestration/spec.md`（2622 行原版，重写为精简版）
- `openspec/archive/`（所有 d7 change 归档，含完整 Scenario 历史）
