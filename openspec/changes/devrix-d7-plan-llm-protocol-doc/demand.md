---
demand-id: DM-20260708-004
title: D7 Plan↔LLM 5 场景输入输出协议沉淀
priority: P1
status: S2_Proposal
dsaft_domain: orchestration
created: 2026-07-08
parent_change: devrix-d7-observe-llm-protocol-doc
parent_demand: DM-20260708-003
origin: |
  从 devrix-d7-observe-llm-protocol-doc (DM-20260708-003) PR #473 S5 后的
  协议沉淀需求。Observe 节点已沉淀 4 kind + 1 混合场景,用户需要按同样
  模板梳理 Plan 节点 — 18 字段 frame / rawStrategicPlan JSON /
  validateStrategicPlan 兜底 / 4 PlanKind 路由 / applySingleModeUncertaintyGate
  fast-path 阻断 (DM-20260706-009)。现有 spec 散落在 strategic_plan_proposer.go
  (524 行) + plan/ 子包 (12 文件) + prompt_dynamic.go:79-104 (StrategicPlanAppendix),
  缺一份统一的"5 场景 I/O 协议" spec。
---

# D7 Plan↔LLM 5 场景输入输出协议沉淀

## 1. 背景

D7 Plan 节点是 MUPS 5 节点流水线的第 2 节点,负责把 Observe 的 `UncertaintyReport`
转成类型化的 `plan.Plan` (4 PlanKind) + `StrategicPlanProposal` (execution_mode
决策),下游 Execute 节点消费。

**关键认知**:
- **Plan↔LLM 协议 ≠ plan.Plan (4 PlanKind) 协议**。LLM 看到的是 `StrategicPlanFrame`
  (18 字段) + `StrategicPlanAppendix` i18n prompt,LLM emit 的是 `rawStrategicPlan`
  (execution_mode / scope_in / child_specs / resolution_strategies / deliverable_contract
  / react_iters_hint / rationale)。`plan.Plan` (4 PlanKind) 是 Go 端 `MatchKind`
  根据 LLM `QuantizedKind` 决策的**确定性构造**,不进 LLM。
- **Plan 节点有两层设计哲学**:
  - **LLM 层**: 决定"如何拆解"(execution_mode)、"做什么"(rationale/deliverable_contract)
  - **Go 层**: 决定"路由到哪个 Channel"(4 PlanKind via MatchKind)
- **Plan↔LLM 没有 observe 那种 11→6 字段过滤**,走 prompttags 反射
  (`pt:"name,plane,omit_X"` struct tag) + `buildStrategicPlanFrame` 显式条件
  (Budget.MaxChildren>0 等),实际 LLM 看到字段数 ~6-15 不等。

现有契约散落在:
- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:65-109` —
  `StrategicPlanFrame` 18 字段 struct + pt tag
- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:117-175` —
  `buildStrategicPlanFrame` 字段过滤条件
- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:323-339` —
  `rawStrategicPlan` JSON wire format
- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:633-649` —
  `validateStrategicPlan` Go 兜底
- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go:723-743` —
  `applySingleModeUncertaintyGate` fast-path bypass
- `internal/layers/contextengine/i18n/prompt_dynamic.go:79-104` — `StrategicPlanAppendix`
- `internal/layers/orchestration/plan/plan.go:30-72` — 4 PlanKind 枚举
- `internal/layers/orchestration/plan/planner.go:116-131` — `MatchKind` 4 Rules
- `internal/layers/orchestration/plan/plan_struct.go:180-262` — `Plan.Validate`
  (PP-1/2/3)
- `internal/layers/orchestration/plan/plan_parse_reject.go` — `ParsePlan`
  DisallowUnknownFields 严格 schema

**契约** = 1 份协议规范(给三方 reviewer / future maintainer / 用户验证用)+ 5 个
trace test (运行验证,Plan 节点当前 **零** trace-level e2e bannered test,对比
observe 16 个是显著 gap)。

## 2. 沉淀目标

产出 1 份 spec doc,**显式**回答以下问题:

| 维度 | 问题 |
|---|---|
| 输入 | LLM 看到哪 18 字段? 字段过滤规则是什么? (区别于 observe 11→6 显式过滤) |
| 输出 | `rawStrategicPlan` JSON schema 是什么? 与 `plan.Plan` (4 PlanKind) 的关系? |
| 兜底 | Go-side `validateStrategicPlan` 做了什么? execution_mode enum / decompose child_specs 必填? |
| 路由 | LLM execution_mode → `QuantizedKind` → `MatchKind` → 4 PlanKind 怎么映射? |
| 场景 1 | `single` + 1 step → CommitmentPlan (direct commit) |
| 场景 2 | `command` + multi-step ≤3 → ProtocolPlan (multi-step async) |
| 场景 3 | `parallel_probe` → ScenarioPlan (read-only probe) |
| 场景 4 | `decompose` + anomalies≥3 → ExplorationPlan (parallel sandbox) |
| 场景 5 (混合) | `single` + 高 UncertaintyMean + hasHighStrengthFact → fast-path bypass `applySingleModeUncertaintyGate` |

## 3. 验收标准

| ID | 标准 | 优先级 | 验证方式 |
|----|------|--------|---------|
| AC1 | spec 覆盖 5 场景, 每场景含 ① 输入协议 ② 期望 LLM 输出 ③ Go 侧处理 ④ 最终路由 | P0 | review |
| AC2 | 18 字段 frame 结构 + 字段过滤规则 (data/control plane + omit 条件) 文档化 | P0 | review |
| AC3 | `rawStrategicPlan` JSON schema + execution_mode 3 选 1 enum 文档化 | P0 | review |
| AC4 | LLM execution_mode → QuantizedKind → MatchKind → 4 PlanKind 映射表明确 | P0 | review |
| AC5 | 混合场景 (single + 高 uncertainty + 高 strength fact) 显式标注 `applySingleModeUncertaintyGate` bypass, 引用 strategic_plan_proposer.go:723 | P0 | review |
| AC6 | 5 个 trace test 全部 PASS (含 TestPlanTraceE2E_SingleModeFastPathBypass 混合场景) | P0 | go test |
| AC7 | trace test 的 stdout 输出本身就是协议契约的"运行验证" | P1 | manual review |

## 4. 依赖与约束

| 类型 | 内容 |
|---|---|
| 依赖 | devrix-d7-observe-llm-protocol-doc (DM-20260708-003, S5_Accepted) |
| 依赖 | devrix-d7-mups-v4-phase2-prb1 (DM-20260623-001, S7_Archived 2026-06-23) — `plan.Plan` 4 PlanKind |
| 依赖 | devrix-d7-mups-v4-phase3-prc2 (DM-20260625-001, S7_Archived 2026-06-23) — 4 Channel router |
| 依赖 | DM-20260706-009 — `applySingleModeUncertaintyGate` fast-path |
| 依赖 | DM-20260704-006 — RC-1 resolution_strategies[] contract |
| 约束 | 不修改任何源代码 — 纯 spec 沉淀 + 5 test 补充 |
| 约束 | spec 章节锚定 file:line (与 d7-observational-fastpath-spec.md 风格一致) |
| 约束 | 不重复 d7-mups-v4-phase2-prb1-archived.md 已覆盖的 4 PlanKind 类型定义 |
| 约束 | 不重复 d7-observational-fastpath-spec.md 已覆盖的 fast-path 闸门契约 |

## 5. 变更范围

### 新增

| 路径 | 描述 |
|------|------|
| `openspec/changes/devrix-d7-plan-llm-protocol-doc/specs/d7-orchestration/d7-plan-llm-io-protocol-spec.md` | 主 spec 文档 (5 场景 I/O 协议) |
| `internal/layers/orchestration/sessionorchestrator/plan_trace_e2e_test.go` (NEW) | 5 trace test (含混合场景) |

### 不变更

- `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go` — 0 修改
- `internal/layers/orchestration/plan/plan.go` — 0 修改
- `internal/layers/orchestration/plan/planner.go` — 0 修改
- `internal/layers/orchestration/plan/plan_struct.go` — 0 修改
- `internal/layers/contextengine/i18n/prompt_dynamic.go` — 0 修改
- `openspec/specs/d7-orchestration/spec.md` — 0 修改(spec doc 在本 change 内, 不合并到主 spec)

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| spec 与实现漂移 | High | trace test stdout 是 spec 的"活验证", 任何 field/scenario 不一致都被 test 暴露 |
| `StrategicPlanFrame` 字段增减 | Medium | Test #1 (FrameStructure_18Fields) 锁死 18 字段, 改 schema 必然 break test |
| `rawStrategicPlan` JSON schema 变更 | Medium | Test #2 (JSONSchema_ExecutionModeEnum) 锁死 3 选 1 enum |
| `MatchKind` 4 Rules 调整 | Medium | Test #3-4 (MatchKind_All4Kinds) 锁死 routing |
| fast-path 闸门调整 | Medium | Test #5 (FastPathBypass) 锁死 `applySingleModeUncertaintyGate` 行为 |

## 7. 关联

### 父 Change
- `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003) — 同模板沉淀的兄弟 spec

### 关联 Change
- `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, S7_Archived 2026-06-23) — `plan.Plan` 4 PlanKind 源头
- `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived 2026-06-23) — 4 Channel router
- `devrix-d7-observational-fastpath` (DM-20260706-011, S7_Archived 2026-07-07) — Observe fast-path
- DM-20260706-009 — Plan single-mode fast-path bypass
- DM-20260704-006 — RC-1 resolution_strategies[] contract
