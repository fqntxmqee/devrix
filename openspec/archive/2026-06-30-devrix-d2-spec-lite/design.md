# Design: d2 上下文引擎 spec 精简

**Change ID:** devrix-d2-spec-lite
**Demand ID:** DM-20260630-004
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-30

---

## ① 架构目标

**业务目标（痛点）**

- d2 spec.md 已 **1622 行**（含 66 Requirement / 96 Scenario 详细文本），是 backlog 中最大目标
- 8 个 ADDED Requirements 段（V2/V3/V4/V5/V6/V6-v2/V7/TOOL-SURFACE-1/S16）累计 1590 行历史过程需求
- 违反 DM-20260630-003 已生效的 lite-mode 规范（spec.md 硬上限 200 行）
- 用户原始诉求：specs 域文档 = 精简设计契约（最新符合代码），过程需求迭代走 archive/

**技术目标（量化指标）**

- `openspec/specs/d2-context-engine/spec.md` ≤ 200 行（精简设计契约，170-200 行目标）
- `openspec/specs/d2-context-engine/CHANGELOG.md` ≤ 300 行（时间线摘要，80-150 行目标）
- 96 个原 Scenario 全部留 archive（0 丢失）
- 不动 Go 代码（`git diff --stat internal/` = 0）
- 不动 d2 其他 12 个子文档（`git diff --stat openspec/specs/d2-context-engine/` 仅 spec.md + 新 CHANGELOG.md）

**约束条件**

- SemVer：lite-mode 规范已生效（DM-20260630-003），本 change 不升规范版本号
- 合规：OpenSpec 流程 S1-S6 完整闭环；d2-domain.md v9.0.0 已 S7_Archived 不动
- 灰度：仅示范 d2，其他域不强推
- 错误码闭合：N/A（纯文档变更）
- 不可变性：CHANGELOG.md 仅追加行，不修改历史

## ② 架构原则

**设计原则（10 条以内）**

1. **复用 lite-mode 模式**（d7 spec-lite 同形，验证已成熟）
2. **d2-domain.md v9.0.0 是 SoT，spec.md 是契约**（双层文档清晰分工）
3. **canonical S 仅 4 个**（S15 Prepare / S17 Persist / S18 Enforce，S19 拆解/S20 移除留 archive）
4. **不创建 spec-s{XX}.md / spec-{topic}.md 子文件**（lite-mode 硬约束）
5. **检索路径固定**：spec.md → CHANGELOG.md → archive/<change>/specs/
6. **写入路径固定**：change 实现 → 修订 spec.md（如架构级）→ 归档时 CHANGELOG.md 追加
7. **d2 其他 12 个子文档**全部不动（a/t/f-registry / design / span-registry / dsaf / observability-guide / prompt-system-design / terminal-state-guide / layer-delta / d7-boundary / d2-domain）
8. **跨域一致性**：规范升级对所有域立即生效（d1/d3/d4/d5/d6 视需求跟进）
9. **0 行为变更**（纯文档规范升级，不动 Go 代码、不改业务逻辑）
10. **复用 d7 lite-mode 6 AC 模式**（spec.md / CHANGELOG.md / 0 子文件 / 0 Go diff / 0 d2 子文档 diff / verify-archive）

**命名规范**

- Change ID: `devrix-d2-spec-lite`（小写连字符）
- 文件名: `spec.md`（主契约）/ `CHANGELOG.md`（时间线）
- Demand ID: `DM-20260630-004`（lite-mode 推广第一站）

**代码风格**

- 文档单段 ≤ 50 行（changelog 表行除外）
- 文件 ≤ 800 行（项目级规范硬上限 300 行；spec.md 200 / CHANGELOG.md 300）
- 表格列宽一致（4-5 列宽度对齐）

## ③ 业务流程

**核心用例：归档 d2 change 时**

```
S6-归档触发
  ↓
  1. 评估受影响域（cat openspec/changes/<id>/.openspec.yaml | grep domains）
     └─ 本 change 仅 D2 域
  ↓
  2. lite-mode 评估：
     ├─ spec.md 架构级变更？ → 修订契约段（Overview / DSAFT / S 层职责 / 关键 Scenario 范式）
     ├─ CHANGELOG.md 追加？  → 1 行（Date / Change ID / 摘要 / 归档链接）
     ├─ a-registry / t-registry / f-registry 增删？ → 本 change 不动
     └─ design.md / d2-domain.md 增删？ → 本 change 不动
  ↓
  3. cp -r changes/<id>/* → archive/<date>-<id>/
  ↓
  4. git rm -r changes/<id>/
  ↓
  5. 更新 demand-archive-index.md（追加 1 行）
  ↓
  6. ./scripts/verify-archive.sh <id>  PASS
```

**异常补偿（Fallback 路径）**

| 触发条件 | Fallback 路径 |
|----------|--------------|
| spec.md 修订后 > 200 行 | 删除次要段（如详细 V 历史段），仅保留 Overview / 核心原则 / S 层职责 / DSAFT / Scenarios / Architecture / 关键 Scenario 范式 / 关键链路口 |
| CHANGELOG.md 修订后 > 300 行 | 折叠 30 天前条目到「历史归档」段（1 行摘要 + 归档链接） |
| Reviewer 担心 96 Scenario 失追溯 | d2 历史已存 21 个 archive 目录，CHANGELOG.md 通过 change-id 链接引用（不复制） |
| d2-domain.md 与 spec.md 冲突 | spec.md 显式声明 d2-domain.md 是 SoT；spec.md 是契约（精简视图） |

**分支处理决策树**

```
变更类型判定
  ├─ 架构级变更？
  │   ├─ 是 → 修订 spec.md（契约段）
  │   │     ├─ spec.md ≤ 200 行 ✓
  │   │     └─ spec.md > 200 行 → 精简次要段
  │   └─ 否 → 仅追加 CHANGELOG.md（1 行）
  │
  ├─ 跨域影响？
  │   ├─ 是 → 评估所有受影响域（d1/d3/d4/d5/d6/d7）
  │   └─ 否 → 仅 d2 域（本 change 模式）
  │
  └─ 行为变更？
      ├─ 是 → 不允许（拒绝提交）
      └─ 否 → 继续 lite-mode 流程
```

## ④ 领域模型

**聚合根（4 个以内）**

- `spec.md`（d2 域主契约文档） — 唯一不可变主索引
- `CHANGELOG.md`（d2 域时间线） — 仅追加不修改
- `d2-domain.md` v9.0.0（D2-Domain SoT） — 不变（DM-20260629-002 收口）
- `archive/<change>/specs/`（过程需求详细文本） — 不可变历史

**限界上下文（包边界图）**

```
openspec/specs/d2-context-engine/        ← D2 域文档根
  spec.md           [主契约，≤ 200 行，本 change REWRITE]
  CHANGELOG.md      [时间线，≤ 300 行，本 change NEW]
  d2-domain.md      [域 SoT v9.0.0，不变]
  a-registry.md     [活动注册，≤ 600 行，不变]
  t-registry.md     [测试点注册，≤ 500 行，不变]
  f-registry.md     [功能点注册，≤ 600 行，不变]
  design.md         [六段式设计，≤ 800 行，不变]
  d7-boundary.md    [D7 边界契约，≤ 500 行，不变]
  span-registry.md  [Span 注册，≤ 500 行，不变]
  dsaf-architecture.md  [DSAFT 架构，≤ 800 行，不变]
  observability-guide.md  [可观测性指南，≤ 500 行，不变]
  prompt-system-design.md [提示系统设计，≤ 500 行，不变]
  terminal-state-guide.md [终止状态指南，≤ 500 行，不变]
  layer-delta.md    [层级 delta，≤ 500 行，不变]
```

**白名单（13 个文件）**：本 change 仅改 spec.md + 新增 CHANGELOG.md，其他 11 个全部不动。

**领域事件（Span / Metric 列表）**

- N/A（纯文档变更，无 Span / Metric 涉及）

**跨域消费模型**

- D7 消费 D2（PreparedTurnRunner → D7 RunTurn）
- D2 消费 D5（tracker LRU）/ D6（verify）
- D2 触发 D4（delegate_* via hubspoke）
- D2→D3 import 禁止（CI 硬阻断，DM-020 v2.0-d）
- 边界契约：见 `d7-boundary.md`（不变）

## ⑤ 核心链路图

**端到端路径：读 d2 specs 域文档**

```
工程师读 d2 设计
  ↓
  1. 读 spec.md（≤ 200 行，Overview / 核心原则 / S 层职责 / DSAFT / Scenarios / Architecture / 关键 Scenario 范式 / 关键链路口）
  ↓
  2. 跳 d2-domain.md v9.0.0（North Star / 物理路径 / 实现状态 SoT）
  ↓
  3. 跳 CHANGELOG.md（≤ 300 行，时间线列表）
  ↓
  4. 跳 archive/<date>-<change-id>/specs/（完整 Gherkin Scenario 集合）
  ↓
  5. 读 archive/<date>-<change-id>/design.md（该 change 详细设计）
```

**SLA 承诺**：单次检索 ≤ 4 跳（spec.md → d2-domain.md → CHANGELOG.md → archive/）

**时序标注**

| 节点 | SLA 承诺 | P99 上限 |
|------|---------|----------|
| spec.md 加载 | < 1s | 2s |
| d2-domain.md SoT 加载 | < 1s | 2s |
| CHANGELOG.md 加载 | < 1s | 2s |
| archive/ 跳转 | < 2s | 5s |
| 端到端读路径 | < 5s | 11s |

**单点风险与缓解**

| 单点 | 风险 | 缓解 |
|------|------|------|
| CHANGELOG.md 链接失效 | archive 目录可能重命名或删除 | 1) 命名规范 `YYYY-MM-DD-devrix-<id>/` 强约束；2) Backlog 立项 `devrix-verify-spec-links` CI 工具 |
| spec.md 漏修订 | 架构级变更未及时更新契约段 | 1) S3-Gate 检查 `spec.md ≤ 200`；2) reviewer 必查 4 段（Overview / 核心原则 / S 层职责 / 关键 Scenario 范式） |
| d2-domain.md 与 spec.md 冲突 | 双层文档职责不清 | spec.md 顶部声明「d2-domain.md 是 SoT, spec.md 是契约」 |
| TOOL-SURFACE-1 占 spec.md 600+ 行（最大累积） | 评审时焦点漂移 | spec.md 仅保留 1 canonical 范式（如 Materialize 路径），详细 Surface → Runner 表 → archive/2026-06-17-devrix-tool-surface-contract/ |
| 8 个 ADDED Requirements 段（V2-V7）累积 1590 行 | 大量历史过程需求 | CHANGELOG.md 按需精简（> 30 天折叠）；不复制 Scenario 文本 |

## ⑥ 接口 / API 设计

**风格（Pure types / Builder / With*）**

- spec.md 顶部契约段：
  ```markdown
  # Context Engine Specification
  > 当前符合代码的设计契约（v9.0.0）。详细 Scenario 历史见 [CHANGELOG.md](CHANGELOG.md)。
  > SoT: [d2-domain.md](d2-domain.md) v9.0.0 — North Star / 物理路径 / 实现状态
  ## Overview
  ## 核心设计原则（5-8 条）
  ## S 层职责（canonical S15-S18）
  ## DSAFT 结构
  ## Scenarios（4 canonical S 状态表）
  ## Architecture（Leader/Follower 拓扑 + 跨域边界引用）
  ## 关键 Scenario 范式（1-2 canonical）
  ## 关键链路口（4-6 端到端路径）
  ```
- CHANGELOG.md 表格行格式：
  ```markdown
  | YYYY-MM-DD | devrix-d2-<id> | <一句话摘要> | [archive](../../archive/YYYY-MM-DD-devrix-d2-<id>/) |
  ```

**契约（错误码三元组 + TraceID）**

- N/A（纯文档变更）

**幂等保障表**

| 操作 | 幂等性 |
|------|--------|
| 追加 CHANGELOG.md 行 | 幂等（相同内容多次追加 = 1 行） |
| 修订 spec.md 段 | 非幂等（每次修订会覆盖前次），但有 PR review 保障 |
| 跨 archive/ grep Scenario | 幂等（只读操作） |

**版本演进路径**

- v1.0：d2 spec.md 精简设计契约（1622 → ≤ 200）+ CHANGELOG.md NEW
- v1.1：视需求扩展至其他域（d1/d3/d4/d5/d6），每个域独立 change 立项
- v2.0：Backlog `devrix-verify-spec-links` CI 工具上线 + d2 design.md / t-registry.md 拆分

---

## 附录 A：File Manifest

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `openspec/specs/d2-context-engine/spec.md` | REWRITE | 1622 → ≤ 200 | 重写为精简设计契约 |
| `openspec/specs/d2-context-engine/CHANGELOG.md` | NEW | 0 → ≤ 300 | d2 域时间线 |
| `openspec/changes/devrix-d2-spec-lite/.openspec.yaml` | NEW | — | S2 元数据 |
| `openspec/changes/devrix-d2-spec-lite/demand.md` | NEW | — | S1 需求 |
| `openspec/changes/devrix-d2-spec-lite/proposal.md` | NEW | — | S2 提案 |
| `openspec/changes/devrix-d2-spec-lite/design.md` | NEW | — | S3 设计（六段式） |
| `openspec/changes/devrix-d2-spec-lite/tasks.md` | NEW | — | S4 任务分解 |
| `openspec/changes/devrix-d2-spec-lite/acceptance-report.md` | NEW | — | S5 验收 |

**不动文件清单**：
- `openspec/specs/d2-context-engine/` 11 个其他子文档（d2-domain / a-registry / t-registry / f-registry / design / d7-boundary / span-registry / dsaf-architecture / observability-guide / prompt-system-design / terminal-state-guide / layer-delta）
- `openspec/specs/d{1,3,4,5,6}-*/spec.md`（其他域不强推）
- Go 代码 / CI 配置 / 业务逻辑
- 其他项目级规范（lite-mode 已生效）

## 附录 B：Rollback Plan

**触发条件**：
- S4 实现失败（spec.md 修订后 > 200 行 / CHANGELOG.md > 300 行）
- S5 验收 FAIL（go vet / go test 不通过）
- Reviewer 强烈反对 lite-mode（需重新讨论）

**回滚步骤**：
1. `git revert <merge-commit>` 回退 PR 合入
2. 恢复 spec.md 到 1622 行原版（`git checkout origin/master -- openspec/specs/d2-context-engine/spec.md`）
3. 删除 CHANGELOG.md
4. 重新立项（不阻塞 devrix d2 域后续 change）

**多层回滚**：
- L1：S3-Gate 阻断（spec.md / CHANGELOG.md 硬上限检查）
- L2：S4-Gate 阻断（go vet / go test）
- L3：S5 验收 FAIL（AC 不全过）
- L4：PR review 不通过

## 附录 C：回归风险评估

**baseline 对比**：
- 文档：d2 spec.md 重写为精简版（1622 → ≤ 200）
- 时间线：d2 CHANGELOG.md 新建（28+ d2 change 引用）

**高风险改动点**：
1. d2 spec.md 全文重写（66 Requirement / 96 Scenario 文本删除）
2. d2 CHANGELOG.md 新建（28 个 d2 change 引用）
3. d2 canonical S 列表从 8 个 ADDED 段 → 4 个 canonical（S15-S18）

**测试策略**：
- 静态：`wc -l` 硬上限检查（spec.md ≤ 200 / CHANGELOG.md ≤ 300）
- 静态：`grep -c "^#### Scenario:" openspec/specs/d2-context-engine/spec.md` ≤ 2（1-2 canonical 范式）
- 静态：跨 archive/ 全局 grep `#### Scenario:` 总数 = 96（0 丢失）
- 静态：d2 21 个 archive 目录中 grep `#### Scenario:` 总数 = 96
- 动态：`go vet ./...` PASS
- 动态：`go test -race ./... -short` PASS（本 change 不改 Go 代码）

**回归矩阵**：
- 读路径：spec.md → d2-domain.md → CHANGELOG.md → archive/ 5s 内（人工 spot check）
- 写路径：change → spec.md 修订 + CHANGELOG.md 追加 1 行（git diff spot check）
- 兼容性：archive/ 历史 0 触碰（git diff --stat archive/ = 0）
- 兼容性：d2 其他 11 个子文档 0 触碰

## 附录 D：S3-Gate 自评

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 六段式完整性 | ✓ | ①②③④⑤⑥ 全部存在，章节标题与 detail-design-framework.md 一致 |
| 六段式非空 | ✓ | 每段 ≥ 3 行实质内容 |
| `dsaft_scenarios` 标注 | ✓ | D2-S15/S17/S18/S20 |
| `dsaft_activities` 标注 | N/A | 本 change 不涉及 D2 活动变更 |
| A↔F 编排关系 | N/A | 纯文档变更，无 A↔F 关系 |
| `specs/*/spec.md` Gherkin Scenario | ✓ | spec.md 精简版含 1-2 canonical 范式 |
| T 层注释 | N/A | 本 change 不改 T 层 |
| 重大决策记录 | ✓ | §② 记录 d7-lite-mode 复用 vs 按 V 阶段分片 vs 维持原样方案对比 |
| **S3-Gate Review 结论** | **Approved** | 决策证据完整，可推进 S4 |
| Draft PR | ⏳ | 待 S4 完成后创建 |

**决策记录（复用 d7 lite-mode vs 按 V 阶段分片）**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 复用 d7 lite-mode（推荐） | 一致性；复用已验证模式；d7 已示范 | 仍需 1 跳到 archive/ |
| B. 按 V 阶段分片（spec-v2.md, ...） | 阶段清晰 | 子文件持续累积；与 lite-mode 反模式 |
| C. 维持 1622 行不拆分 | 0 改动 | 违反 lite-mode 硬上限 |

**选择**：A（复用 d7 lite-mode）

**理由**：
- d7 已示范并归档，模式成熟
- 用户原始诉求 = specs 域文档只放最新符合代码的设计
- 96 个 Scenario 详细文本对当前开发是噪音
- d2-domain.md v9.0.0 已提供 SoT，spec.md 引用即可

## 附录 E：下一步

1. S4 实现：T-2.1 → T-2.5（切分支 + d2 重写 + CHANGELOG.md + 验证）
2. S5 验收：T-3.1 → T-3.3（写 acceptance-report.md + 跑 verify-archive.sh + 全量回归）
3. S6-交付：T-4.1 → T-4.4（push + PR + auto-merge）
4. S6-归档：T-5.1 → T-5.7（独立 PR，archive/ 收尾）