# Tasks: architecture-design.md 六段式规范升级

**Change ID:** `devrix-architecture-design-six-segment-migration`
**Demand ID:** DM-20260629-007
**Status:** S6_Archived (2026-06-29)
**Sprint:** 规范升级（一次性原子操作）
**PR Count:** 0（按 Decision 6，规范文件已直接修改，无 PR 拆分）
**前置:** `devrix-d7-taskcontract-unification`（DM-20260629-006）S3 评审触发
**模板:** `devrix-d7-taskcontract-unification` tasks.md（DM-20260629-006 S3_Design 2026-06-29）

---

## §1 T 总览

| 阶段 | Task | 描述 | 工作量 | 状态 |
| ----- | ---- | ---- | ------ | ---- |
| **Step 1** | T01 | 修改 `architecture-design.md §1.2` "应参照" → "**必须遵循**" + 列出六段标题与符号 | 0.05 天 | ✅ DONE (2026-06-29) |
| **Step 1** | T02 | 修改 `architecture-design.md §1.3` 删豁免 + 加"**范围与详细度裁剪**" | 0.05 天 | ✅ DONE (2026-06-29) |
| **Step 2** | T03 | 重写 `architecture-design.md §4` design.md 模板为六段式（①-⑥）+ 附录 | 0.1 天 | ✅ DONE (2026-06-29) |
| **Step 2** | T04 | 修改 `architecture-design.md §8` S3 checklist 加"六段式完整性"+"六段式非空" 2 项校验 | 0.05 天 | ✅ DONE (2026-06-29) |
| **Step 3** | T05 | 写 memory 记录 + MEMORY.md 索引 +1 行 | 0.05 天 | ✅ DONE (2026-06-29) |
| **Step 3** | T06 | 写本 Change 的 S1 demand.md + S2 proposal.md + S3 design.md/specs/tasks.md | 0.2 天 | ✅ DONE (2026-06-29) |
| **S6 归档** | T07 | 移动本 Change 到 archive/ + 更新 demand-archive-index.md + 运行 verify-archive.sh 12/12 PASS + commit | 0.05 天 | ⬜ 待执行 |

**总计**: ~0.55 天工作量（实际已大部分完成；仅 T07 S6 归档待用户确认后执行）

---

## §2 Step 1：规范文件原子修改（T01-T02，已完成）

### T01 `architecture-design.md §1.2` 强制语义升级 (DONE)

**目标**：把 §1.2 的"应参照"弱语义改为"**必须遵循**"强语义。

```diff
- ### 1.2 六段式框架
-
- 复杂架构文档应参照 `../../docs/methodology/detail-design-framework.md` 六段式：
- 1. 架构目标 — 业务与技术目标、约束
- 2. 架构原则 — 设计原则、命名规范
- 3. 业务流程 — 核心用例、异常补偿
- 4. 领域模型 — 聚合根、限界上下文
- 5. 核心链路 — 端到端路径与时序
- 6. 接口/API 设计 — 契约、幂等、版本

+ ### 1.2 六段式框架（强制）
+
+ **所有 design.md 必须遵循** `../../docs/methodology/detail-design-framework.md` 六段式，章节标题与符号必须与 detail-design-framework.md 一致：
+ 1. **① 架构目标** — 业务目标 + 技术目标 + 约束条件
+ 2. **② 架构原则** — 设计原则 + 命名规范 + 代码风格
+ 3. **③ 业务流程** — 核心用例时序图 + 异常补偿 + 分支处理
+ 4. **④ 领域模型** — 聚合根 + 限界上下文 + 领域事件 + 跨域消费模型
+ 5. **⑤ 核心链路图** — 端到端路径 + 时序标注 + 单点风险
+ 6. **⑥ 接口/API 设计** — 风格 + 契约 + 幂等 + 版本演进
+
+ 附录可自由组织（File Manifest / Rollback Plan / 回归风险 / S3 Checklist / 下一步），不属于六段式主体。
```

**验证**：
- `grep "必须遵循" architecture-design.md` ≥ 1 处
- `grep "应参照" architecture-design.md` = 0 处
- 6 个章节标题（①-⑥）齐全

### T02 `architecture-design.md §1.3` 范围裁剪改造 (DONE)

**目标**：把 §1.3 从"豁免口子"改为"范围与详细度裁剪"。

```diff
- ### 1.3 轻量变更
-
- 非架构级变更可跳过六段式，但 design.md 必须包含：
- - 问题根因
- - 方案描述
- - 关键代码片段或接口变更
- - 回归风险评估

+ ### 1.3 范围与详细度裁剪
+
+ 六段式是结构骨架，**各段详细度可按 Change 规模裁剪**，但**章节不可省略**：
+ - **小型 Change**（< 5 AC / < 1 PR）：每段 1-3 行概要 + 关键示例
+ - **中型 Change**（5-15 AC / 1-3 PR）：每段 5-20 行 + 时序图 / 表格
+ - **大型 Change**（> 15 AC / 多 PR 跨域）：每段 20+ 行 + 完整时序图 + 决策树 + 风险表
+
+ **禁止**用 §1 Root Cause Analysis / §2 Solution Design / §3 Key Interfaces 等旧式 7 段模板替代六段式（2026-06-29 规范升级，已归档 Change 不追溯）。
```

**验证**：
- `grep "范围与详细度裁剪" architecture-design.md` ≥ 1 处
- `grep "可跳过六段式" architecture-design.md` = 0 处
- 列出小型 / 中型 / 大型 Change 详细度参考

---

## §3 Step 2：§4 模板重写 + §8 checklist 加校验（T03-T04，已完成）

### T03 `architecture-design.md §4` design.md 模板重写 (DONE)

**目标**：把 7 段模板改为六段式模板。

```diff
- ## 4. design.md 模板
-
- ```markdown
- # Design: <标题>
-
- ## 1. Root Cause Analysis
- ## 2. Solution Design
- ## 3. Key Interfaces / Types
- ## 4. Data Flow
- ## 5. File Manifest（新增/修改/删除文件清单）
- ## 6. Regression Risk Assessment
- ## 7. Rollback Plan
- ```

+ ## 4. design.md 模板（六段式 — 与 detail-design-framework.md 一致）
+
+ ```markdown
+ # Design: <标题>
+
+ **Change ID:** <change-id>
+ **Demand ID:** DM-YYYYMMDD-NNN
+ **Status:** S6_Archived (2026-06-29)
+ **Parent Proposal:** `proposal.md`
+ **Template:** `docs/methodology/detail-design-framework.md`（六段式）
+ **Created:** YYYY-MM-DD
+
+ ---
+
+ ## ① 架构目标
+ ## ② 架构原则
+ ## ③ 业务流程
+ ## ④ 领域模型
+ ## ⑤ 核心链路图
+ ## ⑥ 接口 / API 设计
+
+ ---
+
+ ## 附录（自由组织，不属于六段式主体）
+ - 附录 A：File Manifest
+ - 附录 B：Rollback Plan
+ - 附录 C：回归风险评估
+ - 附录 D：S3 检查清单自检
+ - 附录 E：下一步
+ ```
```

**验证**：
- `grep "^## ①" architecture-design.md` ≥ 6 处（六段齐全）
- 模板内的六段标题与 detail-design-framework.md 完全一致
- `grep "^## 1. Root Cause Analysis" architecture-design.md` = 0 处（旧式 7 段模板移除）

### T04 `architecture-design.md §8` S3 checklist 加 2 项校验 (DONE)

**目标**：在 S3 checklist 加"六段式完整性"和"六段式非空"。

```diff
  S3 完成前：
+ - [ ] **六段式完整性**：`design.md` 主体包含 ①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计 六段（章节标题与符号与 detail-design-framework.md 完全一致，**不可改名、不可省略**）
+ - [ ] **六段式非空**：每段至少有 3 行实质内容（小型 Change 可放宽至 1-2 行概要，但禁止 "TBD" / "TODO" / 空标题）
  - [ ] `dsaft_activities` 已标注涉及的活动 ID
  - [ ] `design.md` 明确每个 A 的 F 编排关系（A↔F）
  - [ ] `specs/*/spec.md` 包含所有 Gherkin Scenario
  - [ ] 每个 Requirement 有对应的 T 层注释
  - [ ] 重大决策已记录（Decision 节）
  - [ ] Draft PR 已创建
```

**验证**：
- `grep "六段式完整性" architecture-design.md` ≥ 1 处
- `grep "六段式非空" architecture-design.md` ≥ 1 处

---

## §4 Step 3：归档凭证 + memory 记录（T05-T06，已完成）

### T05 memory 记录写入 (DONE)

**目标**：固化升级事实 + 加 MEMORY.md 索引。

**关键文件**：
- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md` (NEW, ~50 行)
- `~/.claude/projects/-Users-fukai-workspace/memory/MEMORY.md` (+1 行索引)

**验证**：
- `grep "六段式升级" MEMORY.md` ≥ 1 处

### T06 本 Change S1-S3 文档 (DONE)

**目标**：写 6 个 OpenSpec 文件作归档凭证。

**关键文件**：
- `openspec/changes/devrix-architecture-design-six-segment-migration/demand.md` (NEW)
- `openspec/changes/devrix-architecture-design-six-segment-migration/proposal.md` (NEW)
- `openspec/changes/devrix-architecture-design-six-segment-migration/.openspec.yaml` (NEW)
- `openspec/changes/devrix-architecture-design-six-segment-migration/design.md` (NEW)
- `openspec/changes/devrix-architecture-design-six-segment-migration/specs/project/spec.md` (NEW)
- `openspec/changes/devrix-architecture-design-six-segment-migration/tasks.md` (NEW, 本文件)

**验证**：
- `wc -l openspec/changes/devrix-architecture-design-six-segment-migration/`.md ≥ 6 文件
- 本 Change design.md 按新六段式落地（grep "^## ①" ≥ 6 处）

---

## §5 S6 归档（T07，待执行）

### T07 S6 归档

**目标**：移动 Change 到 archive/ + 更新索引 + verify-archive + commit。

```bash
# Step 1: 移动 Change 目录
mv openspec/changes/devrix-architecture-design-six-segment-migration/ \
   openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/

# Step 2: 更新 demand-archive-index.md
# 新增 DM-20260629-007 行

# Step 3: 运行 verify-archive.sh
./scripts/verify-archive.sh devrix-architecture-design-six-segment-migration
# 期望：12/12 PASS

# Step 4: commit
git add openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/
git commit -m "chore(openspec): S6 archive devrix-architecture-design-six-segment-migration

规范升级归档凭证（DM-20260629-007）：
- architecture-design.md §1.2/§1.3/§4/§8 六段式强制
- 已归档 18+ Change 不追溯（AC5）
- reference Change: devrix-d7-taskcontract-unification
- memory 记录: devrix-architecture-design-six-segment-upgrade-2026-06-29.md"
```

**验证**：
- `ls openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/` ≥ 6 文件
- `verify-archive.sh` 输出 12/12 PASS
- git log 显示 commit 提交

---

## §6 F-T 映射表

| F (Activity 子活动) | 关联 T 点 | 关联 AC | Phase |
|---------------------|----------|--------|-------|
| F01: §1.2 强制语义改造 | T01 | AC1 | Step 1 |
| F02: §1.3 范围裁剪改造 | T02 | AC2 | Step 1 |
| F03: §4 design.md 模板重写 | T03 | AC3 | Step 2 |
| F04: §8 S3 checklist 加校验 | T04 | AC4 | Step 2 |
| F05: memory 记录 + 索引 | T05 | AC5, AC7 | Step 3 |
| F06: 本 Change S1-S3 文档 | T06 | AC7 | Step 3 |
| F07: S6 归档 | T07 | AC5 | S6 |

---

## §7 风险与缓解（与 demand.md §6 对齐）

| 风险 | T 点 | 缓解 |
|------|------|------|
| 修改规范文件影响范围广 | T01-T04 | AC1 强制语义 + AC4 checklist 校验 |
| 已归档 Change 回填成本 | T05 | AC5 / Decision 1 不追溯（0 成本）|
| 新作者学习曲线 | T03 | detail-design-framework.md 53 行 + 本 Change design.md 作 reference（AC7）|
| 进行中 Change 不合规 | T06 | AC6 / Decision 2 自然过渡（不阻塞）|
| §1.2 / §1.3 / §4 内部一致性 | T01-T04 | 单文件 4 段修改原子完成 |
| 本 Change 自身合规 | T06 | 本 Change design.md 按新六段式落地（"吃自己的狗粮"）|

---

## §8 关联引用

- `demand.md` §3（7 AC + 5 风险）
- `proposal.md` §3.4（6 个 Decision）
- `design.md` §①-§⑥（六段式 + 附录五节）
- `specs/project/spec.md`（4 ADDED Requirement + 5 Gherkin Scenario）
- `.openspec.yaml`（1 scenario + 4 activity + 5 T + 1 metric）
- 前置 Change：`openspec/changes/devrix-d7-taskcontract-unification/`（DM-20260629-006）
- memory 记录：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md`
- 模板源头：`docs/methodology/detail-design-framework.md`（53 行）