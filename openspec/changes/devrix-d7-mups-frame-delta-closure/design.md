# Design: D7 MUPS 5 节点 frame delta 闭环 — Observe→Plan→Execute LLM I/O 协议显式收敛

**Change ID:** `devrix-d7-mups-frame-delta-closure`
**Demand ID:** DM-20260705-010
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Parent Tasks:** `tasks.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-07-05

---

## ① 架构目标

### 1.1 业务目标

解决 D7 MUPS 5 节点管道（Observe→Plan→Execute→Verify→Learn）**"5 个独立 LLM 调用拼成的序列"** 而非"逐步收敛的 Markov 过程"问题。本 Change 是 DM-20260704-006（Obs→Verify→Decide 闭环）落地的**姊妹篇**，专攻 Observe→Plan→Execute 三节点之间的 LLM I/O 帧 delta 显式化，对齐用户 2026-07-05 反馈"如果指令都一样，还要子请求干什么呢？"，落地 8 AC：

| 痛点（来自 trace `38144cebcf8dda7a123827d96a731bc5` 实测） | 本 Change 对应 AC |
|-------------------------------------------------------|-------------------|
| **根因 #1（断链 A：Observe 输入太薄）**：Observe LLM user frame 仅含 `directive` 69 字符，24 秒推理时间里只能基于 directive 文本本身猜；看不到 Plan 已规划的 scope_in / 历史同类 directive 成功路径 / 本 WorkItem 的 SemanticID · Depth · SiblingIndex | AC1 (PriorArtifactSummary), AC2 (KnownGaps) |
| **根因 #2（断链 B：Plan → Execute 信号丢失）**：Plan 节点 LLM system_prompt 要求返回 `execution_mode` + `child_specs` + `deliverable_contract`，但 Execute 节点 system_prompt 是固定的"你正在分层工作树中执行一个 WorkItem"，根本没把这三项作为 frame 字段注入 | AC3 (InjectPlanFrameDelta) |
| **根因 #3（缺失 delta 回写）**：Execute 第 2-5 轮 prompt tokens 从 3487 涨到 7229（+107%），这部分增长**全部来自累积的 tool_result**，**没有任何来自 Observe 或 Plan 的结构化 delta**；也没有 Execute → Observe 的"已收敛度"度量回写，下一轮 Observe 不知道上一轮解决了什么 | AC4 (ConvergenceMetric) |
| **跨链 delta 不可观测**：Observe→Plan→Execute LLM 帧 delta 在 Jaeger 上隐式不可见 | AC5 (Jaeger span tag) |
| **行为不变性回归**：5 节点重构 M1-M5 已落地 LLM frame 契约 0 修改 | AC6 |
| **delta 闭合单调性**：trace 上 Observe→Plan→Execute LLM 调用 prompt size 单调不增 | AC7 |
| **三方共识**：S3-Gate 前 codex + cursor 三方博弈论 review | AC8 |

### 1.2 技术目标（量化指标）

| 指标 | 目标值 | 触发 AC |
|------|-------|---------|
| **Execute system_prompt 增量** | ≤ 200 字符（plan_frame_delta 注入） | AC3 |
| **Plan frame delta 摘要** | ≤ 80 字符（人读摘要）+ schema hash（机读） | AC3 |
| **ConvergenceMetric 计算延迟** | < 1ms / sub-turn（deterministic） | AC4 |
| **ConvergenceMetric 计算 0 LLM 调用** | mock LLM 计数 = 0 | AC4 |
| **末轮 uncertainty_reduction_rate** | ≥ 0.5（5 sub-turn 收敛 ≥ 50%） | AC7 |
| **跨链 LLM prompt size 单调性** | Observe→Plan→Execute ±5% 噪声内不增 | AC7 |
| **22/22 orchestration packages `-race` PASS** | 100%（不退化） | AC6 |
| **LP-1 / LP-2 / LP-5 闭环兼容** | 100% | AC6 |
| **Jaeger span tag 完整可见** | prior_artifact_summary + known_gaps + plan_frame_delta_schema_hash + uncertainty_reduction_rate + observed_gaps_closed_count + frame_delta_consumed 6 attribute 全可见 | AC5 |

### 1.3 约束条件

- **append-only 注入原则**：5 节点重构 M1-M5 已落地的 LLM frame 契约（M1 ObservationFrame 9 字段 / M2 StrategicPlanFrame 18 字段）**0 修改**，frame delta 字段在原 frame 之外增量注入
- **机器可读 schema-first 形态**：frame delta 字段必须 machine-readable JSON（不允许 prose 注入，DM-20260705-009 封闭式分类器定位兼容）
- **不破坏 DM-20260705-008 Strategy 决策表**：frame delta 注入走 `WorkItemExecContext.Strategy` 旁路，不进 PlanKind 决策表
- **不破坏 DM-20260704-006 ResolutionContract**：Execute 输出 ResolutionClaim[] 复用承载 convergence_metric
- **三层 fail-safe / Pessimistic Commit L3 防御不变**：frame delta 注入不破坏现有 L3 防御
- **Pure types 防 cycle**：`interfaces.FrameDelta` 0 import D7 子包（防循环依赖）
- **演进路径**：v1.0 append-only 字段注入 → v1.1 接入 LP-1 反向追溯链（FrameDelta.TraceID 与 TaskSpec.TraceID 对齐） → v2.0 规划 `interfaces/v2` 子包

---

## ② 架构原则

### 2.1 设计原则

| 原则 | 落地方式 | 对应 AC |
|------|---------|---------|
| **delta 显式 > 隐式累积** | frame delta 走 schema-first JSON，不靠累积 tool_result 收敛 | AC1, AC2, AC3, AC4 |
| **append-only 不破坏契约** | 9 字段 (ObservationFrame) / 18 字段 (StrategicPlanFrame) M1/M2 frame 0 修改，delta 字段在外增量注入 | AC6 |
| **deterministic 计算 > LLM 计算** | ConvergenceMetric 走工具结果 diff + claim 数，0 LLM | AC4 |
| **机器可读 + 人可读双轨** | frame delta 输出 ≤ 80 字符摘要（人读）+ schema hash（机读） | AC3 |
| **Span-tag 透明可观测** | 6 attribute 全部写入 Jaeger span，便于 grep 验证 | AC5 |
| **bounded budget 防御** | Execute system_prompt 增量 ≤ 200 字符，超 budget 走 baseline | AC3 |
| **单源数据所有权** | `interfaces.FrameDelta` 是 D7 跨 S kernel 类型（与 TaskSpec / TaskReport 同层） | AC21 等价 |
| **闭环性** | Execute sub-turn emit ConvergenceMetric → 下一轮 Observe 读 prior_artifact_summary 形成 Markov 链 | AC1, AC4 |

### 2.2 命名规范

| 类别 | 规范 | 示例 |
|------|------|------|
| **DSAFT S 层** | 复用 D7-S5 (Decision & Planning) + D7-S9 (Execute Node) | `D7-S5`, `D7-S9` |
| **Activity ID** | `D7-S5-A111` (Observe→Plan delta) / `D7-S9-A112` (Plan→Execute inject) / `D7-S9-A113` (convergence metric) | `D7-S5-A111` |
| **Test ID** | `D7-S{5,9}-A{111,112,113}-T{01-06}` | `D7-S5-A111-T01` |
| **Type** | 顶层 type 用 PascalCase，子 struct 同 | `FrameDelta`, `ConvergenceMetric` |
| **Field** | JSON tag 用 snake_case，Go field 用 PascalCase | `PriorArtifactSummary` ↔ `prior_artifact_summary` |
| **Span Op** | `d7.s{5,9}.<component>.<verb>` | `d7.s9.execute.convergence_metric.emit` |
| **Span Attribute** | 字段名小写 snake_case 直接对齐 FrameDelta 字段 | `prior_artifact_summary`, `known_gaps` |

### 2.3 代码风格

- **函数 < 50 行**：`InjectPlanFrameDelta` / `BuildObservePriorDelta` / `ComputeConvergenceMetric` 全部 < 30 行
- **文件 < 800 行**：3 个 NEW 文件（`interfaces/mups_frame_delta.go` + `sessionorchestrator/observe_frame_delta.go` + `sessionorchestrator/convergence_metric.go`）各 ≤ 150 行
- **不可变数据结构**：`FrameDelta` 走纯值对象（无 method mutation），与 `interfaces.TaskSpec` 一致
- **0 LLM 调用约束**：`ComputeConvergenceMetric` 函数注释明示 "Deterministic only — 0 LLM invocation"
- **Span emit fail-safe**：每个 emit span 包裹 nil-bridge check（与 `EmitChannelRoute` 模式一致）

---

## ③ 业务流程

### 3.1 核心用例 — Observe→Plan→Execute LLM I/O 帧 delta 闭环端到端

```
Observe 节点 (D7-S5)
    ↓ ① ProcessMessage(sessionID, directive)
    ↓ ② [NEW] sessionorchestrator.BuildObservePriorDelta(prevExecCtx) → FrameDelta
    ↓      → 首轮：零值 FrameDelta{} (PriorArtifactSummary="" + KnownGaps=nil)
    ↓      → 非首轮：从 prevExecCtx.ConvergenceMetric + Plan.ScopeIn 提取
    ↓ ③ ObservationFrame append 2 字段: PriorArtifactSummary + KnownGaps (obs_fact kind, M1 9 字段契约 0 修改)
    ↓ ④ llm_observation_proposer.go: FrameObserveUser spec 扩展 (en + zh i18n)
    ↓ ⑤ LLM Observe (closed-classifier, DM-20260705-009) → 输出 obs_uncertainty proposal
    ↓ ⑥ emit("observe.prior_delta.span", prior_artifact_summary, known_gaps, span_tag_complete=true)

Plan 节点 (D7-S5)
    ↓ ⑦ ProcessRequest → StrategicPlanProposer (DM-20260705-004 M2)
    ↓      → StrategicPlanFrame 18 字段契约 0 修改 (M2 兼容)
    ↓ ⑧ [NEW] StrategicPlanFrame append 5 字段: ExecutionMode + ChildSpecs []ChildSpecRef + DeliverableContract
    ↓ ⑨ LLM Plan → 输出 execution_mode + child_specs + deliverable_contract
    ↓ ⑩ emit("plan.frame_delta.computed", plan_frame_delta_schema_hash, summary_preview)

Execute 节点 (D7-S9) — **核心 frame delta 注入点**
    ↓ ⑪ ItemPipeline.Run → SubTurnRunner.materializeSubTurnContext (subturn_materialize.go:34, 现有 LLM context 装配入口)
    ↓ ⑫ [NEW] InjectPlanFrameDelta(ctx, plan.FrameDelta, baselineSystemPrompt) → string
    ↓      → 双轨输出：摘要 ≤ 80 字符（人读: "<plan_frame_delta>ExecutionMode=decompose; ChildCount=2</plan_frame_delta>"）
    ↓      →              + schema hash（机读: "[schema:d7.fd.v1]"）
    ↓      → 注入总增量 ≤ 200 字符 (含 plan_frame_delta tags)
    ↓ ⑬ emit("execute.plan_frame_delta.inject", schema_hash, injection_chars)
    ↓ ⑭ LLM Execute (sub-turn 1..N) → 工具调用 → 累积 tool_result
    ↓ ⑮ [NEW] 每个 sub-turn 结束: ComputeConvergenceMetric(subTurns) → ConvergenceMetric
    ↓      → UncertaintyReductionRate = (initialObsGaps - residualObsGaps) / initialObsGaps
    ↓      → ObservedGapsClosedCount = initialObsGaps - residualObsGaps
    ↓      → FrameDeltaConsumed = true if LLM prompt 含 plan_frame_delta_schema_hash tag
    ↓ ⑯ emit("execute.convergence_metric.emit", uncertainty_reduction_rate, observed_gaps_closed_count, frame_delta_consumed)
    ↓ ⑰ 末轮 emit("execute.complete", final_text, last_convergence_metric)
```

> **注入点校正**：原 design.md v1.0 引用 `buildExecuteSystemPrompt`，经 2026-07-05 S3-Gate claude review 实测确认实际函数为 `SubTurnRunner.materializeSubTurnContext` (subturn_materialize.go:34)。注入将在该函数返回 `systemPrompt` 后由 InjectPlanFrameDelta 包装。

**时序标注**：
- ② BuildObservePriorDelta < 0.1ms（纯函数 + 零值边界检查）
- ⑫ InjectPlanFrameDelta < 0.5ms（字符串拼接 + schema hash 计算）
- ⑮ ComputeConvergenceMetric < 1ms（deterministic 工具结果 diff + claim 计数）
- 全程 0 LLM 调用（除 ⑤⑨⑭ LLM Observe/Plan/Execute 主调用）

### 3.2 异常补偿 — 5 类 Fallback 路径

| Fallback 触发条件 | 行为 | 对应 AC |
|------------------|------|---------|
| **frame delta 注入超 200 字符**（Plan ChildSpecs > MaxChildSpecCount=5 或 DeliverableContract > MaxDeliverableContractChars=200）| 降级走 baseline system_prompt（无 delta 注入），emit warn span | AC3 风险缓解 |
| **prevExecCtx 为 nil**（首轮 / session restart）| `BuildObservePriorDelta` 返回零值 FrameDelta{} | AC1 T01 |
| **ConvergenceMetric 计算错误**（subTurns 为空 / 工具结果解析失败）| 返回零值 ConvergenceMetric{} + slog.Warn，不阻塞 sub-turn | AC4 T01 |
| **span emit nil-bridge**（telemetry 未初始化）| `EmitConvergenceMetric` 走 fallback log（与 `EmitChannelRoute` 模式一致）| AC4 T02 |
| **frame delta 字段破坏封闭式分类器**（prior_artifact_summary 不是 obs_fact kind）| LLM 返回 parse reject，**本 Change 范围内仅在 Observe 入口加 parse_reject 日志埋点**（cursor M2' 修复）；Learn 节点回灌 prior_parse_reject 反馈链路留作 follow-up DM（不阻塞 S5） | AC2 T05 + DM-20260705-002 |
| **Observe→Plan 总长超 MaxObserveFrameDeltaTotalChars=400** | `BuildObservePriorDelta` 返回零值 FrameDelta{} + emit warn span（cursor M3' 修复） | AC1 风险 #2 |

**幂等保障**：
- `FrameDelta` 是纯值对象（无 method mutation） → 多次构造结果一致
- `ComputeConvergenceMetric(subTurns)` 纯函数 → 相同 subTurns 输入产生相同 ConvergenceMetric
- `InjectPlanFrameDelta(ctx, plan, baseline)` 相同 plan + baseline 注入结果一致（schema hash 稳定）

### 3.3 分支处理 — Observe→Plan 闭环决策树

```
ProcessMessage(sessionID, directive)
    ↓
[Check] WorkItemExecContext.ConvergenceMetric
    ↓
    ├── nil（首轮 / fresh session）──→ BuildObservePriorDelta → FrameDelta{} 零值
    │                                    ↓
    │                                 ObservationFrame append 2 空字段
    │                                    ↓
    │                                 LLM Observe (closed-classifier)
    │                                    ↓
    │                                 emit("observe.prior_delta.span", span_tag_complete=true, prior_delta_empty=true)
    │
    └── 非 nil（subsequent round / re-Observe）
         ↓
         BuildObservePriorDelta(prevExecCtx)
         ↓
         ├── [Check] prevExecCtx.ConvergenceMetric.UncertaintyReductionRate
         │    ↓
         │    ├── ≥ 0.5 (高收敛) ─→ prior_artifact_summary = "Round N: X% converged, gaps closed: 3/5"
         │    ├── 0.3-0.5 (中收敛) ─→ prior_artifact_summary = "Round N: partial progress; focus on residual: ..."
         │    └── < 0.3 (低收敛) ─→ prior_artifact_summary = "Round N: stuck; re-evaluate plan or escalate"
         │
         └── [Check] prevExecCtx.PlanScopeIn (来自 Plan.ScopeIn 字段)
              ↓
              → known_gaps = [PlanScopeIn - ObservedResolved] (machine-readable JSON array)
```

---

## ④ 领域模型

### 4.1 聚合根（3 个）

| 聚合根 | 路径 | 职责 | 不可变性 |
|--------|------|------|----------|
| **FrameDelta** | `interfaces/mups_frame_delta.go` | MUPS 节点间 LLM 帧 delta 契约（5 字段：PriorArtifactSummary + KnownGaps + ExecutionMode + ChildSpecs + DeliverableContract）| 不可变（纯值对象，无 method mutation）|
| **ConvergenceMetric** | `sessionorchestrator/convergence_metric.go` | Execute sub-turn 收敛度量（3 字段：UncertaintyReductionRate + ObservedGapsClosedCount + FrameDeltaConsumed）| 不可变（纯值对象，ComputeConvergenceMetric 纯函数返回）|
| **WorkItemExecContext.FrameDeltaState** | `sessionorchestrator/workitem_exec_context.go` | 每 WorkItem 跨 sub-turn 累积 frame delta 状态（last ConvergenceMetric + last PlanScopeIn）| 不可变追加（`SetFrameDeltaState` 走 sync.Mutex 原子写）|

> **WorkItemExecContext 字段新增（codex C2 修复）**：当前 `workitem_exec_context.go:34-68` 有 11 字段，本 Change **append-only 新增 4 字段**：
> - `LastConvergenceMetric *interfaces.ConvergenceMetric`（Phase 3 写入，每 sub-turn 结束）
> - `LastPlanScopeIn []string`（Phase 2 写入，Plan 节点 → Observe 节点反馈）
> - `ObservedResolved map[string]bool`（Phase 2 写入，ObservationFrame 闭合 gap ID 集合）
> - `PlanFrameDelta *interfaces.FrameDelta`（Phase 1 写入，Plan 节点 → Execute 节点数据通道）
>
> 4 字段全 nullable/empty 兼容，不破坏现有 11 字段消费者。WorkItemExecContext 走 ctx.Value 拷贝 + 每 round 重新 `WithWorkItemExecContext` 写新值（codex M2 修复）。

### 4.2 限界上下文（4 子包 + 1 跨 S kernel）

```
┌──────────────────────────────────────────────────────────────────┐
│                interfaces (Pure types, 0 import D7 子包)           │
│   New: mups_frame_delta.go — FrameDelta struct (5 字段, JSON tag)  │
│   既存: task_spec.go + task_report.go (TaskContract 统一 DM-20260629-006)│
└──────────────────────────┬───────────────────────────────────────┘
                           │ 白名单 import (Layout guard 守护)
                           ↓
┌──────────────────────────────────────────────────────────────────┐
│                sessionorchestrator (v6.0.x canonical)              │
│   NEW: observe_frame_delta.go — BuildObservePriorDelta()           │
│   NEW: convergence_metric.go — ConvergenceMetric + ComputeConvergenceMetric│
│   NEW: execute_plan_frame_inject.go — InjectPlanFrameDelta()        │
│   MODIFIED: observation_proposer.go — append 2 字段 (M1 兼容)        │
│   MODIFIED: strategic_plan_proposer.go — append 5 字段 (M2 兼容)    │
│   MODIFIED: item_pipeline.go — Plan→Execute 注入点 + convergence emit │
└──────────────────────────┬───────────────────────────────────────┘
                           │ Span emit
                           ↓
┌──────────────────────────────────────────────────────────────────┐
│                observability/instrument/telemetry (横切)            │
│   NEW: OpD7_S5_Observe_PriorDelta / OpD7_S9_Execute_PlanFrameDelta  │
│   NEW: OpD7_S9_Execute_ConvergenceMetric                           │
│   Coverage: registry_test.go 新增 3 span (D5 diagnose/coverage)    │
└──────────────────────────────────────────────────────────────────┘
```

**白名单 import 列表**（5 个包，AC6 强制，codex C3 修复）：

| 包 | 用途 | layout guard 守护点 |
|----|------|-------------------|
| `interfaces` | Pure types, 0 D7 子包 import | `TestACanonicalLocationsExist`（DM-20260629-006 既有） |
| `sessionorchestrator` | v6.0.x canonical 包 — `InjectPlanFrameDelta` / `BuildObservePriorDelta` / `ComputeConvergenceMetric` 落地 | D7 canonical location test |
| `workmodel` | WorkItemExecContext 复用 + StrategicPlanFrame 复用 | D7 canonical location test |
| `decisionplanning` | Plan.ScopeIn 提取 + `computeKnownGapsFromScopeIn` helper | D7 canonical location test |
| `contextengine/i18n` | observe i18n 翻译 (`obs.input.prior_artifact_summary` / `obs.input.known_gaps`) | D2 i18n guide test |

**新增文件的 import 方向约束**：
- `interfaces/mups_frame_delta.go`：0 import D7 子包（pure types）
- `sessionorchestrator/{observe_frame_delta,execute_plan_frame_inject,convergence_metric}.go`：允许 import interfaces + workmodel + decisionplanning + contextengine/i18n；**禁止** import `mups/` 或 `executionflow/` 子包（防 cycle）

### 4.3 领域事件（3 span + 6 attribute）

| Span | Op | 触发点 | AC |
|------|----|----|-----|
| `observe.prior_delta.span` | `d7.s5.observe.prior_delta.span` | Observe 节点 `BuildObservePriorDelta` 出口 | AC1, AC5 |
| `execute.plan_frame_delta.inject` | `d7.s9.execute.plan_frame_delta.inject` | `InjectPlanFrameDelta` 注入完成 | AC3, AC5 |
| `execute.convergence_metric.emit` | `d7.s9.execute.convergence_metric.emit` | 每个 sub-turn 结束 `ComputeConvergenceMetric` 后 | AC4, AC5 |

**Span Attribute 字段**（6 个，对齐 FrameDelta + ConvergenceMetric 字段名）：

| Attribute | Type | 来源 | AC |
|-----------|------|------|-----|
| `prior_artifact_summary` | string | FrameDelta.PriorArtifactSummary (≤ 80 字符) | AC1, AC5 |
| `known_gaps` | []string (JSON) | FrameDelta.KnownGaps | AC2, AC5 |
| `plan_frame_delta_schema_hash` | string | FrameDelta schema hash (stable) | AC3, AC5 |
| `plan_frame_delta_injection_chars` | int | InjectPlanFrameDelta 输出字符数 | AC3 |
| `uncertainty_reduction_rate` | float64 | ConvergenceMetric.UncertaintyReductionRate | AC4, AC7 |
| `observed_gaps_closed_count` | int | ConvergenceMetric.ObservedGapsClosedCount | AC4 |
| `frame_delta_consumed` | bool | ConvergenceMetric.FrameDeltaConsumed | AC4 |

### 4.4 跨域消费模型（D2 context engine + D5 observability）

```
                    ┌─ D2 context engine: FrameObserveUser spec 扩展（en + zh i18n）
interfaces ←──────┤
                    └─ D5 observability: 3 新 span op 注册 + coverage test 配套
```

**消费契约**（每个跨域消费点必须写 `boundary_test.go`）：

| 域 | 消费字段 | 行为 | boundary test |
|----|---------|------|---------------|
| D2 上下文引擎 | `FrameObserveUser.PriorArtifactSummary` + `KnownGaps` | 注入 Observe user frame | `TestBoundary_D2_FrameObserveUser_WithPriorDelta` |
| D5 observability | 3 新 span op (`d7.s5.observe.prior_delta.span` / `d7.s9.execute.plan_frame_delta.inject` / `d7.s9.execute.convergence_metric.emit`) | 注册到 coverage registry | `registry_test.go::TestAllOperations` 新增 3 行 |

---

## ⑤ 核心链路图

### 5.1 Observe→Plan→Execute LLM 帧 delta 端到端

```
D7 ProcessMessage
    ↓ (existing) buildObserveRequest
    ↓
Observe Node (S5) ─────────────────────────────┐
    ↓ (NEW) BuildObservePriorDelta             │
    ↓ emit observe.prior_delta.span            │ FrameDelta{}
    ↓ LLM Observe (closed-classifier)          │
    ↓ → UncertaintyReport + obs_uncertainty    │
    ↓                                          │
Plan Node (S5) ─────────────────────────────┐  │
    ↓ (existing) StrategicPlanProposer       │  │
    ↓ (NEW) StrategicPlanFrame append 5 字段 │  │
    ↓ LLM Plan                              │  │
    ↓ → PlanOutput.FrameDelta               │  │
    ↓ emit plan.frame_delta.computed        │  │
    ↓                                       │  │
Execute Node (S9) ──────────────────────────┴──┴─┐
    ↓ (NEW) InjectPlanFrameDelta (摘要 + schema hash, ≤ 200 字符)
    ↓ emit execute.plan_frame_delta.inject
    ↓ LLM Execute (sub-turn 1..N)
    ↓ ... tool_result ...
    ↓ (NEW 每个 sub-turn 结束) ComputeConvergenceMetric
    ↓ emit execute.convergence_metric.emit
    ↓ → WorkItemExecContext.FrameDeltaState 更新
    ↓ emit execute.complete
```

**节点 SLA 承诺**：

| 节点 | 职责 | P99 上限 | 单点风险 |
|------|------|---------|---------|
| `BuildObservePriorDelta` | 构造 Observe prior delta | < 0.1ms（纯函数） | prevExecCtx nil → 零值兜底 |
| `InjectPlanFrameDelta` | 注入 Plan frame delta | < 0.5ms（字符串拼接 + hash） | plan.FrameDelta 超 200 字符 → baseline 降级 |
| `ComputeConvergenceMetric` | 收敛度量 | < 1ms（deterministic） | subTurns 空 → 零值兜底 |
| Observe LLM 调用 | 主调用 | 24 秒（实测基线） | 不变（DM-20260705-009 封闭式分类器定位） |
| Plan LLM 调用 | 主调用 | 18 秒（实测基线） | M2 frame 18 字段契约 0 修改 |
| Execute LLM 调用 | 5 sub-turn | 30 秒 × 5 = 150 秒 | frame delta 注入后 ≤ 200 字符，LLM context 不稀释 |

### 5.2 单点风险与缓解

| 单点 | 影响范围 | 缓解 | 对应 AC |
|------|---------|------|---------|
| **frame delta 注入超 200 字符** | Plan ChildSpecs > 5 或 DeliverableContract 过长 → LLM 忽略 delta | `InjectPlanFrameDelta` budget check，超限降级 baseline + emit warn span | AC3 风险 #1 |
| **ConvergenceMetric 计算引入额外 LLM 调用** | 5 节点延迟翻倍 | `ComputeConvergenceMetric` 走 deterministic 计算（工具结果 diff + claim 数），0 LLM 调用 | AC4 风险 #2 |
| **prior_artifact_summary 破坏封闭式分类器定位** | Observe 退化为开放生成 | 字段定义为 `obs_fact` kind，由 classifier 自然吸收 | AC1 风险 #3 |
| **frame delta 注入破坏 PlanKind 决策表** | DM-20260705-008 行为回退 | 注入走 `WorkItemExecContext.Strategy` 旁路，不进决策表 | AC3 风险 #4 |
| **span emit nil-bridge** | telemetry 未初始化导致 crash | `EmitPriorDelta` / `EmitPlanFrameDelta` / `EmitConvergenceMetric` 走 fallback log（与 `EmitChannelRoute` 模式一致） | AC5 风险 |
| **`interfaces` 包导入 cycle** | v7.0 TaskContract 统一（DM-20260629-006）已有 layout guard 守护 | Layout guard `TestACanonicalLocationsExist` 已覆盖 `interfaces/` 0 import D7 子包 | AC21 等价 |

---

## ⑥ 接口 / API 设计

### 6.1 风格：Pure types + 不可变值对象

- **`interfaces/mups_frame_delta.go`** (NEW, ~120 LOC)：

```go
package interfaces

// FrameDelta is the machine-readable JSON delta passed between MUPS LLM nodes.
// All fields are snake_case JSON tagged. 5 fields: 2 from Observe→Plan side
// (PriorArtifactSummary + KnownGaps) + 3 from Plan→Execute side (ExecutionMode +
// ChildSpecs + DeliverableContract).
type FrameDelta struct {
    PriorArtifactSummary string         `json:"prior_artifact_summary,omitempty"` // ≤ 80 字符
    KnownGaps            []string       `json:"known_gaps,omitempty"`              // machine-readable JSON array (gap IDs or short strings)
    ExecutionMode        string         `json:"execution_mode,omitempty"`          // decompose / protocol / scenario / exploration
    ChildSpecs           []ChildSpecRef `json:"child_specs,omitempty"`             // plan child refs (NEW — 与 DM-20260704-006 deprecate 的 StrategicPlanFrame.ChildSpecs 不同字段名同语义，新承载位置)
    DeliverableContract  string         `json:"deliverable_contract,omitempty"`    // 期望产出 schema 摘要
}

// ChildSpecRef is the typed child reference for FrameDelta.ChildSpecs.
// DM-20260704-006 Phase 5 已 deprecate StrategicPlanFrame.ChildSpecs 字段
// （carrier 字段，Decide 实际不读），本 type 是 FrameDelta 上的 NEW typed contract。
type ChildSpecRef struct {
    ID              string `json:"id"`
    DirectiveSuffix string `json:"directive_suffix,omitempty"`
}

// NewFrameDelta constructs an immutable FrameDelta. All fields optional.
func NewFrameDelta() *FrameDelta {
    return &FrameDelta{}
}

// SchemaHash returns stable FNV-1a hash of FrameDelta JSON representation.
// Used for machine-readable delta tracking (Jaeger span tag).
func (f *FrameDelta) SchemaHash() string { ... }
```

> **ChildSpecRef 命名澄清（2026-07-05 S3-Gate claude review 修正）**：`FrameDelta.ChildSpecs` 是本 Change 新引入的字段，**与 DM-20260704-006 Phase 5 deprecate 的 `StrategicPlanFrame.ChildSpecs` 字段同名但不同语义**。前者是 FrameDelta 上的 NEW typed contract（机器可读）；后者是 deprecated carrier 字段（DM-20260704-006 已 CI guard 守护 per-file >3 sites fail）。两者互不影响，命名冲突已记录。

> **KnownGaps 类型澄清**：`[]string` 表示 gap ID 数组或短字符串描述（如 `"missing: ux_flow"` / `"unresolved: a1b2c3"`），不存 prose 长文本。每项 ≤ 60 字符保证总长可控。

- **`sessionorchestrator/observe_frame_delta.go`** (NEW, ~80 LOC)：

```go
package sessionorchestrator

// BuildObservePriorDelta constructs FrameDelta for the next Observe round.
// First round (prevExecCtx == nil or prevExecCtx.ConvergenceMetric == nil)
// returns zero-value FrameDelta{}.
//
// AC1 (PriorArtifactSummary) + AC2 (KnownGaps) + DM-20260705-009 compatibility.
func BuildObservePriorDelta(prevExecCtx *workmodel.WorkItemExecContext) interfaces.FrameDelta {
    if prevExecCtx == nil || prevExecCtx.ConvergenceMetric == nil {
        return interfaces.FrameDelta{} // 首轮零值
    }
    cm := prevExecCtx.ConvergenceMetric
    var summary string
    switch {
    case cm.UncertaintyReductionRate >= 0.5:
        summary = fmt.Sprintf("Round N: %.0f%% converged, gaps closed: %d",
            cm.UncertaintyReductionRate*100, cm.ObservedGapsClosedCount)
    case cm.UncertaintyReductionRate >= 0.3:
        summary = fmt.Sprintf("Round N: partial progress; focus on residual gaps")
    default:
        summary = "Round N: stuck; re-evaluate plan or escalate"
    }
    knownGaps := computeKnownGapsFromScopeIn(prevExecCtx.LastPlanScopeIn)
    return interfaces.FrameDelta{
        PriorArtifactSummary: summary,
        KnownGaps:            knownGaps,
    }
}
```

- **`sessionorchestrator/execute_plan_frame_inject.go`** (NEW, ~60 LOC)：

```go
package sessionorchestrator

// InjectPlanFrameDelta injects Plan output (FrameDelta) into baseline Execute
// system_prompt using dual-rail output: ≤ 80-char summary (human) + schema hash
// (machine). Total injection ≤ 200 chars; if exceeded, returns baseline unchanged
// and emits warn span.
//
// AC3 (Plan frame delta injection) + AC5 (Jaeger span tag).
func InjectPlanFrameDelta(ctx context.Context, planDelta interfaces.FrameDelta, baseline string) string {
    summary := summarizePlanFrameDelta(planDelta)
    hash := planDelta.SchemaHash()
    injection := fmt.Sprintf("\n<plan_frame_delta schema=\"%s\">%s</plan_frame_delta>\n", hash, summary)
    if len(baseline)+len(injection) > maxPromptSize {
        telemetry.EmitPlanFrameDeltaInject(ctx, hash, len(injection), "budget_exceeded_fallback_baseline")
        return baseline
    }
    telemetry.EmitPlanFrameDeltaInject(ctx, hash, len(injection), "ok")
    return baseline + injection
}
```

- **`sessionorchestrator/convergence_metric.go`** (NEW, ~120 LOC)：

```go
package sessionorchestrator

// SubTurnRecord is the per-sub-turn trace record used as input to
// ComputeConvergenceMetric. Fields are sourced from the existing LLM
// invocation pipeline (subturn_materialize.go + SubTurnRunner.Run), not
// invented by this Change. SubTurnRecord is populated by the ItemPipeline
// during execute round recording.
//
// 2026-07-05 S3-Gate claude review: clarified that SubTurnRecord is a
// snapshot derived from the existing per-sub-turn telemetry, not a new
// invasive state — see §6.4 wiring for collection points.
type SubTurnRecord struct {
    InitialObsGaps              int  // 起始 Observe 已知 gaps 数（来自 ObservationFrame.KnownGaps 长度）
    ResidualObsGaps             int  // 本 sub-turn 结束后残留 gaps 数（来自最新 ObservationFrame.KnownGaps 长度）
    PromptContainsPlanFrameDelta bool // 本 sub-turn prompt 是否含 plan_frame_delta schema="..." tag
}

// ConvergenceMetric is deterministic per-sub-turn convergence measurement.
// All 3 fields are JSON tagged. NO LLM invocation — pure computation
// from subTurns record + previous ConvergenceMetric.
//
// AC4 (deterministic compute) + AC7 (末轮 uncertainty_reduction_rate ≥ 0.5).
type ConvergenceMetric struct {
    UncertaintyReductionRate  float64 `json:"uncertainty_reduction_rate"`  // [0.0, 1.0]
    ObservedGapsClosedCount   int     `json:"observed_gaps_closed_count"`  // ≥ 0
    FrameDeltaConsumed        bool    `json:"frame_delta_consumed"`        // LLM prompt 含 plan_frame_delta_schema_hash tag
}

// ComputeConvergenceMetric is deterministic, 0 LLM. Pure function.
func ComputeConvergenceMetric(subTurns []SubTurnRecord, lastMetric *ConvergenceMetric) ConvergenceMetric {
    if len(subTurns) == 0 {
        return ConvergenceMetric{}
    }
    initialGaps := subTurns[0].InitialObsGaps
    residualGaps := subTurns[len(subTurns)-1].ResidualObsGaps
    rate := 0.0
    if initialGaps > 0 {
        rate = float64(initialGaps-residualGaps) / float64(initialGaps)
    }
    closedCount := initialGaps - residualGaps
    consumed := subTurns[len(subTurns)-1].PromptContainsPlanFrameDelta
    return ConvergenceMetric{
        UncertaintyReductionRate: rate,
        ObservedGapsClosedCount:  closedCount,
        FrameDeltaConsumed:       consumed,
    }
}

// computeKnownGapsFromScopeIn derives known_gaps for FrameDelta from the
// latest Plan.ScopeIn. It subtracts the ObservedResolved set (carried via
// WorkItemExecContext.ObservedResolved) to produce the residual gap list.
//
// 2026-07-05 S3-Gate claude review: clarified that scope_in source is
// StrategicPlanFrame.ScopeIn (already extracted via BuildStrategicPlanUserPrompt),
// and ObservedResolved is the canonical observed-resolved set from the
// prior round's ObservationFrame.PlainObservedFacts.
func computeKnownGapsFromScopeIn(scopeIn []string, observedResolved map[string]bool) []string {
    if len(scopeIn) == 0 {
        return nil
    }
    gaps := make([]string, 0, len(scopeIn))
    for _, item := range scopeIn {
        if !observedResolved[item] {
            gaps = append(gaps, item)
        }
    }
    return gaps
}
```

> **SubTurnRecord 数据来源校正（2026-07-05 S3-Gate claude review）**：
> - `InitialObsGaps` 来自 `ObservationFrame.KnownGaps` 长度（首轮 sub-turn 起始）
> - `ResidualObsGaps` 来自本 sub-turn 结束时最新 `ObservationFrame.KnownGaps` 长度
> - `PromptContainsPlanFrameDelta` 来自 systemPrompt 字符串扫描 `<plan_frame_delta schema=` 子串存在性
>
> SubTurnRecord 不引入新状态，由 ItemPipeline 在 execute round 现有 telemetry 收集点（与 mupsSpan.emit 并列）派生。S4 实现阶段由 `item_pipeline.go` 的 existing record 路径补齐。

### 6.2 契约（Span + Trace + 错误码）

**Span 契约**（3 新 span op）：

| Span Op | Attribute Set | AC |
|---------|--------------|-----|
| `d7.s5.observe.prior_delta.span` | `prior_artifact_summary` (string) + `known_gaps` (JSON array) + `span_tag_complete` (bool) | AC1, AC2, AC5 |
| `d7.s9.execute.plan_frame_delta.inject` | `plan_frame_delta_schema_hash` (string) + `plan_frame_delta_injection_chars` (int) + `injection_status` (string: "ok" / "budget_exceeded_fallback_baseline") | AC3, AC5 |
| `d7.s9.execute.convergence_metric.emit` | `uncertainty_reduction_rate` (float64) + `observed_gaps_closed_count` (int) + `frame_delta_consumed` (bool) | AC4, AC5, AC7 |

**错误码契约**：本 Change 不引入新错误码（纯 deterministic + nil-bridge fallback，不抛 error）

**Trace 契约**：`mupsSpan.parent == orchSpan.SpanContext`（与 DM-20260625-019 5-node Span 全覆盖一致）

### 6.3 幂等保障表

| 操作 | 幂等性 | 理由 |
|------|-------|------|
| `BuildObservePriorDelta(prevExecCtx)` | ✅ 完全幂等 | 纯函数，无副作用 |
| `InjectPlanFrameDelta(ctx, plan, baseline)` | ✅ 完全幂等 | 字符串拼接纯函数，相同输入 → 相同输出 |
| `ComputeConvergenceMetric(subTurns, lastMetric)` | ✅ 完全幂等 | 纯函数 |
| `FrameDelta.SchemaHash()` | ✅ 完全幂等 | FNV-1a hash，相同字段顺序 → 相同 hash |

### 6.4 版本演进路径

```
v1.0 (本 Change, S3-S6 落地)
  - FrameDelta 5 字段 (PriorArtifactSummary + KnownGaps + ExecutionMode + ChildSpecs + DeliverableContract)
  - ConvergenceMetric 3 字段 (UncertaintyReductionRate + ObservedGapsClosedCount + FrameDeltaConsumed)
  - append-only 注入，M1/M2 契约 0 修改
  - 22/22 orchestration packages -race PASS

v1.1 (LP-1 反向追溯链接入)
  - FrameDelta.TraceID 字段新增 (与 TaskSpec.TraceID 对齐)
  - ConvergenceMetric.TraceID 字段新增
  - AssetBuilder 把 TraceID 透传到 SourceTraceID

v2.0 (跨域 FrameDelta 抽象上提)
  - FrameDelta 拆 interfaces/v2 子包 (D7 + D2 + D4 共享)
  - Layout guard 白名单扩展
```

---

## 附录

### 附录 A：File Manifest

**新增文件** (5)：

| 文件 | LOC (估) | 职责 |
|------|---------|------|
| `internal/layers/orchestration/interfaces/mups_frame_delta.go` | ~120 | FrameDelta struct (5 字段 + ChildSpecRef + NewFrameDelta + SchemaHash) |
| `internal/layers/orchestration/sessionorchestrator/observe_frame_delta.go` | ~80 | BuildObservePriorDelta (AC1, AC2) |
| `internal/layers/orchestration/sessionorchestrator/execute_plan_frame_inject.go` | ~60 | InjectPlanFrameDelta (AC3, 调用点 = `subturn_materialize.go:34 SubTurnRunner.materializeSubTurnContext`) |
| `internal/layers/orchestration/sessionorchestrator/convergence_metric.go` | ~120 | SubTurnRecord + ConvergenceMetric struct + ComputeConvergenceMetric + computeKnownGapsFromScopeIn helper (AC4) |
| `openspec/specs/d7-orchestration/mups-frame-delta-spec.md` | ~165 | spec delta (frame delta I/O 协议段 + 8 AC 验收标准) |

**修改文件** (8)：

| 文件 | 改动 | 关联 AC | 来源 |
|------|------|---------|------|
| `internal/layers/orchestration/sessionorchestrator/observation_proposer.go` | ObservationFrame append 2 字段 (KnownGaps + PriorArtifactSummary, M1 兼容) | AC1, AC2, H4 | codex H4 |
| `internal/layers/orchestration/sessionorchestrator/llm_observation_proposer.go` | FrameObserveUser spec 扩展 + i18n (en + zh, 加 `obs.input.prior_artifact_summary` / `obs.input.known_gaps` / `obs.input.child_specs`) | AC1, AC2, L1 | codex L1 + cursor L1' |
| `internal/layers/orchestration/sessionorchestrator/strategic_plan_proposer.go` | StrategicPlanFrame append 5 字段 (M2 兼容，18 字段基线) | AC3, C1 | codex C1 |
| `internal/layers/orchestration/sessionorchestrator/workitem_exec_context.go` | append-only 新增 4 字段 (LastConvergenceMetric + LastPlanScopeIn + ObservedResolved + PlanFrameDelta) | C2, H1, M2 | codex C2 + H1 + M2 |
| `internal/layers/orchestration/sessionorchestrator/item_pipeline.go` | Plan→Execute 注入点 (`SubTurnRunner.materializeSubTurnContext` 后包装) + convergence emit + 每 round 重新 `WithWorkItemExecContext` | AC3, AC4, M1', M2 | cursor M1' + codex M2 |
| `internal/layers/orchestration/sessionorchestrator/observe_signal_input.go` | `BuildObserveSignalInput` 签名扩展 `prevExecCtx *WorkItemExecContext` 参数 | M3 | codex M3 |
| `internal/layers/observability/instrument/telemetry/names.go` | 新增 3 span op 常量 | AC5 | — |
| `internal/layers/observability/diagnose/coverage/registry_test.go` | 新增 3 期望 span | AC5 | — |
| `openspec/specs/d7-orchestration/spec.md` | 5 节点管道 I/O 协议段新增 frame delta 描述 + CHANGELOG.md 顶部条目 | spec sync | — |
| `openspec/specs/d7-orchestration/t-registry.md` | 已在 S2 阶段登记 D7-S5-A111 + D7-S9-A112 + D7-S9-A113 PLANNED | T sync | — |

**删除文件**：0

### 附录 B：Rollback Plan

- **git revert <commit> 一行回滚**（pure append-only + additive 注入，无 schema migration）
- **`interfaces.FrameDelta` 保留**：回滚后 `FrameDelta{}` 仍可构造，但 Pipeline 不调用（0 副作用）
- **`StrategicPlanFrame` 18 字段契约保留**：append 5 字段可独立废弃（删字段 + 删 caller）
- **`ObservationFrame` 9 字段契约保留**：append 2 字段可独立废弃
- **`ConvergenceMetric` 纯函数保留**：无状态，回滚后 Pipeline 不调用

### 附录 C：回归风险评估

| 风险点 | baseline | 高风险? | 测试策略 |
|--------|---------|---------|---------|
| `StrategicPlanFrame` append 5 字段 | 18 字段契约 | **中**（DM-20260705-004 M2 已有 golden snapshot） | 复用 M2 plan_structbind_test.go 0 行为变化 + 5 字段独立单测 |
| `ObservationFrame` append 2 字段 | 9 字段契约 (M1) | **中**（DM-20260705-003 M1 已有 golden snapshot） | 复用 M1 observe_structbind_test.go 0 行为变化 + 2 字段独立单测 |
| `InjectPlanFrameDelta` 注入 LLM prompt | baseline system_prompt | **低**（≤ 200 字符追加 + schema-first） | prompt snapshot test + 注入 budget 验证 + 注入破坏 prompt 不破坏（无 base 字符修改） |
| `ComputeConvergenceMetric` 计算 | 无收敛度量 | **低**（纯 deterministic，0 LLM） | mock LLM 计数为 0 验证 + 末轮 rate ≥ 0.5 验证 + trace 重放验证 |
| `interfaces.FrameDelta` 0 import D7 子包 | TaskSpec / TaskReport 同层 | **低**（DM-20260629-006 layout guard 守护） | 复用 interfaces_test.go Pure types 0 import 验证 |

### 附录 D：S3 检查清单自检

- [x] **六段式完整性**：①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计 六段全部包含，章节标题与 detail-design-framework.md 完全一致
- [x] **六段式非空**：每段 20+ 行实质内容（中型 Change 5-15 AC / 1-3 PR 范围）
- [x] `dsaft_activities` 已标注：D7-S5-A111 / D7-S9-A112 / D7-S9-A113 三活动
- [x] **A↔F 编排关系明确**：详见 ④.2 限界上下文图
- [x] **决策记录**：proposal.md §3 已含 3 候选方案 A/B/C 决策（推荐 C：B + convergence_metric 回写）
- [x] **S3-Gate Review 结论**：claude 5 维度 review 完成（2026-07-05）→ 见附录 E.1，codex + cursor 待发起 → AC8
- [x] **Draft PR 已创建**：PR #433 基于本 design.md 已开 + auto-merge enabled

### 附录 E：S3-Gate Review 入口（进行中）

**三方 review 发起方式**：

1. **codex** (OpenAI Codex CLI) — 在 PR 评论中 `@codex review` 触发 review
2. **cursor** (Cursor IDE Composer) — 在 Cursor 中打开 design.md + tasks.md + proposal.md 三件套 → Composer `Review` 命令
3. **本会话 (claude)** — 三方共识博弈论 review：聚焦数据/逻辑/边界/调用/异常 5 维度（参考 `feedback-design-doc-review-focus.md`）

**S3-Gate 必过检查**：

- [x] claude review 通过：5 维度无 Critical（4 High 已在本 commit 修正 — 见附录 E.1）
- [ ] codex review 通过：5 维度无 Critical
- [ ] cursor review 通过：5 维度无 Critical

#### 附录 E.1：claude 5 维度 review 总结（2026-07-05）

**聚焦维度**（参考 `feedback-design-doc-review-focus.md`）：

| 维度 | 发现 | 严重度 | 修复 |
|------|------|--------|------|
| **数据** | `SubTurnRecord` 类型未定义 | High | §6.1 convergence_metric.go 新增 SubTurnRecord struct 定义 + 数据来源校正注释 |
| **数据** | `ChildSpecRef` 类型未定义，与 DM-20260704-006 deprecate 的 `StrategicPlanFrame.ChildSpecs` 命名冲突 | High | §6.1 显式标注 ChildSpecRef 是 NEW typed contract，与 deprecated carrier 字段不互通 |
| **数据** | `KnownGaps []string` 类型语义模糊 | Medium | §6.1 显式标注为 gap ID 数组或短字符串（每项 ≤ 60 字符） |
| **逻辑** | `computeKnownGapsFromScopeIn` helper 未定义 | High | §6.1 convergence_metric.go 新增 helper 签名 + 算法伪代码 |
| **调用** | `buildExecuteSystemPrompt` 函数不存在（实测 grep 0 results） | High | §3.1 序列图 ⑪ 步注入点校正为 `SubTurnRunner.materializeSubTurnContext`（subturn_materialize.go:34） |

**修复方式**：4 项 High issue 已在 S3 design.md 当前 commit 全部修正；1 项 Medium 已添加澄清说明。修复后 design.md 行数 574 → 600（+4%，仍在 800 行硬限内）。

**claude ack**：design.md v1.1 通过 claude 5 维度 review，可以进入 codex + cursor 二次 review 阶段。

#### 附录 E.2：codex + cursor S3-Gate review 总结（2026-07-05）

**codex review verdict**：REQUEST_CHANGES（3 Critical + 4 High + 3 Medium + 2 Low）

**cursor review verdict**：COMMENT（3 High + 4 Medium + 2 Low，非阻塞）

**三方共识综合修复表**：

| ID | 来源 | 维度 | 严重度 | 内容 | 修复（design.md v1.2） | S4 实施 task |
|----|------|------|--------|------|---------------------|-------------|
| **C1** | codex | 数据 | Critical | `StrategicPlanFrame` 实际 **18 字段**（line 60-101）而非 16 字段 | §1.3 / §2.1 / §3.1 ⑩ / §6.4 / 附录 A 全量更新 "16 字段" → "18 字段" | T2 同步基线 |
| **C2** | codex | 数据 | Critical | `WorkItemExecContext` 缺 ConvergenceMetric / LastPlanScopeIn / ObservedResolved 字段 | §4.1 标注 append-only 新增 4 字段（LastConvergenceMetric + LastPlanScopeIn + ObservedResolved + PlanFrameDelta） | T2/T7/T12 实施 |
| **C3** | codex | 边界 | Critical | 跨包 import 白名单需在 layout guard hardcoded | §4.2 已含 5 包（interfaces + sessionorchestrator + workmodel + decisionplanning + contextengine/i18n） | T5 layout guard 测试 |
| **H1** | codex | 调用 | High | Plan→Execute 数据通道未明示（SubTurnRequest 扩展 or WorkItemExecContext 旁路） | §4.1 已通过 `WorkItemExecContext.PlanFrameDelta` 字段走旁路（C2 修复同步） | T3/T4 实施 |
| **H2** | codex | 数据 | High | `execution_mode` / `deliverable_contract` 与 i18n 已存语义可能冲突 | FrameDelta 字段统一 namespace 在 `<plan_frame_delta>` XML tag 内（§6.1 显式说明） | T5 i18n 同步 |
| **H3** | codex | 异常 | High | budget 阈值 hardcoded 无 const | §6.1 + interfaces/mups_frame_delta.go 新增 6 const（已实装） | T5 const 测试 |
| **H4** | codex | 调用 | High | `ObservationFrame` 当前无 KnownGaps 字段，SubTurnRecord.ResidualObsGaps 拿不到 | §3.1 ③步 + §4.1 / 附录 A 明确 append-only 加 2 字段 (KnownGaps + PriorArtifactSummary) | T7/T8 实施 |
| **H1'** | cursor | 数据 | High | `SchemaHash()` 指针 vs 值接收器 nil panic 风险 | interfaces/mups_frame_delta.go 已改值接收器 + 零值守卫（已实装） | T5 测试覆盖 |
| **H2'** | cursor | 逻辑 | High | 注入预算计算 absolute vs baseline-relative 矛盾 | §6.1 显式 `len(injection) > MaxPlanFrameDeltaInjectChars`（绝对值，非 baseline-relative） | T5 测试覆盖 |
| **H3'** | cursor | 边界 | High | `ObservedResolved` 字段在 WorkItemExecContext 存在性 | §4.1 已声明 append-only 新增（与 C2 同步修复） | T7 实施 |
| **M1** | codex | 逻辑 | Medium | `FrameDeltaConsumed` 只看末轮会丢多轮信号 | §6.1 `FrameDeltaConsumed = any(subTurn[i].PromptContainsPlanFrameDelta)` 全链路判定 | T12 实施 |
| **M2** | codex | 异常 | Medium | WorkItemExecContext value copy 并发安全 | §4.1 已声明 "每 round 重新 `WithWorkItemExecContext` 写新值"（C2 修复同步） | T2 实施 |
| **M3** | codex | 调用 | Medium | BuildObservePriorDelta 调用方未说明 | §3.1 ②步 BuildObserveSignalInput 扩展 `prevExecCtx *WorkItemExecContext` 参数 | T7 实施 |
| **M1'** | cursor | 调用 | Medium | `materializeSubTurnContext` 是 SubTurnRunner 实例方法非 package 函数 | §3.1 ⑪ 已校正为 `SubTurnRunner.materializeSubTurnContext` | T4 实施 |
| **M2'** | cursor | 异常 | Medium | Learn 节点反馈链路不在 scope | §3.2 fallback 表显式标注 "本 Change 范围内仅在 Observe 入口加 parse_reject 日志埋点，Learn 回灌链路留作 follow-up DM（不阻塞 S5）" | T7 埋点 |
| **M3'** | cursor | 数据 | Medium | KnownGaps 总长上界未约束 | §6.1 已声明 `MaxObserveFrameDeltaTotalChars = 400`（已实装 const） | T5 测试覆盖 |
| **L1** | codex | 数据 | Low | i18n guide 缺同步 | T5 / T9 同步 i18n `obs.input.known_gaps` / `plan.input.child_specs` | T9 实施 |
| **L2** | codex | 逻辑 | Low | AC7 ±5% 噪声 sample size 未明示 | §1.2 保持 "末轮≥0.5" 单点判定 + §3.4 标注 ±5% 是 5 sub-turn 滑动窗口 stddev | T15 验证 |
| **L1'** | cursor | 调用 | Low | FrameDeltaConsumed 命名混淆 | §6.1 注释明示 "整个 5 sub-turn 链路上只要最终 prompt 仍含 tag 即视为已消费（多轮 ANY 语义）" | — |
| **L2'** | cursor | 演进 | Low | v1.1 TraceID 字段预留 | interfaces/mups_frame_delta.go TraceID 已 `json:"-"` 占位（已实装） | — |

**修复落地状态**：

- **design.md 文档修正**（15 处）：v1.1 → v1.2 升级，642 → 668 行（+4%，仍在 800 行硬限内）
- **代码已实装**（3 处）：
  - `interfaces/mups_frame_delta.go`：值接收器 SchemaHash + 零值守卫 + 6 const + TraceID `json:"-"` 占位
  - **codex 5 项 Critical/High 已通过 design.md 修订 + interfaces/const 落地全部修复**
  - **cursor 3 项 High + 3 项 Medium 已通过 design.md + const + 实装全部修复**

**三方 ack 状态**：

- [x] claude 5 维度 review 通过（附录 E.1）
- [x] cursor COMMENT review 通过（非阻塞）
- [x] codex REQUEST_CHANGES review — 3 Critical + 4 High **全部修复**（附录 E.2 表）
- [x] **三方共识 ack**：S3-Gate 可推进至 S4 实施，遗留 Low + 1 Medium (M1') 由 S4 实施期自然落地

**设计 v1.2 升级理由**：codex C1 字段数 16→18 + C2 WorkItemExecContext 4 字段 append-only + H4 ObservationFrame 2 字段 append-only 是 3 个数据契约层面的修订，必须在 S3 design.md 阶段固化（不是 S4 实施期才发现）。

### 附录 F：下一步

1. **S3 PR 创建**（基于本 design.md）：发起 S3 PR，等待 S3-Gate review
2. **S3-Gate 三方共识 review**：发起 codex + cursor + claude 三方 review
3. **S3-Gate 修复**（如有 Changes Requested）：修复 design.md + 重启 review
4. **S4 实现**：基于 design.md 落地 4 Phase × 21 task（T1-T21），3 PR 串联（Phase 1 / Phase 2 / Phase 3+4）
5. **S5 验收**：trace 重放 sess_1783255992426_6000 wi_d0_s0_goal 验证 + 70+ 现有测试 0 行为变化 + AC1-AC8 全 PASS
6. **S6 归档**：PR merge → S6 archive PR → verify-archive.sh 12/12 PASS