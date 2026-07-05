# Delta: d7-orchestration — Spawn decision-algebra kernel

**Change ID:** `d7-spawn-decision-algebra`
**Demand:** DM-20260705-006
**Affects:** `internal/layers/orchestration/workmodel/spawn_decision_algebra.go` (NEW), `internal/layers/orchestration/workmodel/spawn_policy.go` (MOD)

## ADDED Requirements

### Requirement: `normalizeCtx` helper

`workmodel` 包定义 `normalizeCtx(ctx TreeEvalContext) TreeEvalContext` 函数，对 5 字段做 `<=0` 兜底：

```go
func normalizeCtx(ctx TreeEvalContext) TreeEvalContext {
    if ctx.MaxDepth <= 0 { ctx.MaxDepth = DefaultMaxDecomposeDepth }
    if ctx.Threshold <= 0 { ctx.Threshold = DefaultUncertaintyDecomposeThreshold }
    if ctx.MaxIndeterminateRetries <= 0 { ctx.MaxIndeterminateRetries = DefaultMaxIndeterminateRetries }
    if ctx.MaxRollupRetries <= 0 { ctx.MaxRollupRetries = DefaultMaxRollupRetries }
    if ctx.MaxInlineRetriesAtMaxDepth <= 0 { ctx.MaxInlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth }
    return ctx
}
```

不可变：返回新 ctx（value type，零成本 copy），不改入参。

#### Scenario: 5 字段 default 兜底
- **GIVEN** `TreeEvalContext{MaxDepth: 0, Threshold: 0, MaxIndeterminateRetries: 0, MaxRollupRetries: 0, MaxInlineRetriesAtMaxDepth: 0}`
- **WHEN** `normalizeCtx(ctx)`
- **THEN** 5 字段全部填默认值（`DefaultMaxDecomposeDepth=3` / `DefaultUncertaintyDecomposeThreshold=0.6` / `DefaultMaxIndeterminateRetries=3` / `DefaultMaxRollupRetries=3` / `DefaultMaxInlineRetriesAtMaxDepth=3`）

#### Scenario: 5 字段保留原值
- **GIVEN** `TreeEvalContext{MaxDepth: 5, Threshold: 0.7, ...}`（所有字段 > 0）
- **WHEN** `normalizeCtx(ctx)`
- **THEN** 5 字段全部保留原值

### Requirement: `checkBudget(round, ctx) (SpawnPolicy, bool)` 4 budget gates

R0/R0.5/R1/R2 4 个早退 budget gates；与 VerdictKind 无关；返回 `(SpawnNone, false)` 表示继续下个子决策。

| # | Rule | Fire 条件 | Return |
|---|------|-----------|--------|
| 1 | R0 | `ctx.RunningChildren > 0` | `SpawnAwait, true` |
| 2 | R0.5 | `applicableDeliverableSchema(round) && !deliverableContinuationRequired(round)` | `SpawnNone, true` |
| 3 | R1 | `ctx.Depth >= ctx.MaxDepth` | 3 sub-branch: w/ cont → `spawnForDeliverableContinuation(round, ctx), true`; w/ exhaust → `SpawnEscalateHuman, true`; no schema → `SpawnInline, true` |
| 4 | R2 | `ctx.DailyLimitExceeded` | `SpawnEscalateHuman, true` |
| default | — | 都不 fire | `SpawnNone, false` |

#### Scenario: R0 fire
- **GIVEN** `ctx.RunningChildren = 2`
- **WHEN** `checkBudget(round, ctx)`
- **THEN** 返回 `(SpawnAwait, true)`

#### Scenario: R0.5 fire at depth 0
- **GIVEN** `round.DeliverableSchema = FirstRegisteredDeliverableSchema()` + `round.DeliverableStatus = DeliverableStatusComplete`
- **WHEN** `checkBudget(round, ctx)` (ctx.Depth = 0)
- **THEN** 返回 `(SpawnNone, true)`（deliverable complete → terminal before R1）

#### Scenario: R1 w/ cont fire
- **GIVEN** `ctx.Depth = 3, ctx.MaxDepth = 3, round.DeliverableStatus = DeliverableStatusIncomplete`
- **WHEN** `checkBudget(round, ctx)`
- **THEN** 返回 `(spawnForDeliverableContinuation(round, ctx), true)`（CC-1.1 优先 rollup synth）

#### Scenario: R1 w/ exhaust fire
- **GIVEN** `ctx.Depth = 3, ctx.MaxDepth = 3, ctx.InlineRetriesAtMaxDepth = 3, ctx.MaxInlineRetriesAtMaxDepth = 3, DeliverableStatus = DeliverableStatusIncomplete`
- **WHEN** `checkBudget(round, ctx)`
- **THEN** 返回 `(SpawnEscalateHuman, true)`（CC-1.2 预算耗尽）

#### Scenario: R1 no schema fire
- **GIVEN** `ctx.Depth = 3, ctx.MaxDepth = 3, no deliverable schema`
- **WHEN** `checkBudget(round, ctx)`
- **THEN** 返回 `(SpawnInline, true)`

#### Scenario: R2 fire
- **GIVEN** `ctx.DailyLimitExceeded = true`
- **WHEN** `checkBudget(round, ctx)`
- **THEN** 返回 `(SpawnEscalateHuman, true)`

#### Scenario: fall-through
- **GIVEN** `ctx.RunningChildren = 0, no deliverable, ctx.Depth < ctx.MaxDepth, ctx.DailyLimitExceeded = false`
- **WHEN** `checkBudget(round, ctx)`
- **THEN** 返回 `(SpawnNone, false)`（继续下个子决策）

### Requirement: `checkRollupGuard(round, ctx) (SpawnPolicy, bool)` 跨 verdict guard

RH-MUPS-03 (DM-20260701-001) rollup retry exhausted guard；3 处 verdict 块共享同一 guard；返回 `(SpawnNone, false)` 表示非 rollup 轮 / 继续下个子决策。

| # | Fire 条件 | Return |
|---|-----------|--------|
| 1 | `ctx.RollupRound && ctx.RollupRetries >= ctx.MaxRollupRetries` | `SpawnEscalateHuman, true` |
| 2 | `ctx.RollupRound && ctx.RollupRetries < ctx.MaxRollupRetries` | `SpawnInline, true` |
| default | `!ctx.RollupRound` | `SpawnNone, false` |

#### Scenario: at-limit escalate
- **GIVEN** `ctx.RollupRound = true, ctx.RollupRetries = 3, ctx.MaxRollupRetries = 3`
- **WHEN** `checkRollupGuard(round, ctx)` (round.VerdictKind 任意)
- **THEN** 返回 `(SpawnEscalateHuman, true)`

#### Scenario: below-limit inline
- **GIVEN** `ctx.RollupRound = true, ctx.RollupRetries = 1, ctx.MaxRollupRetries = 3`
- **WHEN** `checkRollupGuard(round, ctx)` (round.VerdictKind = VerdictPartial)
- **THEN** 返回 `(SpawnInline, true)`

#### Scenario: non-rollup fall-through
- **GIVEN** `ctx.RollupRound = false`
- **WHEN** `checkRollupGuard(round, ctx)` (round.VerdictKind = VerdictPartial)
- **THEN** 返回 `(SpawnNone, false)`（继续到 checkVerdictDirection）

### Requirement: `checkVerdictDirection(round, ctx) SpawnPolicy` R3..R8 switch on VerdictKind

| # | Rule | VerdictKind | 决策 |
|---|------|-------------|------|
| 1 | R3 | `VerdictPass + deliverableContinuationRequired` | `spawnForDeliverableContinuation(round, ctx)` (CC-1 §8.1) |
| 2 | R4 | `VerdictPass` else | `SpawnNone` |
| 3 | R5 | `VerdictPartial` | exploratory + CanDecompose+ChildTotal==0 → `SpawnDecompose`; exploratory else → `SpawnInline`; UncertaintyMean >= Threshold → `SpawnDecompose`; deliverableContinuationRequired → `spawnForDeliverableContinuation`; else `SpawnNone` |
| 4 | R6 | `VerdictFail` | ScenarioPlan → `SpawnParallelExplore`; ExplorationPlan+CanDecompose+ChildTotal==0 → `SpawnDecompose`; ExplorationPlan else → `SpawnInline`; else `SpawnNone` |
| 5 | R7 | `VerdictIndeterminate` | exhausted + (exploratory || U>=Threshold) + CanDecompose+ChildTotal==0 → `SpawnDecompose`; exhausted + else → `SpawnEscalateHuman`; else `SpawnInline` |
| 6 | R8 | unknown | `SpawnNone` (default catch-all) |

#### Scenario: R3 (Pass w/ cont)
- **GIVEN** `round.VerdictKind = VerdictPass, round.DeliverableStatus = DeliverableStatusIncomplete`
- **WHEN** `checkVerdictDirection(round, ctx)`
- **THEN** 返回 `spawnForDeliverableContinuation(round, ctx)` (CC-1 §8.1)

#### Scenario: R4 (Pass + CommitmentPlan)
- **GIVEN** `round.VerdictKind = VerdictPass, plan.PlanKind = plan.CommitmentPlan, no deliverable`
- **WHEN** `checkVerdictDirection(round, ctx)`
- **THEN** 返回 `SpawnNone`

#### Scenario: R5 (Partial + CommitmentPlan + low U)
- **GIVEN** `round.VerdictKind = VerdictPartial, plan.PlanKind = plan.ProtocolPlan, UncertaintyMean = 0.5, no cont`
- **WHEN** `checkVerdictDirection(round, ctx)`
- **THEN** 返回 `SpawnNone`

#### Scenario: R6 (Fail + CommitmentPlan)
- **GIVEN** `round.VerdictKind = VerdictFail, plan.PlanKind = plan.CommitmentPlan`
- **WHEN** `checkVerdictDirection(round, ctx)`
- **THEN** 返回 `SpawnNone`

#### Scenario: R7 (Indeterminate + retry < max)
- **GIVEN** `round.VerdictKind = VerdictIndeterminate, plan.PlanKind = plan.ExplorationPlan, ctx.IndeterminateRetries = 1`
- **WHEN** `checkVerdictDirection(round, ctx)`
- **THEN** 返回 `SpawnInline`

#### Scenario: R8 (unknown verdict)
- **GIVEN** `round.VerdictKind = VerdictKind(99)` (unknown)
- **WHEN** `checkVerdictDirection(round, ctx)`
- **THEN** 返回 `SpawnNone` (default catch-all)

### Requirement: `SpawnPolicyEvaluator` 50+→8 行

```go
func SpawnPolicyEvaluator(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
    if round == nil {
        return SpawnNone
    }
    ctx = normalizeCtx(ctx)
    if policy, fired := checkBudget(round, ctx); fired {
        return policy
    }
    if policy, fired := checkRollupGuard(round, ctx); fired {
        return policy
    }
    return checkVerdictDirection(round, ctx)
}
```

行数 8（含函数签名 + nil 兜底 + normalizeCtx + 3 步 checkXxx 显式调用）。比现状 50+ 行减 84%。

#### Scenario: 0 行为变化 (byte-equivalent)
- **GIVEN** 22 组合 (R0-R8 + nil round + 4 rollup guard)
- **WHEN** `SpawnPolicyEvaluator(round, ctx)` vs `SpawnPolicyEvaluatorLegacy(round, ctx)` (build tag `legacy_spawn`)
- **THEN** 22/22 SpawnPolicy 字节级等价（typed enum `==` 比较）

#### Scenario: 3 子决策顺序锁定
- **GIVEN** `ctx.RunningChildren = 2 + round.VerdictKind = VerdictPass` (R0 fired)
- **WHEN** `SpawnPolicyEvaluator(round, ctx)`
- **THEN** checkBudget fired=true → return SpawnAwait; checkRollupGuard 不被调; checkVerdictDirection 不被调
- **GIVEN** `ctx.RollupRound = true, ctx.RollupRetries = 1, ctx.MaxRollupRetries = 3 + round.VerdictKind = VerdictPass` (rollup guard below-limit fired)
- **WHEN** `SpawnPolicyEvaluator(round, ctx)`
- **THEN** checkBudget fired=false (no budget gate); checkRollupGuard fired=true → return SpawnInline; checkVerdictDirection 不被调
- **GIVEN** `round.VerdictKind = VerdictPass, plan.CommitmentPlan, no cont` (direction fired)
- **WHEN** `SpawnPolicyEvaluator(round, ctx)`
- **THEN** checkBudget fired=false; checkRollupGuard fired=false; checkVerdictDirection → return SpawnNone

## MODIFIED

| 文件 | 变更 |
|------|------|
| `internal/layers/orchestration/workmodel/spawn_policy.go` | `SpawnPolicyEvaluator` 50+→8 行；移除内联 if/switch 链 + 5 行 default 兜底 + 3 处 rollup guard 重复 |

## ADDED

| 文件 | 用途 |
|------|------|
| `internal/layers/orchestration/workmodel/spawn_decision_algebra.go` | 3 子决策 + normalizeCtx kernel（~120 行） |
| `internal/layers/orchestration/workmodel/spawn_decision_algebra_test.go` | 3 子决策单测 (15 case) + 1 顺序锁定 + 1 normalizeCtx (~280 行) |
| `internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go` | 旧 `SpawnPolicyEvaluatorLegacy` 50+ 行保留 + build tag `legacy_spawn` (~250 行) |

## REMOVED

- `SpawnPolicyEvaluator` 函数体内 50+ 行 if/switch 链
- `SpawnPolicyEvaluator` 函数体顶部 5 行 `if ctx.X <= 0` default 兜底（被 `normalizeCtx` 取代）
- R5/R6/R7 verdict 块顶部 3 处 `if ctx.RollupRound` 重复 5 行（被 `checkRollupGuard` 取代）

## Invariants

1. **3 子决策顺序是隐性契约**：`checkBudget` → `checkRollupGuard` → `checkVerdictDirection` 顺序由主函数 3 步显式调用声明；修改顺序必须同步更新 `spawn_decision_algebra.go` 注释 + 子决策顺序测试。
2. **0 行为变化承诺**：22 现有测试 0 修改 PASS + 1 byte-equivalent 测试覆盖 22 组合。
3. **`SpawnPolicyEvaluatorLegacy` 是临时死代码**：下个 change (`mups-cleanup-legacy`) 必须删除。
4. **`SpawnPolicy` typed enum `==` 比较足够 byte-equal**：6 态枚举无附加字段；未来若 SpawnPolicy 加结构字段，需改用深比较。
5. **`normalizeCtx` value copy 性能 < 200 ns/次**：远低于决策本身 < 1 μs；不是热点。

## Test Points

| T ID | 描述 | L5 |
|------|------|-----|
| D7-S15-A102-T01 | `checkBudget` 6 case (R0/R0.5/R1 w/ cont/R1 w/ exhaust/R1 no schema/R2 + fall-through) | L5-MUPS-SDA-01 |
| D7-S15-A102-T02 | `checkRollupGuard` 4 case (at-limit escalate/below-limit inline/non-rollup fall-through/Pass+RollupRound inline) | L5-MUPS-SDA-02 |
| D7-S15-A102-T03 | `checkVerdictDirection` 5 case (Pass+R3/R4/Pass w/ cont+CC-1/Partial+R5/Fail+R6/Indeterminate+R7) | L5-MUPS-SDA-03 |
| D7-S15-A102-T04 | `normalizeCtx` 5 字段 default 兜底单测 | L5-MUPS-SDA-04 |
| D7-S15-A102-T05 | `SpawnPolicyEvaluator` 22 组合字节级等价旧实现 | L5-MUPS-SDA-05 |
| D7-S15-A102-T06 | 现有 22 测试 0 修改 PASS (21 + 1 inline) | L5-MUPS-SDA-06 |
| D7-S15-A102-T07 | 3 子决策顺序锁定测试 | L5-MUPS-SDA-07 |
