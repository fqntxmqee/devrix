# Design: specs 域文档轻量化

**Change ID:** devrix-spec-lite-mode
**Demand ID:** DM-20260630-003
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-06-30

---

## ① 架构目标

**业务目标（痛点）**

- d7 spec.md 已 2622 行（含 63 Requirement / 174 Scenario 详细文本），远超 PR #332 合入的 800 行硬上限
- PR #332 思路（按 S 分片）仅解决单文件超 800，未解决 specs/ 整体规模累积
- 用户原始诉求：specs/ 域文档 = 精简设计契约（最新符合代码），过程需求迭代走 archive/

**技术目标（量化指标）**

- `openspec/specs/d7-orchestration/spec.md` ≤ 200 行（精简设计契约，150-180 行目标）
- `openspec/specs/d7-orchestration/CHANGELOG.md` ≤ 300 行（时间线摘要）
- d7 specs 目录总行数 ≤ 500（spec.md 200 + CHANGELOG.md 300）
- 174 个原 Scenario 全部留 archive（0 丢失）
- 不动 Go 代码（`git diff --stat internal/` = 0）

**约束条件**

- SemVer：architecture-design.md v1.2.0 → v1.3.0；archiving.md v1.3.0 → v1.4.0
- 合规：OpenSpec 流程 S1-S6 完整闭环，PR #332 既有 §6.4/§2.5 条款升级为新版本（保留可追溯性）
- 灰度：仅示范 d7，其他域不强推；规范升级对所有域立即生效但 d1/d2/d3/d4 视需求后续拆分
- 错误码闭合：N/A（纯文档变更）
- 不可变性：CHANGELOG.md 仅追加行，不修改历史（无 in-place mutation）

## ② 架构原则

**设计原则（10 条以内）**

1. **specs = 精简设计契约 + 轻量 changelog**（核心原则）
2. **过程需求留在 archive/**（174 个 Scenario 详细文本不进 specs/）
3. **架构级变更才修订 spec.md**（非架构变更只动 CHANGELOG.md）
4. **CHANGELOG.md 每次归档追加 1 行**（Date / Change ID / 摘要 / 状态 / 归档链接）
5. **不创建 spec-s{XX}.md / spec-{topic}.md 子文件**（lite-mode 不需要）
6. **检索路径固定**：spec.md → CHANGELOG.md → archive/<change>/specs/
7. **写入路径固定**：change 实现 → 修订 spec.md（如架构级）→ 归档时 CHANGELOG.md 追加
8. **d{N} 其他子文档**（design.md / a-registry.md / t-registry.md 等）按主题拆分（≤ 800 行）
9. **跨域一致性**：规范升级对所有域立即生效（d1/d2/d3/d4/d5/d6 视需求跟进）
10. **0 行为变更**（纯文档规范升级，不动 Go 代码、不改业务逻辑）

**命名规范**

- Change ID: `devrix-spec-lite-mode`（小写连字符）
- 文件名: `spec.md`（主契约）/ `CHANGELOG.md`（时间线）
- 版本号: SemVer 递增 v1.2.0 → v1.3.0 / v1.3.0 → v1.4.0

**代码风格**

- 文档单段 ≤ 50 行（changelog 表行除外）
- 文件 ≤ 800 行（项目级规范硬上限 300 行）
- 表格列宽一致（4-5 列宽度对齐）

## ③ 业务流程

**核心用例：归档 d7 change 时**

```
S6-归档触发
  ↓
  1. 评估受影响域（cat openspec/changes/<id>/.openspec.yaml | grep domains）
  ↓
  2. 对每个受影响域，lite-mode 评估：
     ├─ spec.md 架构级变更？ → 修订契约段（Overview / DSAFT / Architecture / 关键 Scenario 范式）
     ├─ CHANGELOG.md 追加？  → 1 行（Date / Change ID / 摘要 / 状态 / 归档链接）
     ├─ a-registry / t-registry / f-registry 增删？ → 对应更新
     └─ design.md 增删？ → 按主题拆分（≤ 800 行）
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
| spec.md 修订后 > 200 行 | 删除次要段（如 "版本里程碑" 表），仅保留 Overview / DSAFT / Architecture / 关键 Scenario 范式 |
| CHANGELOG.md 修订后 > 300 行 | 折叠 30 天前条目到「历史归档」段（1 行摘要 + 归档链接） |
| Reviewer 偏好 PR #332 思路 | design.md §② 已记录方案对比（按 S 分片 vs 完全轻量化），决策证据完整 |
| 174 个 Scenario 中部分不在 archive/ | 实施时只删 d7 spec.md 中累积部分，archive/ 历史已存在 0 触碰 |

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
  │   ├─ 是 → 评估所有受影响域（d1/d2/d3/d4/d5/d6）
  │   └─ 否 → 仅 d7 域
  │
  └─ 行为变更？
      ├─ 是 → 不允许（拒绝提交）
      └─ 否 → 继续 lite-mode 流程
```

## ④ 领域模型

**聚合根（4 个以内）**

- `spec.md`（域主契约文档） — 唯一不可变主索引
- `CHANGELOG.md`（域时间线） — 仅追加不修改
- `archive/<change>/specs/`（过程需求详细文本） — 不可变历史
- 其他子文档（a-registry / t-registry / f-registry / design.md） — 按主题拆分

**限界上下文（包边界图）**

```
openspec/specs/d7-orchestration/        ← D7 域文档根
  spec.md          [主契约，≤ 200 行]
  CHANGELOG.md     [时间线，≤ 300 行]
  a-registry.md    [活动注册，≤ 600 行]
  f-registry.md    [功能点注册，≤ 600 行]
  t-registry.md    [测试点注册，≤ 500 行]
  design.md        [六段式设计，≤ 800 行]
  d7-domain.md     [域 SoT，≤ 500 行]
  ...（其他按需）   [主题子文档，≤ 500 行]
```

**白名单**：spec.md / CHANGELOG.md / a-registry.md / f-registry.md / t-registry.md / design.md / d7-domain.md / observability-guide.md / pipeline-architecture.md / span-registry.md / d3-boundary.md / layer-delta.md / d7-requirements-clarifications.md / terminal-state-guide.md / workitem-*.md（17 个子文档，本 change 全部不动）

**领域事件（Span / Metric 列表）**

- N/A（纯文档变更，无 Span / Metric 涉及）

**跨域消费模型**

- 规范升级对所有域立即生效（d1/d2/d3/d4/d5/d6）
- 边界契约：`spec.md ≤ 200` + `CHANGELOG.md ≤ 300` + 其他 d{N} 子文档 ≤ 800
- 不复制 Requirement 文本（引用 archive/，避免双源不一致）

## ⑤ 核心链路图

**端到端路径：读 specs 域文档**

```
工程师读 d7 设计
  ↓
  1. 读 spec.md（≤ 200 行，Overview / DSAFT / Architecture / 关键 Scenario 范式）
  ↓
  2. 跳 CHANGELOG.md（≤ 300 行，时间线）
  ↓
  3. 跳 archive/<date>-<change-id>/specs/（完整 Gherkin Scenario 集合）
  ↓
  4. 读 archive/<date>-<change-id>/design.md（该 change 详细设计）
```

**SLA 承诺**：单次检索 ≤ 3 跳（spec.md → CHANGELOG.md → archive/）

**时序标注**

| 节点 | SLA 承诺 | P99 上限 |
|------|---------|----------|
| spec.md 加载 | < 1s | 2s |
| CHANGELOG.md 加载 | < 1s | 2s |
| archive/ 跳转 | < 2s | 5s |
| 端到端读路径 | < 4s | 9s |

**单点风险与缓解**

| 单点 | 风险 | 缓解 |
|------|------|------|
| CHANGELOG.md 链接失效 | archive 目录可能重命名或删除 | 1) 命名规范 `YYYY-MM-DD-devrix-<id>/` 强约束；2) Backlog 立项 `devrix-verify-spec-links` CI 工具 |
| spec.md 漏修订 | 架构级变更未及时更新契约段 | 1) S3-Gate 检查 `spec.md ≤ 200`；2) reviewer 必查 4 段（Overview / DSAFT / Architecture / 关键 Scenario 范式） |
| archive 目录膨胀 | 174 个 Scenario 分散在 17 个 archive | 1) CHANGELOG.md 按需精简（> 30 天折叠）；2) 不复制 Scenario 文本（仅引用） |
| 规范升级覆盖旧条款 | PR #332 §6.4 / §2.5 被本 change 取代 | 1) SemVer 递增 v1.2.0 → v1.3.0 / v1.3.0 → v1.4.0；2) 在 §6.4 顶部加注释"PR #332 既有条款已升级" |

## ⑥ 接口 / API 设计

**风格（Pure types / Builder / With*）**

- spec.md 顶部契约段：
  ```markdown
  # D7 Orchestration Domain Specification
  > 当前符合代码的设计契约。详细 Scenario 历史见 [CHANGELOG.md](CHANGELOG.md)。
  ## Recent Changes（最近 4 条 + 链接到 CHANGELOG.md）
  ## Overview
  ## DSAFT 结构
  ## 核心设计原则（5-8 条）
  ## Scenarios（17 个 S 层表）
  ## Architecture（5 节点管道 + 域边界）
  ## 关键 Scenario 范式（1-2 canonical 范式）
  ## 关键链路口（4-6 端到端路径）
  ## 附录：总览
  ```
- CHANGELOG.md 表格行格式：
  ```markdown
  | YYYY-MM-DD | devrix-d7-<id> | <一句话摘要> | IMPLEMENTED | [archive](../../archive/YYYY-MM-DD-devrix-d7-<id>/) |
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

- v1.0：specs 域文档 = 精简设计契约 + 轻量 changelog（替代 #332 的"按 S 分片"硬要求）
- v1.1：视需求扩展至其他域（d2/d3/d4/d5/d6），每个域独立 change 立项
- v2.0：Backlog `devrix-verify-spec-links` CI 工具上线 + d7 design.md / t-registry.md 拆分

---

## 附录 A：File Manifest

| 文件 | 类型 | 行数 | 说明 |
|------|------|------|------|
| `openspec/specs/project/architecture-design.md` | MODIFY | 303 → 312 | v1.2.0 → v1.3.0，§6.4 改精简模式 |
| `openspec/specs/project/archiving.md` | MODIFY | 274 → 278 | v1.3.0 → v1.4.0，§2.5 删除 + §2.4 lite-mode |
| `openspec/specs/d7-orchestration/spec.md` | REWRITE | 2622 → 195 | 重写为精简设计契约 |
| `openspec/specs/d7-orchestration/CHANGELOG.md` | NEW | 0 → 103 | d7 域时间线 |
| `openspec/changes/devrix-d7-spec-split/.openspec.yaml` | MODIFY | 29 → 36 | 标 s1_cancelled + 元数据 |
| `openspec/changes/devrix-d7-spec-split/{proposal,design,tasks,demand}.md` | DELETE | — | 仅保留 .openspec.yaml 作为 s1_cancelled 标记 |
| `openspec/changes/devrix-spec-lite-mode/{.openspec.yaml,demand.md,proposal.md,design.md,tasks.md,acceptance-report.md}` | NEW | — | S1+S2+S3+S4+S5 六阶段文档 |

**不动文件清单**：`openspec/specs/d7-orchestration/` 17 个其他子文档 / `openspec/specs/d{1..6}-*/` 其他域 / Go 代码 / CI 配置 / 其他项目级规范

## 附录 B：Rollback Plan

**触发条件**：
- S4 实现失败（spec.md 修订后 > 200 行 / CHANGELOG.md > 300 行）
- S5 验收 FAIL（go vet / go test 不通过）
- Reviewer 强烈反对 lite-mode（需重新讨论）

**回滚步骤**：
1. `git revert <merge-commit>` 回退 PR 合入
2. 恢复 spec.md 到 2622 行原版（git checkout origin/master -- openspec/specs/d7-orchestration/spec.md）
3. 删除 CHANGELOG.md
4. devrix-d7-spec-split 保持 s1_cancelled（不复活原方案）
5. 重新立项（不阻塞 devrix d7 域后续 change）

**多层回滚**：
- L1：S3-Gate 阻断（spec.md / CHANGELOG.md 硬上限检查）
- L2：S4-Gate 阻断（go vet / go test）
- L3：S5 验收 FAIL（AC 不全过）
- L4：PR review 不通过

## 附录 C：回归风险评估

**baseline 对比**：
- 规范文本：architecture-design.md §6.4 升级（v1.2.0 → v1.3.0）
- 文档：d7 spec.md 重写为精简版
- 时间线：d7 CHANGELOG.md 新建

**高风险改动点**：
1. `architecture-design.md §6.4` 全文重写（影响所有未来 S3-Gate 检查）
2. `archiving.md §2.4` 全文重写（影响所有 S6-归档流程）
3. d7 spec.md 全文重写（174 个 Scenario 文本删除）
4. d7 CHANGELOG.md 新建（46 个 d7 change 引用）

**测试策略**：
- 静态：`wc -l` 硬上限检查（spec.md ≤ 200 / CHANGELOG.md ≤ 300）
- 静态：`grep -c "^#### Scenario:" openspec/specs/d7-orchestration/spec.md` = 2（仅 1-2 canonical 范式）
- 静态：跨 archive/ 全局 grep `#### Scenario:` 总数 = 174（0 丢失）
- 动态：`go vet ./...` PASS
- 动态：`go test -race ./... -short` PASS（本 change 不改 Go 代码）

**回归矩阵**：
- 读路径：spec.md → CHANGELOG.md → archive/ 4s 内（人工 spot check）
- 写路径：change → spec.md 修订 + CHANGELOG.md 追加 1 行（git diff spot check）
- 兼容性：archive/ 历史 0 触碰（git diff --stat archive/ = 0）

## 附录 D：S3-Gate 自评

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 六段式完整性 | ✓ | ①②③④⑤⑥ 全部存在，章节标题与 detail-design-framework.md 一致 |
| 六段式非空 | ✓ | 每段 ≥ 3 行实质内容 |
| `dsaft_activities` 标注 | N/A | 本 change 不涉及 D7 活动变更 |
| A↔F 编排关系 | N/A | 纯文档变更，无 A↔F 关系 |
| `specs/*/spec.md` Gherkin Scenario | ✓ | spec.md 精简版含 2 个 canonical 范式 |
| T 层注释 | N/A | 本 change 不改 T 层 |
| 重大决策记录 | ✓ | §② 记录 PR #332 vs 本 change 方案对比 |
| **S3-Gate Review 结论** | **Approved** | 决策证据完整，可推进 S4 |
| Draft PR | ⏳ | 待 S4 完成后创建 |

**决策记录（lite-mode vs PR #332）**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| PR #332 "按 S 分片" | 解决单文件超 800；与 DSAFT 一致 | specs 整体规模仍累积；Scenario 散落 14 文件 |
| **本 change "完全轻量化"** | specs 整体规模受控；Scenario 集中在 archive/ | 主 spec.md 引用需双跳 |

**选择**：完全轻量化

**理由**：
- 用户明确指示"specs 域文档只放最新符合代码的设计"
- 174 个 Scenario 详细文本对当前开发是噪音（已合入代码，需求态已转为代码态）
- CHANGELOG.md + archive/ 双跳可接受（devbrain / Obsidian 等工具链都是这种模式）

## 附录 E：下一步

1. S4 实现：T-2.1 → T-2.7（切分支 + 规范升级 + d7 重写 + 验证）
2. S5 验收：T-3.1 → T-3.3（写 acceptance-report.md + 跑 verify-archive.sh + 全量回归）
3. S6-交付：T-4.1 → T-4.4（push + PR + auto-merge）
4. S6-归档：T-5.1 → T-5.7（独立 PR，archive/ 收尾）
