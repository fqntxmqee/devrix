# Project Specification Delta — architecture-design.md 六段式规范升级

**Change ID:** devrix-architecture-design-six-segment-migration
**Demand ID:** DM-20260629-007
**Delta Type:** MODIFIED (architecture-design.md §1.2/§1.3/§4/§8 升级)
**SOT:** `openspec/specs/project/architecture-design.md`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. §1.2 "应参照" → "**必须遵循**"，列出六段式标题与符号（①-⑥）| `architecture-design.md §1.2` (MODIFIED) | P0 | design.md 强制遵循 detail-design 六段式 |
| 2. §1.3 删"非架构级变更可跳过六段式"，改"**范围与详细度裁剪**"（小型/中型/大型 Change 详细度可裁剪但章节不可省略）| `architecture-design.md §1.3` (MODIFIED) | P0 | 禁止旧式 7 段模板替代六段式 |
| 3. §4 design.md 模板改为**六段式**（①-⑥）+ 附录自由组织 | `architecture-design.md §4` (MODIFIED) | P0 | 所有未来 design.md 必须按六段式 |
| 4. §8 S3 checklist 加"**六段式完整性**"+"**六段式非空**" 2 项校验 | `architecture-design.md §8` (MODIFIED) | P0 | 评审自动校验六段式合规 |

---

## 2. ADDED Requirements

### Requirement: §1.2 六段式框架强制 ✅ IMPLEMENTED

`architecture-design.md §1.2` MUST 标注"**必须遵循**"（must 而非 should），列出六段式标题与符号（① 架构目标 / ② 架构原则 / ③ 业务流程 / ④ 领域模型 / ⑤ 核心链路图 / ⑥ 接口/API 设计），与 `docs/methodology/detail-design-framework.md` 完全一致。

**Priority:** P0
**T:** PROJECT-S1-A01-T01
**Design:** `openspec/changes/devrix-architecture-design-six-segment-migration/design.md §①`

<!-- T: PROJECT-S1-A01-T01 -->

#### Scenario: §1.2 "必须遵循" 语义升级

- GIVEN `architecture-design.md §1.2` 原内容"复杂架构文档应参照 detail-design-framework.md 六段式"
- WHEN 修订为"**所有 design.md 必须遵循**"
- THEN `grep "必须遵循" architecture-design.md` ≥ 1 处命中
- AND `grep "应参照" architecture-design.md` = 0 处命中（弱语义移除）
- AND 六段标题（①架构目标 ②架构原则 ③业务流程 ④领域模型 ⑤核心链路图 ⑥接口/API 设计）全部列出

---

### Requirement: §1.3 范围与详细度裁剪（禁止旧式 7 段模板）✅ IMPLEMENTED

`architecture-design.md §1.3` MUST 改"非架构级变更可跳过六段式"为"**范围与详细度裁剪**"，明确小型 / 中型 / 大型 Change 详细度可裁剪，但**章节不可省略**。

**Priority:** P0
**T:** PROJECT-S1-A02-T01
**Design:** `openspec/changes/devrix-architecture-design-six-segment-migration/design.md §①`

<!-- T: PROJECT-S1-A02-T01 -->

#### Scenario: §1.3 范围裁剪 + 禁止 7 段模板

- GIVEN `architecture-design.md §1.3` 原内容"非架构级变更可跳过六段式"
- WHEN 修订为"**范围与详细度裁剪**"
- THEN `grep "范围与详细度裁剪" architecture-design.md` ≥ 1 处命中
- AND `grep "可跳过六段式" architecture-design.md` = 0 处命中（豁免口子移除）
- AND §1.3 明确列出小型 / 中型 / 大型 Change 详细度参考
- AND §1.3 明确标注"**禁止用 §1 Root Cause Analysis / §2 Solution Design / §3 Key Interfaces 等旧式 7 段模板替代六段式（2026-06-29 规范升级，已归档 Change 不追溯）**"

---

### Requirement: §4 design.md 模板六段式 ✅ IMPLEMENTED

`architecture-design.md §4` MUST 改为**六段式模板**（①架构目标 ②架构原则 ③业务流程 ④领域模型 ⑤核心链路图 ⑥接口/API 设计），与 `detail-design-framework.md` 完全对齐。附录（File Manifest / Rollback Plan / 回归风险 / S3 Checklist / 下一步）可自由组织，**不属于六段式主体**。

**Priority:** P0
**T:** PROJECT-S1-A03-T01
**Design:** `openspec/changes/devrix-architecture-design-six-segment-migration/design.md §④ + §⑥`

<!-- T: PROJECT-S1-A03-T01 -->

#### Scenario: §4 模板六段式 + 附录自由组织

- GIVEN `architecture-design.md §4` 原内容 7 段模板（Root Cause / Solution / Key Interfaces / Data Flow / File Manifest / Regression Risk / Rollback）
- WHEN 修订为六段式模板
- THEN `grep "^## ①" architecture-design.md` ≥ 6 处命中（六段齐全）
- AND §4 模板的六段标题与 `detail-design-framework.md` 完全一致
- AND §4 模板注明"附录可按需裁剪（小型 Change 可合并 A+C 至一附录），但**主体六段不可省略、不可改名**"
- AND `grep "^## 1. Root Cause Analysis" architecture-design.md` = 0 处命中（旧式 7 段模板移除）

#### Scenario: reference Change 合规验证

- GIVEN reference Change `devrix-d7-taskcontract-unification` design.md 已按新六段式落地
- WHEN 校验六段齐全性
- THEN `grep "^## ①" openspec/changes/devrix-d7-taskcontract-unification/design.md` ≥ 6 处命中
- AND 每段至少有 3 行实质内容（小型 Change 可放宽至 1-2 行概要）

---

### Requirement: §8 S3 checklist 加六段式校验 ✅ IMPLEMENTED

`architecture-design.md §8` S3 checklist MUST 加"**六段式完整性**"和"**六段式非空**" 2 项校验。

**Priority:** P0
**T:** PROJECT-S1-A04-T01
**Design:** `openspec/changes/devrix-architecture-design-six-segment-migration/design.md §⑤ + §⑥`

<!-- T: PROJECT-S1-A04-T01 -->

#### Scenario: §8 S3 checklist 加 2 项校验

- GIVEN `architecture-design.md §8` S3 checklist 原内容（7 项）
- WHEN 加 2 项校验
- THEN `grep "六段式完整性" architecture-design.md` ≥ 1 处命中
- AND `grep "六段式非空" architecture-design.md` ≥ 1 处命中
- AND "六段式完整性"项明确列出 6 段（①-⑥）必须齐全 + 与 detail-design-framework.md 完全一致
- AND "六段式非空"项明确每段 ≥ 3 行实质内容（小型 Change 可放宽至 1-2 行概要，但禁止 "TBD" / "TODO" / 空标题）

---

## 3. 行为不变保证

- **`docs/methodology/detail-design-framework.md`**：完全不变（53 行，六段式定义源头）
- **已归档 18+ Change**：保持 7 段模板（AC5 / Decision 1 不追溯）
- **进行中 4 个 Change**：
  - `devrix-d7-taskcontract-unification`（已合规）
  - `devrix-d7-multiturn-session-state`（spec delta 轻量路径，自然过渡）
  - `devrix-d7-mups-v4-5node-coverage-orchestration`（spec delta 轻量路径，自然过渡）
  - `devrix-d7-6s-observe-merge-cancel`（s1_cancelled，无需处理）
- **`master.md` 流程定义**：不变
- **`git-workflow.md` / `coding.md` / `testing.md`**：不变（本次仅升级 architecture-design.md）

---

## 4. 跨域边界影响

| 域 | 影响 | 备注 |
|----|------|------|
| 所有 D{N} Change | design.md 必须按六段式（未来 Change 强制）| 由 §8 checklist 校验 |
| 历史 18+ 已归档 Change | 不追溯（AC5）| 保持 7 段模板 |
| 项目规范体系 | 规范源头与模板对齐（消除 §1.2 vs §4 内部矛盾）| 强一致性 |

---

## 5. Out of Scope（本 Change 不做）

- 修改 `docs/methodology/detail-design-framework.md`（六段式定义源头，不变）
- 回填已归档 18+ Change 的 design.md（AC5 不追溯）
- 强制 active 4 个 Change 回填（AC6 自然过渡）
- 修改 `master.md` / `git-workflow.md` / `coding.md` / `testing.md`（其他子规范，本次范围外）
- 引入 v2 六段式（如新增 "⑦ 风险与缓解" 段）（v2.0 远期规划）

---

## 6. 关联变更

- **`devrix-d7-taskcontract-unification`**（DM-20260629-006）：触发本 Change 的评审依据 + reference 合规 Change
- **`devrix-d7-dsaft-restructuring`**（DM-20260629-001）：v6.0.x 收官，7 段模板历史代表
- **`devrix-d1-ac-restructuring`**（DM-20260629-005）：D1 AC 重构，7 段模板历史代表
- 前置归档：`openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/`（v6.0.x 维护阶段收官）
- memory 记录：`~/.claude/projects/-Users-fukai-workspace/memory/devrix-architecture-design-six-segment-upgrade-2026-06-29.md`