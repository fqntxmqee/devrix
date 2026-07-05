# D7 MUPS 5 节点 Frame Delta I/O 协议规范

**Change ID:** `devrix-d7-mups-frame-delta-closure`
**Demand ID:** DM-20260705-010
**Status:** S4_Implemented (Phase 1-3 code landed via PR #434; Phase 4 端到端 trace 重放 + 三方 review 待 S5 验收)
**Parent Proposal:** [proposal.md](../../changes/devrix-d7-mups-frame-delta-closure/proposal.md)
**Parent Design:** [design.md](../../changes/devrix-d7-mups-frame-delta-closure/design.md)（六段式 detail-design-framework）
**Created:** 2026-07-05

---

## 1. 范围

本 spec delta 仅定义 MUPS 5 节点管道中 **Observe→Plan→Execute** 三节点之间的 LLM I/O 帧 delta 显式协议。Verify / Learn 节点是 deterministic（0 LLM），不在本 Change 范围。

**不变性承诺**：5 节点重构 M1-M5 已落地的 LLM frame 契约（M1 ObservationFrame 9 字段 / M2 StrategicPlanFrame 16 字段）**0 修改**，frame delta 字段在原 frame 之外增量注入。

## 2. Frame Delta 字段契约

### 2.1 FrameDelta（5 字段）

```go
type FrameDelta struct {
    PriorArtifactSummary string         `json:"prior_artifact_summary,omitempty"`
    KnownGaps            []string       `json:"known_gaps,omitempty"`
    ExecutionMode        string         `json:"execution_mode,omitempty"`
    ChildSpecs           []ChildSpecRef `json:"child_specs,omitempty"`
    DeliverableContract  string         `json:"deliverable_contract,omitempty"`
}

type ChildSpecRef struct {
    ID              string `json:"id"`
    DirectiveSuffix string `json:"directive_suffix,omitempty"`
}
```

| 字段 | 方向 | 类型 | 长度约束 | 语义 |
|------|------|------|---------|------|
| `prior_artifact_summary` | Observe→Plan | string | ≤ 80 字符 | 上一轮 Execute 收敛度的人读摘要 |
| `known_gaps` | Observe→Plan | []string | 0-N 项 | Plan.ScopeIn 与已 ObservedResolved 的差集（machine-readable JSON array） |
| `execution_mode` | Plan→Execute | string | enum | decompose / protocol / scenario / exploration |
| `child_specs` | Plan→Execute | []ChildSpecRef | 0-5 项 | Plan 决策的子节点 ref（DM-20260704-006 已 deprecate 旧 child_specs[]） |
| `deliverable_contract` | Plan→Execute | string | ≤ 200 字符 | 期望产出 schema 摘要 |

### 2.2 ConvergenceMetric（3 字段）

```go
type ConvergenceMetric struct {
    UncertaintyReductionRate  float64 `json:"uncertainty_reduction_rate"`  // [0.0, 1.0]
    ObservedGapsClosedCount   int     `json:"observed_gaps_closed_count"`  // ≥ 0
    FrameDeltaConsumed        bool    `json:"frame_delta_consumed"`        // LLM prompt 含 plan_frame_delta_schema_hash tag
}
```

| 字段 | 计算方式 | 取值范围 | AC |
|------|---------|---------|-----|
| `uncertainty_reduction_rate` | (initialObsGaps - residualObsGaps) / initialObsGaps | [0.0, 1.0] | AC4 + AC7（末轮 ≥ 0.5）|
| `observed_gaps_closed_count` | initialObsGaps - residualObsGaps | ≥ 0 | AC4 |
| `frame_delta_consumed` | subTurn[N].PromptContainsPlanFrameDelta | bool | AC4 |

**确定性承诺**：`ComputeConvergenceMetric(subTurns, lastMetric)` 是纯函数，**0 LLM 调用**。所有字段从 `subTurns[]` deterministic 计算。

## 3. 注入协议

### 3.1 Observe→Plan 注入点（首轮零值边界）

```
BuildObservePriorDelta(prevExecCtx *WorkItemExecContext) FrameDelta
  ├─ prevExecCtx == nil || prevExecCtx.ConvergenceMetric == nil  → FrameDelta{} 零值
  └─ 非首轮：从 ConvergenceMetric + PlanScopeIn 提取
       ├─ UncertaintyReductionRate ≥ 0.5 → "Round N: X% converged, gaps closed: Y"
       ├─ 0.3 ≤ rate < 0.5 → "Round N: partial progress; focus on residual gaps"
       └─ rate < 0.3 → "Round N: stuck; re-evaluate plan or escalate"
```

**兼容性**：append-only 注入，ObservationFrame 9 字段契约 0 修改，DM-20260705-009 封闭式分类器定位不破坏。

### 3.2 Plan→Execute 注入点（双轨 + budget 防御）

```
InjectPlanFrameDelta(ctx, plan.FrameDelta, baseline string) string
  ├─ summary = summarizePlanFrameDelta(plan)        // ≤ 80 字符人读摘要
  ├─ hash = plan.FrameDelta.SchemaHash()            // 稳定 FNV-1a hash
  ├─ injection = "<plan_frame_delta schema=\"...\">summary</plan_frame_delta>"
  ├─ if len(baseline) + len(injection) > 200 → 降级走 baseline（emit warn span）
  └─ return baseline + injection
```

**budget 防御**：Plan ChildSpecs > 5 或 DeliverableContract > 200 字符时降级走 baseline + emit warn span，**不破坏 baseline**。

### 3.3 Execute→Observe 回写（每个 sub-turn）

```
ComputeConvergenceMetric(subTurns []SubTurnRecord, lastMetric *ConvergenceMetric) ConvergenceMetric
  ├─ subTurns 为空 → ConvergenceMetric{} 零值
  ├─ UncertaintyReductionRate = (initial - residual) / initial
  ├─ ObservedGapsClosedCount = initial - residual
  ├─ FrameDeltaConsumed = last subTurn.PromptContainsPlanFrameDelta
  └─ 写入 WorkItemExecContext.FrameDeltaState（atomic.Pointer）
```

**0 LLM 承诺**：通过 `mock LLM 计数 = 0` 单测保证。

## 4. Span 契约

| Span Op | 触发点 | Attribute |
|---------|--------|-----------|
| `d7.s5.observe.prior_delta.span` | `BuildObservePriorDelta` 出口 | `prior_artifact_summary` + `known_gaps` + `span_tag_complete` |
| `d7.s9.execute.plan_frame_delta.inject` | `InjectPlanFrameDelta` 注入完成 | `plan_frame_delta_schema_hash` + `plan_frame_delta_injection_chars` + `injection_status` |
| `d7.s9.execute.convergence_metric.emit` | 每个 sub-turn 结束 `ComputeConvergenceMetric` 后 | `uncertainty_reduction_rate` + `observed_gaps_closed_count` + `frame_delta_consumed` |

**Trace 契约**：`mupsSpan.parent == orchSpan.SpanContext`（与 DM-20260625-019 5-node Span 一致）

## 5. AC 验收标准

| AC | 内容 | 度量方式 |
|----|------|---------|
| AC1 | Observe LLM user frame 含 `prior_artifact_summary` | trace span tag `prior_artifact_summary` 存在 |
| AC2 | Observe LLM user frame 含 `known_gaps` | trace span tag `known_gaps` 存在 + 封闭式 JSON 不破坏 |
| AC3 | Execute system_prompt 注入 plan_frame_delta ≤ 200 字符 | trace span tag `plan_frame_delta_injection_chars` ≤ 200 |
| AC4 | ConvergenceMetric deterministic 0 LLM | mock LLM 计数 = 0 + 5 sub-turn span 完整 |
| AC5 | Jaeger span tag 全可见 | 6 attribute (prior_artifact_summary + known_gaps + plan_frame_delta_schema_hash + uncertainty_reduction_rate + observed_gaps_closed_count + frame_delta_consumed) |
| AC6 | M1/M2 frame 契约 0 修改 | 70+ 现有测试 0 行为变化 PASS |
| AC7 | 跨链 LLM prompt size 单调不增 | trace 上 Observe→Plan→Execute ±5% 噪声内不增 |
| AC8 | S3-Gate 三方共识 | codex + cursor + claude 5 维度 review 无 Critical |

## 6. 演进路径

```
v1.0 (本 Change)
  - FrameDelta 5 字段 + ConvergenceMetric 3 字段
  - append-only 注入，M1/M2 契约 0 修改

v1.1 (LP-1 反向追溯链接入)
  - FrameDelta.TraceID 字段新增（与 TaskSpec.TraceID 对齐）
  - ConvergenceMetric.TraceID 字段新增
  - AssetBuilder 把 TraceID 透传到 SourceTraceID

v2.0 (跨域 FrameDelta 抽象上提)
  - FrameDelta 拆 interfaces/v2 子包（D7 + D2 + D4 共享）
  - Layout guard 白名单扩展
```

## 7. 与现有契约的关系

| 现有契约 | 关系 | 影响 |
|---------|------|------|
| M1 ObservationFrame 9 字段 (DM-20260705-003) | **append-only** 增加 2 字段 | 0 修改，回归测试 0 行为变化 |
| M2 StrategicPlanFrame 16 字段 (DM-20260705-003) | **append-only** 增加 5 字段 | 0 修改，回归测试 0 行为变化 |
| DM-20260705-004 节点 prompt 净化 | 不冲突 | frame delta 字段是 schema-first JSON |
| DM-20260705-008 Strategy 抽象 | 不冲突 | frame delta 注入走 Strategy 旁路，不进 PlanKind 决策表 |
| DM-20260705-009 封闭式分类器 | **兼容** | prior_artifact_summary / known_gaps 字段定义为 obs_fact kind |
| DM-20260704-006 ResolutionContract | 不冲突 | Execute 输出 ResolutionClaim[] 复用承载 convergence_metric |
| DM-20260629-006 TaskContract 统一 | 不冲突 | FrameDelta 是 D7 跨 S kernel 类型，与 TaskSpec / TaskReport 同层 |
| 三层 fail-safe / Pessimistic Commit L3 | 不破坏 | frame delta 注入不破坏 L3 防御 |
| PlanKind / VerdictKind 决策表 | 不破坏 | frame delta 字段不进决策表 |
| workmodel.DivergenceBudget | 不破坏 | budget 字段独立 |

## 8. 参考

- **完整设计（6 段式）**：[design.md](../../changes/devrix-d7-mups-frame-delta-closure/design.md)
- **任务分解（4 Phase × 21 T）**：[tasks.md](../../changes/devrix-d7-mups-frame-delta-closure/tasks.md)
- **提案（决策记录）**：[proposal.md](../../changes/devrix-d7-mups-frame-delta-closure/proposal.md)
- **MUPS 5 节点总图**：[mups-5node-refactor-roadmap.md](mups-5node-refactor-roadmap.md)
- **Pipeline 端到端运行时序**：[pipeline-architecture.md](pipeline-architecture.md)
- **DM-20260704-006 Obs→Verify→Decide 闭环**：spec.md ResolutionContract 段 + uncertainty-spawn-contract.md
- **DM-20260629-006 TaskContract 统一**：interfaces/mups_frame_delta.go 与 task_spec.go / task_report.go 同层