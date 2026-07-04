# Demand: D7 MUPS 不确定性驱动 Spawn（Deliverable 与拓扑解耦）

- **Demand ID:** DM-20260704-001
- **Change ID:** d7-uncertainty-spawn-decouple
- **Priority:** P0
- **Domain:** D7 Orchestration
- **Status:** S5 Accepted
- **Source:** 会话 `sess_1783138563281_8000` 复盘 + MUPS 架构原则审查（2026-07-04）

---

## 1. 原始描述

> MUPS 五节点管道的设计意图是 **只关注不确定性程度**：U 高时通过 decompose/explore **发散**取证，子 WI terminal 后通过 rollup **收敛**合成结论；**不应**按任务类型（审查 / 实现 / 调研）走不同编排路径。
>
> 当前实现中，`DeliverableContract`（`findings_json`、`planning_meta` 等 **输出格式**）直接驱动 `SpawnInline` 与 CC-1.2 inline 预算耗尽 → 人工升级；`StrategicPlan execution_mode=single` 可绕过 explore/decompose/rollup 拓扑。典型现象：Execute 已读完 scope 内文件（发散成功），Verify 因格式拒收 → 同一 leaf 原地 retry 3 次 → escalate，**从未进入 rollup 收敛相**。
>
> 期望：Spawn/Decide 的主信号回归 **UncertaintyMean + 子树拓扑 + 证据进度**；Deliverable gate 降级为 **呈现/提取层**（CC-1.5 Session complete），不单独决定 MUPS 拓扑；Partial + 证据已足但合成失败时 **强制 rollup synth**，而非 inline 或人工。

---

## 2. 澄清记录

### Q1: 与 DM-20260703-001（收敛契约）的关系？
**A:** DM-001 补齐 CC-1～CC-5 的 terminalization / rollup gate / session exit。**本需求不重复 CC-1.2 inline 预算本身**，而是修正 **何时应走 inline vs decompose vs rollup** 的决策输入：从 deliverable-incomplete 为主，改为 uncertainty+topology 为主。两 change 可同 PR 链或本 change 在 DM-001 之后落地。 — 2026-07-04

### Q2: 是否取消 DeliverableContract / findings_json？
**A:** **否。** 保留 Verify 对输出格式的检测（IM 可读、结构化提取）；**仅**将其从 SpawnPolicy 的 **continuation 主条件** 中解耦。格式失败 → 触发 **收敛相**（rollup synth / salvage extract），而非 **同一 leaf 无限形态 retry**。 — 2026-07-04

### Q3: 是否硬编码 review 或改 Execute 战术 prose？
**A:** **否**（与 DM-001 Q2 一致）。机制 invariant only：SpawnPolicy、StrategicPlan 默认值、rollup 触发、spawnRationale 文案。Execute synthesis 的 thinking 隔离可作为 **可选 Phase 2** 降低 planning_meta 假阳性，非本 change 核心。 — 2026-07-04

### Q4: `execution_mode=single` 是否禁止？
**A:** **不禁止**，但 **需 U 门控**：仅当 `UncertaintyMean < SingleModeThreshold`（默认 0.45）且 scope 已 `IsCompleteEnoughForDecompose` 时允许；否则 Planner 强制 `intent_orchestrate` → 可走 decompose。 — 2026-07-04

### Q5: 人工升级何时仍允许？
**A:** U 仍高 + decompose 预算耗尽 + rollup 失败 N 次（沿用 DM-001 CC-1 rollup retry）；或 daily decompose limit（R2）。**禁止**仅因 deliverable 格式在 depth-0 单 leaf 上 escalate。 — 2026-07-04

---

## 3. L1–L5 映射（草案）

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | D7 | Orchestration | 已有 |
| L2 | D7-S5 | DecisionPlanning + Observe（U 量化） | 已有 |
| L2 | D7-S6 | MUPS Pipeline（Execute / Learn） | 已有 |
| L3-BE | D7-S5-A01 | SpawnPolicyEvaluator | **修改** |
| L3-BE | D7-S5-A95 | UncertaintySpawnContinuation（新增） | **新增** |
| L3-BE | D7-S15-A44 | EvidenceProgressSignal（tool_calls / scope coverage） | **新增** |
| L4-BE | D7-S5-A96 | StrategicPlanSingleModeGate | **新增** |
| L4-BE | D7-S15-A45 | RollupSynthOnFormatFailure | **新增** |
| L4-BE | D7-S4-A51 | DeliverableVerifyPresentationLayer | **修改** |
| L5 | L5-D7-U-01 | Partial + 高证据进度 + U↓ → rollup 非 inline | 草拟 |
| L5 | L5-D7-U-02 | deliverable incomplete 不单独触发 depth-0 inline 第 4 次 escalate | 草拟 |
| L5 | L5-D7-U-03 | spawnRationale 区分 CC-1.2 vs R7 | 草拟 |
| L5 | L5-D7-U-04 | U 高时 strategic single 被拒绝 → decompose | 草拟 |
| L5 | L5-D7-U-05 | session complete 走 ExtractSessionDeliverable salvage | 草拟 |

---

## 4. 根因清单（RH-D7-U）

| ID | 严重度 | 一句话 | 主要代码位 |
|----|--------|--------|-----------|
| RH-D7-U-01 | P0 | `deliverableContinuationRequired` 是 Spawn inline 主因，与 U 无关 | `spawn_policy.go` R0.5/CC-1.2 |
| RH-D7-U-02 | P0 | strategic `single` → CommitmentPlan → Partial 只 inline，不 decompose/rollup | `strategic_plan_proposer.go`, `spawn_policy.go:71-98` |
| RH-D7-U-03 | P0 | 无「证据已足、格式未过」→ rollup synth 路径 | `rollup_gate.go`, `item_pipeline.go` |
| RH-D7-U-04 | P1 | SpawnEscalateHuman rationale 误标 R7 | `spawn_policy.go:178-190` |
| RH-D7-U-05 | P1 | Observe 不携带 verify/deliverable 结构化信号进 U | `item_observe.go` |

---

## 5. In Scope / Out of Scope

**In scope**
- CC-U1～CC-U6 不变式（见 design.md）
- SpawnPolicy 决策树修改 + spawnRationale 修正
- StrategicPlan single 模式 U 门控
- RollupSynthOnEvidence（格式失败收敛）
- Observe 增加 deliverable/verify 结构化 ObsSignal
- 单测 + stub 集成；OpenSpec delta

**Out of scope**
- 重写 MUPS 五节点
- 按任务类型（review/edit）分支
- Execute 战术 prose / prompt 大改（可选 follow-up）
- findings_json parser 字段别名（可并行 hotfix PR，非本 change 阻塞）

---

## 6. 验收口径（L5 Given-When-Then）

### L5-D7-U-01 — 证据足、格式失败 → rollup（P0）
- **GIVEN** Goal depth-0, single leaf, Execute `tool_calls >= 2` covering `ScopeIn`
- **AND** `DeliverableStatus == incomplete` with reason `planning_meta` or `findings_json_incomplete`
- **AND** `UncertaintyMean < 0.5` after round
- **WHEN** `SpawnPolicyEvaluator` runs
- **THEN** `SpawnPolicy != inline` for a 4th consecutive format-only failure
- **AND** parent or ephemeral rollup WI receives `NeedsRollup=true` OR `SpawnDecompose` creates synthesis child
- **AND** session MUST NOT `escalate_human` solely for deliverable format at depth-0

### L5-D7-U-02 — U 高禁止 single 锁拓扑（P0）
- **GIVEN** `UncertaintyMean >= DefaultUncertaintyDecomposeThreshold` (0.6)
- **WHEN** StrategicPlan proposes `execution_mode=single`
- **THEN** proposal rejected or coerced to `decompose`
- **AND** next Plan `QuantizedKind` allows ExplorationPlan spawn rules

### L5-D7-U-03 — spawnRationale 准确（P1）
- **GIVEN** escalate triggered by `InlineRetriesAtMaxDepth` at depth 0 with deliverable incomplete
- **WHEN** `spawnRationale` is recorded
- **THEN** string contains `CC-1.2` or `inline retries exhausted`
- **AND** MUST NOT attribute to `R7 indeterminate`

### L5-D7-U-04 — Session complete salvage（P1）
- **GIVEN** rollup partial success with salvageable findings in artifact
- **WHEN** `buildSessionCompleteEvent` runs
- **THEN** `ExtractSessionDeliverable` returns non-empty best-effort formatted text before `task_incomplete`

### L5-D7-U-05 — 回归：真 U 高仍 decompose（P1）
- **GIVEN** large scope, low tool coverage, `UncertaintyMean >= 0.6`
- **WHEN** first Partial verdict
- **THEN** `SpawnDecompose` or `SpawnInline` with decompose on next round per existing R5/R5-explore

---

## 7. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260703-001 | 前置/并行：CC-1～CC-5 基础设施 |
| DM-20260630-012 | DeliverableVerifier 保留，职责收窄 |
| DM-20260701-001 | uncertainty reconcile、rollup retry |
| sess_1783138563281_8000 | 触发会话 |

---

## 8. 追溯

```
DM-20260704-001 (demand.md)
  → proposal.md / design.md / specs/d7-orchestration_uncertainty_spawn_delta.md
  → tasks.md
  → code: spawn_policy, strategic_plan_proposer, rollup_gate, item_observe, session_complete
  → acceptance-report.md (S5)
```
