# 架构设计规范

**版本:** 1.1.0
**状态:** Active
**所属阶段:** S2、S3
**关联规范:** `requirements.md`、`review-design.md`、`../../docs/methodology/dsaft-methodology.md`

---

## 1. 设计原则

### 1.1 DSAFT 五层架构（强制）

所有架构设计必须遵循 DSAFT 方法论（详见 `../../docs/methodology/dsaft-methodology.md`）：

```
D 领域 → S 场景 → A 活动 → F 功能点 → T 测试点
```

各阶段 DSAFT 产出：

| OpenSpec 阶段 | DSAFT 产出 | 说明 |
|---------------|-----------|------|
| S2 proposal | D + S | 定位领域和场景 |
| S3 design | A + F | 定义活动和功能点编排 |
| S4 tasks | F 实现 | 任务标注归属 T |
| S5 verify | T 验收 | P0 全绿方可交付 |

S3 设计文档必须包含：
- **领域归属**：本 change 涉及哪个 D，领域类型（核心/支撑/公共）
- **活动定义**：对外暴露哪些 A（A-BE 或 A-FE），输入/输出/状态变更
- **功能点编排**：每个 A 由哪些 F 协作完成

### 1.2 六段式框架

复杂架构文档应参照 `../../docs/methodology/detail-design-framework.md` 六段式：
1. 架构目标 — 业务与技术目标、约束
2. 架构原则 — 设计原则、命名规范
3. 业务流程 — 核心用例、异常补偿
4. 领域模型 — 聚合根、限界上下文
5. 核心链路 — 端到端路径与时序
6. 接口/API 设计 — 契约、幂等、版本

### 1.3 轻量变更

非架构级变更可跳过六段式，但 design.md 必须包含：
- 问题根因
- 方案描述
- 关键代码片段或接口变更
- 回归风险评估

---

## 2. .openspec.yaml 模板

```yaml
change_id: devrix-{module-name}
priority: P0 | P1 | P2
demand_id: DM-YYYYMMDD-NNN
status: s2_design | s3_design | s4_implementation | s5_acceptance | s7_archived
domains: [D1, D2, ...]
dsaft_scenarios: [D{X}-S{X}, ...]        # 涉及的场景 ID
dsaft_activities: [D{X}-S{X}-A{XX}, ...]  # 涉及的活动 ID
t_points: [D{X}-S{X}-A{XX}-T{XX}, ...]
version_scope:
  v1.0: <scope>
  v1.1: <optional>
  v2.0: <optional>
metrics_definitions: []
span_naming: []
```

---

## 3. proposal.md 模板

```markdown
# Proposal: <标题>

**Change ID:** devrix-{name}
**Demand ID:** DM-YYYYMMDD-NNN
**Status:** S2_Design

## 1. Background
## 2. Problem Statement
## 3. Proposed Solution
## 4. Success Metrics
## 5. Implementation Plan
## 6. Risks & Mitigations
## 7. Out of Scope
```

---

## 4. design.md 模板

```markdown
# Design: <标题>

## 1. Root Cause Analysis
## 2. Solution Design
## 3. Key Interfaces / Types
## 4. Data Flow
## 5. File Manifest（新增/修改/删除文件清单）
## 6. Regression Risk Assessment
## 7. Rollback Plan
```

---

## 5. 禁止工时估算

proposal.md、design.md **不得**包含工时估算。理由：
- 设计阶段的估算是方案级的，应与需求分离
- 同一设计可能有多种实现路径，工时放在 tasks.md 跟踪
- 避免「估算 → 承诺」的心理锚定

估算仅出现在 `tasks.md` 中，且仅为参考值。

---

## 6. specs/*/spec.md 规范

### 6.1 格式要求

使用 Gherkin 场景格式：

```markdown
# <Module> Specification

## ADDED

### Requirement: <名称>

#### Scenario: <场景名>
- GIVEN <前置条件>
- WHEN <触发动作>
- THEN <期望结果>
- AND <附加条件>

## MODIFIED
(None)

## REMOVED
(None)
```

### 6.2 Gherkin 编写规则

- 每个 Requirement 至少 1 个 Scenario
- Scenario 使用大写关键词：GIVEN / WHEN / THEN / AND
- THEN 语句包含可验证的具体结果
- 错误路径（sad path）必须有独立 Scenario

### 6.3 T 层映射

每个 Requirement 应在注释中标注关联的 T 层测试点：

```markdown
<!-- T: D4-S1-A01-T01, D4-S1-A01-F01-T01 -->
```

- **T 归属 A**（4 段）：`D{X}-S{X}-A{XX}-T{XX}` — 契约/E2E 级验证
- **T 归属 F**（5 段）：`D{X}-S{X}-A{XX}-F{XX}-T{XX}` — 单元/集成级验证

---

## 7. 设计决策记录

重大架构决策（选择 A 而非 B）必须在 design.md 中记录：

```markdown
### Decision: <决策名称>

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A   | ...  | ...  |
| B   | ...  | ...  |

**选择:** A
**理由:** <1-2 句核心原因>
```

---

## 8. 检查清单

S2 完成前：
- [ ] `.openspec.yaml` 所有字段已填写
- [ ] `dsaft_scenarios` 已标注涉及的 DSAFT 场景 ID
- [ ] `proposal.md` 包含方案对比与风险评估
- [ ] T 层测试点在 T 层注册表预登记（PLANNED）— 根索引 `openspec/t-registry.md`，域明细 `openspec/specs/d{N}-*/t-registry.md`

S3 完成前：
- [ ] `design.md` 包含根因、方案、文件清单、回归风险
- [ ] `dsaft_activities` 已标注涉及的活动 ID
- [ ] `design.md` 明确每个 A 的 F 编排关系（A↔F）
- [ ] `specs/*/spec.md` 包含所有 Gherkin Scenario
- [ ] 每个 Requirement 有对应的 T 层注释
- [ ] 重大决策已记录（Decision 节）
- [ ] Draft PR 已创建
