# Proposal: architecture-design.md 六段式规范升级

**Change ID:** devrix-architecture-design-six-segment-migration
**Demand ID:** DM-20260629-007
**Status:** S6_Archived (2026-06-29)

---

## 1. Background

2026-06-29 在评审 `devrix-d7-taskcontract-unification`（DM-20260629-006）design.md 时发现：**`openspec/specs/project/architecture-design.md` 自身存在矛盾** — §1.2 引用了 `docs/methodology/detail-design-framework.md` 六段式，但 §4 自己定义的 design.md 模板却是另一套（7 段轻量变更），且 §1.3 给了"非架构级变更可跳过六段式"的豁免口子，导致 18+ 已归档 Change（包括 devrix-d7-dsaft-restructuring 等）都按 7 段模板写，无人按六段式实际落地。

详细背景见 `demand.md §1`。

## 2. Problem Statement

### P1 规范内部矛盾

`architecture-design.md §1.2` 与 `§4` 是**冲突的规范**：
- §1.2 要求复杂架构文档遵循 detail-design 六段式（should）
- §4 提供的是 7 段模板（与六段式无关）
- §8 checklist 校验 §4 7 段模板

### P2 模板缺口

已实施的 18+ Change 普遍使用 7 段模板 → **新规范无人遵循**：
- 设计文档缺乏"架构原则"（设计原则 + 命名规范 + 代码风格）
- 设计文档缺乏"领域模型"（聚合根 + 限界上下文）
- 设计文档缺乏"核心链路图"（端到端 + SLA + 单点风险）

### P3 评审不闭环

S3 评审时无"六段式完整性"检查 → 评审者只能凭记忆判断 → 不同 reviewer 标准不一。

### P4 历史 Change 回填成本

18+ 已归档 Change 按 7 段模板写，回填成本（重写 design.md + 重新过 S3-Gate）远大于价值。

### P5 进行中 Change 不合规风险

当前 4 个 active Change 中仅 `devrix-d7-taskcontract-unification` 走完整 S3 流程，其他 3 个走 spec delta 轻量路径。

## 3. Proposed Solution

### 3.1 解决方案总览

**单文件 4 段原子修改**（已完成实施，归档跟踪）：

| § | 修改前 | 修改后 |
|---|--------|--------|
| §1.2 | "复杂架构文档应参照 detail-design 六段式" | "**所有 design.md 必须遵循** detail-design 六段式"+ 列出 ①-⑥ 标题与符号 |
| §1.3 | "非架构级变更可跳过六段式" | "**范围与详细度裁剪**"（禁止旧式 7 段模板替代）+ 标注 "2026-06-29 规范升级" |
| §4 | 7 段模板（Root Cause / Solution / Key Interfaces / Data Flow / File Manifest / Regression Risk / Rollback）| **六段式模板**（①架构目标 ②原则 ③业务流程 ④领域模型 ⑤链路图 ⑥接口/API 设计）+ 附录自由组织 |
| §8 | S3 checklist 校验 §4 7 段模板 | + "六段式完整性" + "六段式非空" 2 项校验 |

### 3.2 与 detail-design-framework.md 对齐

`architecture-design.md` §4 模板的六段标题与 `docs/methodology/detail-design-framework.md` 完全对齐：
- ① 架构目标 — 业务+技术+约束
- ② 架构原则 — 设计+命名+代码风格
- ③ 业务流程 — 时序+异常+分支
- ④ 领域模型 — 聚合根+限界上下文+领域事件+跨域模型
- ⑤ 核心链路图 — 端到端+SLA+单点风险
- ⑥ 接口/API 设计 — 风格+契约+幂等+版本

### 3.3 附录约定

附录（File Manifest / Rollback Plan / 回归风险 / S3 Checklist / 下一步）**不属于六段式主体**，可自由组织。

### 3.4 关键决策（6 个 Decision）

#### Decision 1: 不追溯已归档 Change

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | 不追溯已归档（18+ Change 保持 7 段模板）| 0 成本；尊重历史 |
| B   | 全量回填（18+ design.md 改六段式 + 重新 S3-Gate）| 历史统一 | 工作量大（~3 周）；重审成本 |

**选择:** A
**理由:** 18+ 已归档 Change 是历史交付物，重新评审成本远大于价值。规范升级面向未来。

#### Decision 2: 进行中 Change 自然过渡

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | 进行中 4 个 Change 走自然过渡（不强制回填）| 不阻塞在途工作 |
| B   | 强制所有 active Change 重新按六段式落地 | 规范立即生效 | 阻塞在途工作（最坏情况重写 3 个 design.md）|

**选择:** A
**理由:** 4 个 active Change 中仅 1 个走完整 S3 流程（`devrix-d7-taskcontract-unification` 已合规），其他 3 个走 spec delta 轻量路径。阻塞在途工作得不偿失。

#### Decision 3: §4 模板完全对齐 detail-design-framework.md

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | §4 模板完全对齐 detail-design-framework.md（六段标题 + 符号 + 内容要求完全一致）| 单一规范源头；强一致性 |
| B   | §4 保留 §1.2 引用但模板仍自定义 | 引用强但模板不一致（历史问题）|

**选择:** A
**理由:** §1.2 引用了 detail-design-framework.md，§4 模板必须与之对齐才能消除"规范内部矛盾"。否则规范升级是空话。

#### Decision 4: §1.3 改为"范围与详细度裁剪"

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | §1.3 改"范围与详细度裁剪"（小型/中型/大型 Change 详细度可裁剪，但章节不可省略）| 保留可裁剪性 + 禁止绕过 |
| B   | §1.3 完全删除（强制六段式无例外）| 强一致性 | 失去小型 Change 灵活性 |

**选择:** A
**理由:** 范围与详细度裁剪是合理的（小型 Change 不需要 50 行端到端时序图），但章节不可省略（否则又退回 7 段混乱）。

#### Decision 5: §8 S3 checklist 加 2 项校验

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | §8 加"六段式完整性"（6 段齐全）+ "六段式非空"（每段 ≥ 3 行实质内容）2 项 | 强制规范；可验证 |
| B   | §8 加"六段式完整性" 1 项 | 强制规范 | 不查空段（可 TBD/TODO 绕过）|

**选择:** A
**理由:** 仅检查完整性不够（"## ① 架构目标\n\nTBD" 也算"完整"），必须加"非空"校验。

#### Decision 6: 本 Change 无 PR（规范已直接修改）

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | 本 Change 无 PR（规范已直接修改，归档即可）| 0 PR 成本；归档凭证 |
| B   | 本 Change 走完整 PR 流程 | 标准流程 | 冗余（修改已实施）|

**选择:** A
**理由:** 规范文件修改是一次性原子操作（4 段同时改），已通过本 Change 评审直接实施。归档凭证 + memory 记录足以追溯。

## 4. Success Metrics

| 指标 | 目标值 | 验证方式 |
|------|-------|---------|
| `architecture-design.md` 总行数 | 200 → ~250 | `wc -l` |
| §1.2 "必须遵循" 出现 | ≥ 1 处 | `grep "必须遵循"` |
| §1.3 "范围与详细度裁剪" 出现 | ≥ 1 处 | `grep "范围与详细度裁剪"` |
| §4 模板六段标题（①-⑥）出现 | 6 处 | `grep "^## ①"` |
| §8 checklist 六段式校验项 | ≥ 2 项 | `grep "六段式完整性"` |
| 已归档 Change（18+）保留 7 段模板 | 18/18 | `grep -l "Root Cause Analysis" archive/` |

## 5. Implementation Plan

### 5.1 实施步骤

```bash
# Step 1: 修改 architecture-design.md（已完成 2026-06-29）
#   - §1.2 改"必须遵循"
#   - §1.3 改"范围与详细度裁剪"
#   - §4 改六段式模板
#   - §8 加六段式校验

# Step 2: 写本 Change 的 S1 demand.md + S2 proposal.md + S3 design.md/specs/tasks.md（归档凭证）

# Step 3: memory 记录（已完成 2026-06-29）
#   - ~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md

# Step 4: 第一个按新六段式落地的 Change 是 devrix-d7-taskcontract-unification（已完成）

# Step 5: S6 归档（本 Change 不需要 PR）
#   - 移动 openspec/changes/devrix-architecture-design-six-segment-migration/
#     → openspec/archive/2026-06-29-devrix-architecture-design-six-segment-migration/
#   - 更新 openspec/demand-archive-index.md 新增 DM-20260629-007 行
#   - 运行 ./scripts/verify-archive.sh devrix-architecture-design-six-segment-migration 12/12 PASS
```

### 5.2 PR 拆分（本 Change 无 PR）

按 Decision 6，本 Change 规范修改已直接实施，无 PR 拆分。归档凭证：
- 规范文件：`openspec/specs/project/architecture-design.md`（200 → 251 行）
- memory 记录：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md`
- reference Change：`openspec/changes/devrix-d7-taskcontract-unification/design.md`（648 行，六段式落地实例）

## 6. Risks & Mitigations

| 风险 | 缓解 | 状态 |
|------|------|------|
| 修改规范文件影响范围广 | AC1 强制语义 + AC4 checklist 校验 | ✅ 已实施 |
| 已归档 Change 回填成本 | AC5 / Decision 1 不追溯 | ✅ 已决议 |
| 新作者学习曲线 | detail-design-framework.md 53 行 + 本 Change design.md 作 reference | ✅ 已实施 |
| 进行中 Change 不合规 | AC6 / Decision 2 自然过渡 | ✅ 已决议 |
| §1.2 / §1.3 / §4 内部一致性 | 单文件 4 段修改原子完成 | ✅ 已实施 |

## 7. Out of Scope

- `docs/methodology/detail-design-framework.md`（模板源头，不变）
- 已归档 18+ Change 的 design.md（不追溯，AC5）
- 进行中 3 个走 spec delta 轻量路径的 Change（不强制，AC6）
- `master.md` 流程定义（不变）
- `git-workflow.md` / `coding.md` / `testing.md` 等其他子规范（本次仅升级 architecture-design.md）

## 8. 关联变更

- `devrix-d7-taskcontract-unification`（DM-20260629-006）— 触发规范升级的 Change
- `devrix-d7-dsaft-restructuring`（DM-20260629-001）— v6.0.x 收官，7 段模板历史代表
- `devrix-d1-ac-restructuring`（DM-20260629-005）— D1 AC 重构，7 段模板历史代表
- 前置归档：`openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/`（v6.0.x 维护阶段收官）

## 9. 备注

- 本 Change 实施时已同步修订 `architecture-design.md`（2026-06-29 用户授权路径 1）
- 规范升级是**不可逆**操作（一旦新 Change 落地，回滚成本高），但**当前没有在途依赖**（4 个 active Change 中 1 个已合规，3 个走轻量路径）
- 本 Change 是规范升级后**第一个**也是**唯一一个**走"无 PR + 直接归档"路径的 Change（特殊性质：规范修改一次性原子操作）