# 架构设计规范

**版本:** 1.2.0
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

### 1.2 六段式框架（强制）

**所有 design.md 必须遵循** `../../docs/methodology/detail-design-framework.md` 六段式，章节标题与符号必须与 detail-design-framework.md 一致：

1. **① 架构目标** — 业务目标（解决哪些痛点）+ 技术目标（量化指标：QPS/RT/Coverage/P99）+ 约束条件（SemVer / 合规 / 灰度）
2. **② 架构原则** — 设计原则（10 条以内）+ 命名规范（ID/Type/Error/Span 模板）+ 代码风格（函数 < 50 行 / 文件 < 800 行）
3. **③ 业务流程** — 核心用例时序图 + 异常补偿（Fallback 路径表）+ 分支处理决策树
4. **④ 领域模型** — 聚合根（4 个以内）+ 限界上下文（包边界图）+ 领域事件（Span/Metric 列表）+ 跨域消费模型
5. **⑤ 核心链路图** — 端到端路径 + 时序标注（SLA/P99）+ 单点风险与缓解
6. **⑥ 接口/API 设计** — 风格（Pure types / Builder / With*）+ 契约（错误码三元组 + TraceID）+ 幂等 + 版本演进

附录可自由组织（File Manifest / Rollback Plan / 回归风险 / S3 Checklist / 下一步），不属于六段式主体。

### 1.3 范围与详细度裁剪

六段式是结构骨架，**各段详细度可按 Change 规模裁剪**，但**章节不可省略**：
- **小型 Change**（< 5 AC / < 1 PR）：每段 1-3 行概要 + 关键示例
- **中型 Change**（5-15 AC / 1-3 PR）：每段 5-20 行 + 时序图 / 表格
- **大型 Change**（> 15 AC / 多 PR 跨域）：每段 20+ 行 + 完整时序图 + 决策树 + 风险表

**禁止**用 §1 Root Cause Analysis / §2 Solution Design / §3 Key Interfaces 等旧式 7 段模板替代六段式（2026-06-29 规范升级，已归档 Change 不追溯）。

---

## 2. .openspec.yaml 模板

```yaml
change_id: devrix-{module-name}
priority: P0 | P1 | P2
demand_id: DM-YYYYMMDD-NNN
# 流程阶段 S1–S6；下列为 .openspec.yaml 元数据状态（小写 snake_case）
# s7_archived = 归档完成终端态（对应 S6-归档，不是第八流程阶段）
# s1_cancelled / s0_abandoned = 取消或放弃（见 archiving.md §5）
status: s2_design | s3_design | s4_implementation | s5_acceptance | s7_archived | s1_cancelled
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

### 2.1 status 枚举（元数据，非流程阶段编号）

| 值 | 含义 | 对应流程 |
|----|------|----------|
| `s2_design` | 提案阶段 | S2 |
| `s3_design` | 设计阶段 | S3 |
| `s4_implementation` | 实现阶段 | S4 |
| `s5_acceptance` | 验收阶段 | S5 |
| `s7_archived` | 已归档（终端态） | S6-归档完成 |
| `s1_cancelled` | 已取消 | 见 `archiving.md` §5 |

`proposal.md` / `design.md` 头部 **Status** 使用可读形式（如 `S3_Design`、`Archived`），须与 `.openspec.yaml` 的 `status` 语义一致。

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

## 4. design.md 模板（六段式 — 与 detail-design-framework.md 一致）

```markdown
# Design: <标题>

**Change ID:** <change-id>
**Demand ID:** DM-YYYYMMDD-NNN
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** YYYY-MM-DD

---

## ① 架构目标
- 业务目标（解决哪些痛点，列出对应 AC）
- 技术目标（量化指标：P99 / Coverage / QPS 等）
- 约束条件（SemVer / 合规 / 灰度 / Pure types / 错误码闭合）

## ② 架构原则
- 设计原则（10 条以内，每条对应落地方式 + AC）
- 命名规范（DSAFT ID / Type / Error Code / Span Op / Metric 模板）
- 代码风格（函数 < 50 行 / 文件 < 800 行 / 异常不过模块边界）

## ③ 业务流程
- 核心用例时序图（Downlink + Uplink 端到端）
- 异常补偿（Fallback 路径表 + 触发条件 + 幂等保障）
- 分支处理决策树（资源耗尽 / 异常码 / 重试退避）

## ④ 领域模型
- 聚合根（4 个以内，标注职责 + 不可变性）
- 限界上下文（包边界图 + 白名单）
- 领域事件（Span / Metric 列表）
- 跨域消费模型（D2/D4/D6 boundary contract）

## ⑤ 核心链路图
- 端到端路径（每跳节点 + SLA 承诺 + P99 上限）
- 时序标注（识别瓶颈节点）
- 单点风险与缓解（每个单点对应 AC）

## ⑥ 接口 / API 设计
- 风格（Pure types / Builder / With* 不可变）
- 契约（错误码三元组 Code + Message + Remediation + TraceID 全链路）
- 幂等保障表
- 版本演进路径（v1.0 / v1.1 / v2.0）

---

## 附录（自由组织，不属于六段式主体）
- 附录 A：File Manifest（新增 / 修改 / 删除文件清单）
- 附录 B：Rollback Plan（多层回滚机制 + 触发条件）
- 附录 C：回归风险评估（baseline 对比 + 高风险改动点 + 测试策略）
- 附录 D：S3 检查清单自检 + **S3-Gate Review 结论**（Approved / Changes Requested；架构级变更须含 Grill Review 要点）
- 附录 E：下一步
```

**附录可按需裁剪**（小型 Change 可合并 A+C 至一附录），但**主体六段不可省略、不可改名**。

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

### 6.4 文档规模约束（强制）

所有 `openspec/specs/` 下的域文档必须遵循以下规模上限。**超过上限的 PR 在 S3-Gate / S4-Gate 一律阻断**，须先拆分再合入。

| 文档 | 软上限（推荐） | 硬上限（强制） | 超限处理 |
|------|---------------|---------------|----------|
| `spec.md`（Gherkin 域规） | 600 行 | **800 行** | 按 S 拆分：`spec-s{XX}.md` + 主 `spec.md` 仅含索引 |
| `design.md`（六段式） | 500 行 | **800 行** | 按 ④领域模型 / ⑤核心链路图 子拆分：引用 `design-{subsection}.md` |
| `t-registry.md` | 400 行 | **500 行** | 按 A 拆分：`t-registry-a{XX}.md` + 主表 |
| `a-registry.md` / `f-registry.md` | 400 行 | **600 行** | 按 D 跨域切分到子注册表 |
| `layer-delta.md` / `d{N}-domain.md` | 300 行 | **500 行** | 按主题拆分为 `*-{topic}.md` |
| 项目级规范（`specs/project/*.md`） | 200 行 | **300 行** | 拆分为多个子规范文件 |

**拆分原则：**

1. **主文档 = 索引 + 跨章节内容**（≤ 200 行）
2. **子文档 = 单一主题**（按 S / A / 子领域切分）
3. **引用方式**：`详见 [spec-s08.md](spec-s08.md)`（相对路径，不引外部 URL）
4. **拆分不破坏 DSAFT ID 体系**（Requirement / Scenario / T 编号全局唯一，跨文件仍可追溯）
5. **归档时域文档同步**（见 `archiving.md` §2.4）按子文档粒度逐个评估

**反模式（禁止）：**

- ❌ 单 `spec.md` 累积所有 S 的 Requirement/Scenario（d7-orchestration 已 2622 行）
- ❌ 拆分后主文档不维护索引，沦为"挂载页"
- ❌ 跨文件复制 Requirement 文本（必须引用，复制导致双源不一致）
- ❌ 用"附录"无限追加大章节（应改为独立子文档）

**S3-Gate 检查项**：

- [ ] `specs/d{N}-*/spec.md` ≤ 800 行（硬上限）
- [ ] `specs/d{N}-*/design.md` ≤ 800 行（硬上限）
- [ ] `specs/d{N}-*/t-registry.md` ≤ 500 行（硬上限）
- [ ] 主文档含有效索引链接到子文档

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
- [ ] **六段式完整性**：`design.md` 主体包含 ①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计 六段（章节标题与符号与 detail-design-framework.md 完全一致，**不可改名、不可省略**）
- [ ] **六段式非空**：每段至少有 3 行实质内容（小型 Change 可放宽至 1-2 行概要，但禁止 "TBD" / "TODO" / 空标题）
- [ ] `dsaft_activities` 已标注涉及的活动 ID
- [ ] `design.md` 明确每个 A 的 F 编排关系（A↔F，可在 ④领域模型或附录）
- [ ] `specs/*/spec.md` 包含所有 Gherkin Scenario
- [ ] 每个 Requirement 有对应的 T 层注释
- [ ] 重大决策已记录（Decision 节，通常在 proposal.md §3 或 design.md）
- [ ] **S3-Gate Review 结论**已写入 design.md 附录 D（Approved / Changes Requested；架构级变更含 Grill Review 要点）
- [ ] Draft PR 已创建
