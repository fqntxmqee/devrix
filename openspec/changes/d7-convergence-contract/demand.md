# Demand: D7 任务树收敛契约（向下传播 + 向上反馈）

- **Demand ID:** DM-20260703-001
- **Change ID:** d7-convergence-contract
- **Priority:** P0
- **Domain:** D7 Orchestration
- **Status:** S2 Clarified
- **Source:** 会话 `sess_1783064119386_3000` 复盘 + PR #379 (`fix/d7-session-loop-anomaly-exit`) 合入后回归审查 + 2026-07-03 机制 review

---

## 1. 原始描述

> D7 编排域 MUPS 五节点 + WorkTree 已有 SpawnPolicy R0–R8、Rollup Gate、Child Bubble、Deliverable Gate（DM-20260630-012）与传播闭环修复（DM-20260701-001），但 **向下传播（decompose/spawn）** 与 **向上反馈（terminal → rollup → session complete）** 之间仍缺少可执行的 **收敛契约（Convergence Contract）** 文档与代码 invariant。
>
> 典型故障：`review d2 领域 kernel目录下代码` 类任务在 leaf@maxDepth 且 deliverable 已 complete 时，R1 仍强制 `SpawnInline`，子 WI 永不变 terminal → 父 `await_child` 永不开 rollup → PR #379 移除 16 轮 cap 后会话可 30+ 分钟不停止或假 complete。
>
> 期望：引入 Convergence Contract v1（CC-1～CC-4），补齐 R0.5 terminalization、max-depth inline 预算、decompose 前 scope 校验、全层 rollup 与有界 session exit；集成测试覆盖 4 层 decompose + 并行兄弟。

---

## 2. 澄清记录

### Q1: 与 DM-20260630-012 / DM-20260701-001 的边界？
**A:** DM-012 交付 DeliverableVerifier + Session complete gate + StatusAfterSpawnNone；DM-001 修复 uncertainty reconcile、rollup retry 上限、session stagnation 检测。**本需求不重复上述能力**，而是补齐 **SpawnPolicy 规则顺序（R1 先于 deliverable 判定）**、**max depth inline 无上限**、**MaybeDecomposeParentRollup 仅 root Goal**、**decompose 无 repo 路径校验** 四个结构性缺口。 — 2026-07-03

### Q2: 是否硬编码 "review" 关键词或改 LLM prompt 战术？
**A:** **否。** 只改机制 invariant（SpawnPolicy / terminalization / rollup gate / session exit），不改 Execute directive 战术 prose。Review 质量本身 out of scope。 — 2026-07-03

### Q3: PR #379 移除 session loop cap 后本 change 是否回滚 cap？
**A:** **否。** CC-4 用 natural completion + inline 预算 + subtree stuck 替代固定 16 轮；可选 Phase 4 软上限 `MaxMUPSRounds` 作最后保险。 — 2026-07-03

### Q4: `InlineRetriesAtMaxDepth` 计数存哪里？
**A:** 挂在 **WorkItem 字段** `InlineRetriesAtMaxDepth`；每次 max-depth `SpawnInline` increment；terminal / escalate / decompose 成功时清零；ReopenForRollup 不 reset（rollup 与 leaf inline 独立）。 — 2026-07-03

### Q5: T7 是否依赖真实 LLM？
**A:** CI 用 **stub executor 多轮集成**（T7a）；staging 飞书 `review d2 领域 kernel目录下代码` 为 **手工验收**（T7b，非 CI gate）。 — 2026-07-03

### Q6: VerdictPass + deliverable incomplete 边界如何处理？
**A:** Verify 层 invariant：**applicable schema 下 MUST NOT emit Pass when deliverable incomplete**；SpawnPolicy R0.5 作为双保险。 — 2026-07-03

---

## 3. 澄清范围

### 3.1 L1–L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | D7 | Orchestration（编排） | 已有 |
| L2 | D7-S2 | Session Turn Loop / Task Progress | 已有 |
| L2 | D7-S5 | Decision & Planning（SpawnPolicy） | 已有 |
| L2 | D7-S15 | WorkModel Resolve / Rollup Gate | 已有 |
| L3-BE | D7-S2-A06 | RunSessionTurnLoop | 已有 |
| L3-BE | D7-S5-A01 | SpawnPolicyEvaluator | 已有 |
| L3-BE | D7-S15-A01 | ReevaluateParentAfterChild / ShouldRollupAfterChildren | 已有 |
| L4-BE | D7-S5-A93 | ConvergenceContract SpawnPolicy（R0.5 + max-depth budget） | **新增** |
| L4-BE | D7-S2-A86 | ApplyRoundTerminalization + GetPipelineFocus 续跑 | **新增** |
| L4-BE | D7-S5-A94 | ScopeValidator（decompose 前） | **新增** |
| L4-BE | D7-S15-A43 | MaybeParentRollup + SiblingBestEffortRollup | **新增** |
| L4-BE | D7-S2-A87 | EvaluateSessionExit / subtree stuck | **新增** |
| L5 | L5-D7-CC-01 | leaf@maxDepth deliverable complete → terminal | 草拟 |
| L5 | L5-D7-CC-02 | max depth inline retry 预算耗尽 → escalate/fail | 草拟 |
| L5 | L5-D7-CC-03 | 4 层 + 并行兄弟 1 complete 1 stuck → 有界 session exit | 草拟 |
| L5 | L5-D7-CC-04 | 4 层 decompose 链逐层 rollup → root deliverable | 草拟 |
| L5 | L5-D7-CC-05 | LLM 幻觉 scope → Validator reject + fallback | 草拟 |
| L5 | L5-D7-CC-06 | rollup fail×3 → human_review | 草拟（已有 DM-001，回归） |
| L5 | L5-D7-CC-07 | review d2 kernel stub 多轮 + staging 手工 | 草拟 |

### 3.2 根因清单（RH-D7-CC）

| ID | 严重度 | 一句话 | 主要代码位 |
|----|--------|--------|-----------|
| RH-D7-CC-01 | P0 | R1 max depth 在 deliverable 判定前强制 Inline，complete leaf 无法 terminal | `spawn_policy.go:40-42` |
| RH-D7-CC-02 | P0 | max depth incomplete 无 inline 上限，PR #379 后 infinite MUPS | `spawn_policy.go` + `session_turn_loop.go` |
| RH-D7-CC-03 | P1 | MaybeDecomposeParentRollup 仅扫 root Goal，中间 Implement 父漏 rollup | `rollup_gate.go:174-181` |
| RH-D7-CC-04 | P1 | 并行兄弟 1 complete + 1 stuck，Running>0 永不开 rollup | `resolve.go` + `rollup_gate.go` |
| RH-D7-CC-05 | P2 | decompose ChildSpecs 无 repo 存在性校验，幻觉路径放大阻塞 | `spawn_apply.go` PrepareDecomposeSpecs |

### 3.3 In Scope / Out of Scope

**In scope**
- CC-1 Round Terminalization（R0.5、max-depth budget、ApplyRoundTerminalization、focus 续跑）
- CC-2 Downward ScopeValidator + general schema decompose 扩展
- CC-3 Upward MaybeParentRollup、MaybeSiblingBestEffortRollup
- CC-4 Session exit boundedness（subtree stuck、统一 EvaluateSessionExit）
- 集成测试 T1–T7；OpenSpec delta + pipeline-architecture 交叉引用

**Out of scope**
- 重写 MUPS 五节点；LLM 直接设置 SpawnPolicy
- Review 内容质量；跨 session 调度
- 回滚 PR #379 cap（用机制替代）

---

## 4. 验收口径（L5 Given-When-Then）

### L5-D7-CC-01 — Round terminalization at max depth（P0）
- **GIVEN** WorkItem at `Depth >= MaxDecomposeDepth` AND `DeliverableStatus == complete`
- **WHEN** `SpawnPolicyEvaluator` runs after Verify
- **THEN** `SpawnPolicy == none` AND `TaskStatus` terminal AND item NOT refocused for inline retry

### L5-D7-CC-02 — Max depth inline budget（P0）
- **GIVEN** WorkItem at max depth AND `DeliverableContinuationRequired == true`
- **WHEN** `InlineRetriesAtMaxDepth >= MaxInlineRetriesAtMaxDepth` (default 3)
- **THEN** `SpawnPolicy ∈ {escalate_human, failed}` AND session loop MUST NOT schedule unbounded inline rounds

### L5-D7-CC-03 — Sibling best-effort + bounded session（P0）
- **GIVEN** 4-level tree with 2 sibling Implement children under same parent
- **AND** one sibling terminal complete, one exhausted max-depth inline retries incomplete
- **WHEN** `MaybeSiblingBestEffortRollup` runs
- **THEN** stuck sibling failed with reason `inline_retries_exhausted_at_max_depth`
- **AND** parent `NeedsRollup == true` AND session completes within configured N MUPS rounds

### L5-D7-CC-04 — Full decompose chain rollup（P1）
- **GIVEN** Goal → Implement → Implement → Implement decompose chain, deepest leaf completes
- **WHEN** each level's direct children all terminal
- **THEN** each decompose parent rollup bottom-up AND root `ExtractSessionDeliverable` non-empty

### L5-D7-CC-05 — Scope validation（P1）
- **GIVEN** StrategicPlan emits `ScopeIn` paths not existing under session WorkDir
- **WHEN** `PrepareDecomposeSpecs` runs
- **THEN** invalid paths rejected AND fallback ensures ≥1 valid child OR DefaultDecomposeProposer structural split

### L5-D7-CC-06 — Rollup retry escalation（P1，回归 DM-001）
- **GIVEN** rollup verify fails 3 consecutive rounds
- **WHEN** `SpawnPolicyEvaluator` runs on rollup parent
- **THEN** `SpawnEscalateHuman` AND user-visible human_review event

### L5-D7-CC-07 — Review task E2E（P0 stub / P1 staging）
- **GIVEN** stub executor: round1 planning-only incomplete, round2 valid p0_p1 file:line
- **WHEN** `RunSessionTurnLoop` runs
- **THEN** executor calls ≥2, complete WITHOUT `task_incomplete=true`
- **AND** (staging manual) 飞书 `review d2 领域 kernel目录下代码` complete 非 task_incomplete

---

## 5. 关联变更

| 变更 | 关系 |
|------|------|
| DM-20260630-012 | 前置：DeliverableVerifier、StatusAfterSpawnNone、Session complete |
| DM-20260701-001 | 前置：rollup retry、sessionNoForwardProgress、uncertainty reconcile |
| PR #379 | 互补：移除 cap → 本 change 提供 terminal 闭环 |
| DM-20260701-007 | 无直接依赖；ToolSpec ConvergenceContract 为 D2 侧，本 change 为 D7 运行时 invariant |

---

## 6. 追溯

```
DM-20260703-001 (demand.md)
  → proposal.md / design.md / specs/d7-orchestration_convergence_delta.md
  → tasks.md (Phase 1–4, T1–T7)
  → code: spawn_policy, terminalize, scope_validator, rollup_gate, session_loop_signals
  → acceptance-report.md (S5)
```
