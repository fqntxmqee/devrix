# Delta: D7 Orchestration — MUPS+WorkTree 传播闭环修复

**Change ID:** `devrix-mups-propagation-convergence`  
**Demand ID:** DM-20260701-001  
**Affects:** D7-S1, D7-S5, D7-S8, D7-S9, D7-S15, D7-S16

---

## MODIFIED

### Requirement: D7-S1-A88 UncertaintyReconcile — 收敛允许下降且单一写入

`item.Uncertainty` SHALL 通过唯一函数 `ReconcileUncertainty(prevStored, roundSignal, childStats)` 写入。`item_pipeline` 的 round 计算与 `reevaluateParentAfterChild` MUST 调用同一函数。当子节点全部终态且通过时，结果 MUST 允许低于历史值（移除无条件 `max` ratchet）；进行中轮次得保守取偏高但不得无脑沿用历史最大。

替换：`item_pipeline.go:366-368` 裸 `if item.Uncertainty > uncertaintyMean { uncertaintyMean = item.Uncertainty }`。

#### Scenario: Convergence allows uncertainty to drop
- GIVEN a parent whose children are all `Completed` (stats.Failed=0, Running=0)
- WHEN the rollup round computes uncertainty
- THEN stored `item.Uncertainty` MUST be ≤ the historical uncertainty for those stats
- AND MUST NOT remain pinned at a prior higher value

#### Scenario: Single reconcile entry is order-independent
- GIVEN both the pipeline round and `reevaluateParentAfterChild` write `item.Uncertainty`
- WHEN they execute in either order for the same terminal child set
- THEN the final stored value MUST be identical regardless of write order

---

### Requirement: D7-S15-A89 RollupTerminationGuard — Rollup 重试上限与升级出口

Rollup 轮 SHALL 维护 `RollupRetries` 计数。当 `RollupRetries >= MaxRollupRetries`（默认 3）且 verdict 非 `Pass` 时，`SpawnPolicyEvaluator` MUST 返回 `SpawnEscalateHuman`（不再无条件 `SpawnInline`）。`RunSessionTurnLoop` 在 loop 上限 break 前，若存在 `NeedsRollup` 且未终态的 parent，MUST emit 显式 `human_review`/`error` 事件，不得静默退出。

#### Scenario: Rollup retries exhausted escalates
- GIVEN a rollup parent whose verify returns non-Pass for `MaxRollupRetries` consecutive rounds
- WHEN `SpawnPolicyEvaluator` evaluates the next round
- THEN it MUST return `SpawnEscalateHuman`
- AND a human-review WorkItem MUST be created

#### Scenario: Loop budget exhaustion is not silent
- GIVEN a rollup parent still `InProgress` when the turn loop reaches `defaultSessionTurnLoopMax`
- WHEN the loop is about to `break`
- THEN it MUST emit a `human_review` or `error` `EngineEvent`
- AND MUST NOT terminate the session with an implicit success/empty signal

---

### Requirement: D7-S15-A90 RollupOutcomeAggregation — 全失败不得收敛为成功

Rollup gate/verify SHALL 读取 `ChildOutcomeStats`。当 `Total>0 且 Failed==Total` 时，rollup 收敛结局 MUST NOT 为 `Completed`（至少 `Partial`/`Failed`），且合成的 deliverable MUST 显式列出失败子集。

#### Scenario: All-failed children do not roll up to success
- GIVEN a parent with `stats.Failed == stats.Total`
- WHEN the rollup round produces a well-formed summary that would otherwise pass `verifyRollupArtifact`
- THEN the parent status MUST NOT become `Completed`
- AND the rollup deliverable MUST enumerate the failed children

---

### Requirement: D7-S9-A91 AcceptanceCriteriaVisibility — 验收标准对生产者可见且失败可回灌

当 deliverable schema 适用时，Execute prompt SHALL 包含**人类可读的验收要点**（来源 i18n/`format_hints`，非 Go 硬编码）。当 Verify 返回 `Partial`/`Incomplete` 后进入 inline 重试，上一轮 `verdict.Reason` MUST 回灌进下一轮 Execute prompt。`rollupPlanningDenylist` MUST 迁出 Go 源码到 i18n/`format_hints`。

#### Scenario: Execute prompt carries readable acceptance criteria
- GIVEN `DeliverableSchemaP0P1FileLine` is active
- WHEN the Execute node builds the LLM directive
- THEN the directive MUST contain readable criteria (file:line citation + P0/P1 severity required) sourced from i18n
- AND MUST NOT rely solely on an opaque schema tag

#### Scenario: Verify failure reason is fed back to retry
- GIVEN round N verify returns `Partial` with reason "missing p0/p1 file:line deliverable"
- WHEN round N+1 (inline retry) builds the Execute prompt
- THEN the prompt MUST include the prior failure reason
- AND MUST NOT re-issue the identical directive with no feedback

---

### Requirement: D7-S5-A92 DivergenceBudgetVisibility — 发散提案可见树预算

`StrategicPlanProposer` 的 user prompt SHALL 注入当前 `depth`、`max_depth`、`remaining_children`、`remaining_daily_decompose`、`parent_scope_in`。发散数量/迭代上限 MUST 来自 `workmodel` 单一配置来源，同时供 prompt 文案与 `CapChildSpecs` 使用（消除 prompt 写死 "max 2" 与代码 cap 的漂移）。LLM 提案超额时 MUST 收到结构化 reject（含 `max_allowed`）。

#### Scenario: Plan prompt exposes tree budget
- GIVEN a Plan node about to request a strategic proposal at depth d
- WHEN `buildStrategicPlanUserPrompt` runs
- THEN the prompt MUST include depth, max_depth, remaining_children, remaining_daily_decompose, and parent scope_in

#### Scenario: Budget constants single source
- GIVEN the prompt states a max children count
- THEN that number MUST be derived from the same constant used by `CapChildSpecs`
- AND a hardcoded literal in the prompt appendix MUST NOT diverge from the enforced cap

---

### Requirement: D7-S9-A93 SchemaMonotonicNarrowing — LLM 只能收紧验收 schema

Plan 提案的 `DeliverableSchema` SHALL 只能**收紧**（`not_applicable → 具体 schema`），MUST NOT 放宽（具体 schema → `not_applicable`）已由 directive 关键词推断出的强 schema 信号。

#### Scenario: LLM cannot downgrade inferred schema
- GIVEN `InferDeliverableSchema` returns `p0_p1_file_line` from directive keywords
- WHEN the strategic proposal returns `deliverable_schema = not_applicable`
- THEN the effective schema MUST remain `p0_p1_file_line`

---

## ADDED

### Requirement: D7-S16-A94 ScopeSubdivisionContract — 发散范围真子集校验

`DefaultChildDownlink` MUST NOT 在 `ChildSpec.ScopeIn` 为空时无条件继承父全量 `InScope`。`mapRawChildSpecs`/decompose SHALL 校验：每个 child 的 `ScopeIn` 为父 `InScope` 的真子集；子集合并 SHOULD 覆盖父 scope。Plan prompt SHALL 指引"真子集 / 互不重叠 / 合并覆盖"。

#### Scenario: Child scope must be a subset of parent
- GIVEN a parent with `ScopeContract.InScope = ["a/", "b/"]`
- WHEN a child spec declares `scope_in = ["c/"]` (outside parent)
- THEN decompose MUST reject or clamp the child scope to the parent subset

---

### Requirement: D7-S8-A95 ChildUncertaintyBubble — 子不确定性上浮为不确定性信号

当子 round `UncertaintyMean >= threshold` 时，`observationsFromChildStructuredBubbles` SHALL 额外产出一条 `ObsUncertainty`（而非仅低 strength 的 `ObsFact`），使父在 rollup Observe 阶段能基于子的不确定性继续发散判断。

#### Scenario: High-uncertainty child surfaces as uncertainty
- GIVEN a structured child bubble with `UncertaintyMean = 0.8`
- WHEN the parent rollup Observe collects child bubbles
- THEN the parent UncertaintyReport MUST contain an `ObsUncertainty` derived from that child
- AND MUST NOT represent it only as a low-strength `ObsFact`
