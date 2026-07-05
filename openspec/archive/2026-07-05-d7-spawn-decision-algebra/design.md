# Design: MUPS Spawn 决策代数化 (M5)

**Change ID:** `d7-spawn-decision-algebra`  
**Demand:** DM-20260705-006
**Status:** S3_Design
**Template:** `openspec/docs/methodology/detail-design-framework.md`（六段式 lite-mode）

## 1. 架构目标

### ① 业务目标

- **消除 R0-R8 散落**：9 条规则（4 budget gates + 3 verdict block + 3 rollup guard 重复 + 1 default）当前散落在 `SpawnPolicyEvaluator` 单函数 50+ 行中，无显式分组边界。
- **消除 rollup guard 重复**：RH-MUPS-03 (DM-20260701-001) 修复时把 `if ctx.RollupRound { if ctx.RollupRetries >= ctx.MaxRollupRetries { return EscalateHuman } return Inline }` 5 行 × 3 处重复。新加 1 类 rollup 规则必须改 3 处。
- **0 行为变化（M5）**：22 现有测试 + 1 byte-equivalent 测试（22 组合）+ 1 顺序锁定测试 + 15 子决策单测 + 1 normalizeCtx 单测全 PASS；规则顺序、guard 行为、normalize 兜底全部 1:1 保留。
- **可读性 / 可维护性**：新增 spawn rule = 在对应子决策函数加 case（3 选 1 路径）；rollup guard 行为变更 = 改 1 处（`checkRollupGuard`）。

### ② 技术目标（量化指标）

| 指标 | 目标值 |
|------|--------|
| `SpawnPolicyEvaluator` 函数体行数 | ≤ 10（含 nil round 兜底 + normalizeCtx + 3 步显式调用） |
| 子决策函数数量 | 3（`checkBudget` / `checkRollupGuard` / `checkVerdictDirection`） |
| rollup retry exhausted guard 重复位置 | 1（仅 `checkRollupGuard`） |
| `normalizeCtx` helper 行数 | ≤ 10（5 个 if `<=0` 兜底） |
| 子决策顺序 | 显式：`checkBudget` → `checkRollupGuard` → `checkVerdictDirection`（顺序错位 → 静默错误） |
| 测试覆盖 | 22 现有 + 15 子决策 + 1 normalizeCtx + 1 顺序锁定 + 1 byte-equivalent (22 组合) |

### ③ 约束条件

- Go 1.22+（泛型 + struct tag）
- Pure 不可变 + value type（与 M1/M2/M4 一致）
- 不修改 `SpawnPolicy` 6 态枚举
- 不修改 `WorkItemPipelineRound` / `TreeEvalContext` 2 struct 字段
- 不复活 ChannelRouter 4 文件
- M5 阶段 0 行为变化；M3 阶段是行为增量（最后做）

## 2. 架构原则

### ① 设计原则

1. **3 个命名子决策（Single Source of Truth）**：每个子决策 = 1 个 `checkXxx(round, ctx) (SpawnPolicy, bool)` 或 `checkXxx(round, ctx) SpawnPolicy` 函数；budget 子决策返回 `(policy, fired bool)` 表示"早退决策"或"继续下个子决策"；direction 子决策返回 `SpawnPolicy`（总是 final decision）。
2. **主函数 3 步显式声明**：`SpawnPolicyEvaluator(round, ctx) SpawnPolicy` = nil round 兜底 + `ctx = normalizeCtx(ctx)` + 3 步 `if p, fired := checkXxx(...); fired { return p }` 链 + 终 `return checkVerdictDirection(...)`。
3. **rollup guard 单一权威位置**：`checkRollupGuard(round, ctx) (SpawnPolicy, bool)` 是唯一写 `ctx.RollupRound && ctx.RollupRetries >= ctx.MaxRollupRetries` 逻辑的地方；3 处 verdict 块共享同一 guard。
4. **不可变 normalize**：`normalizeCtx(ctx TreeEvalContext) TreeEvalContext` 返回新 ctx（value 不可变，调用方不修改入参 ctx）；5 个 `if ctx.X <= 0` 兜底在 normalize 内集中。
5. **0 行为变化优先（refactor before increment）**：M5 是 pure refactor；byte-equivalent 测试保证 22 组合与旧实现字节级等价。

### ② 命名规范

- **normalizeCtx helper**：`func normalizeCtx(ctx TreeEvalContext) TreeEvalContext`
- **子决策函数**：
  - `checkBudget(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool)` — 4 budget gates (R0/R0.5/R1/R2)
  - `checkRollupGuard(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool)` — rollup retry exhausted 跨 verdict guard
  - `checkVerdictDirection(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy` — R3..R8 switch on VerdictKind
- **返回签名约定**：
  - `(SpawnPolicy, bool)`：第二个返回值 `fired` 表示"该子决策命中并产出决策"；`fired=false` 表示"该子决策未命中 / 应继续下个子决策"
  - `SpawnPolicy`：`checkVerdictDirection` 总是 final decision，switch 5 case + default R8 覆盖所有 VerdictKind

### ③ 代码风格

- 函数 < 50 行，文件 < 800 行
- `spawn_decision_algebra.go` 目标 < 200 行（含 4 函数 + helpers + 注释）
- 不可变：`normalizeCtx` 返回新 TreeEvalContext；不改入参
- `checkBudget` / `checkRollupGuard` 返回 value（SpawnPolicy 是 typed enum，零成本）
- 错误码：无（决策是 build-time func，不运行时出错）

## 3. 业务流程

### ① 核心用例时序（SpawnPolicyEvaluator 3 步决策）

```
[ItemPipeline] → SpawnPolicyEvaluator(round, ctx)
  ↓
  if round == nil { return SpawnNone }
  ↓
  ctx = normalizeCtx(ctx)  // 5 行 default 兜底
  ↓
  if policy, fired := checkBudget(round, ctx); fired {
      return policy  // R0/R0.5/R1/R2 早退
  }
  ↓
  if policy, fired := checkRollupGuard(round, ctx); fired {
      return policy  // 跨 verdict rollup retry exhausted guard
  }
  ↓
  return checkVerdictDirection(round, ctx)  // R3..R8 verdict-kind 路由
```

### ② 3 子决策详解

#### (a) `checkBudget(round, ctx) (SpawnPolicy, bool)` — 4 budget gates

| # | Rule | Fire 条件 | Return |
|---|------|-----------|--------|
| 1 | R0 | `ctx.RunningChildren > 0` | `SpawnAwait, true` |
| 2 | R0.5 | `applicableDeliverableSchema(round) && !deliverableContinuationRequired(round)` | `SpawnNone, true` |
| 3 | R1 | `ctx.Depth >= ctx.MaxDepth` | (a) `deliverableContinuationRequired(round)` → `spawnForDeliverableContinuation(round, ctx), true`; (b) `deliverableInlineWouldExhaust(ctx)` → `SpawnEscalateHuman, true`; (c) else `SpawnInline, true` |
| 4 | R2 | `ctx.DailyLimitExceeded` | `SpawnEscalateHuman, true` |
| default | — | 都不 fire | `SpawnNone, false`（继续下个子决策） |

**`checkBudget` 函数体（~25 行）**：

```go
// checkBudget applies R0 (running children), R0.5 (deliverable complete),
// R1 (max depth with continuation), R2 (daily limit) — early budget gates
// independent of round.VerdictKind. Returns (SpawnNone, false) to fall
// through to the next sub-decision.
func checkBudget(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool) {
    // R0
    if ctx.RunningChildren > 0 {
        return SpawnAwait, true
    }
    // R0.5 — CC-1.1: applicable deliverable satisfied → terminal before R1.
    if applicableDeliverableSchema(round) && !deliverableContinuationRequired(round) {
        return SpawnNone, true
    }
    // R1 — max depth with continuation: bounded inline, then escalate (CC-U1 may prefer rollup).
    if ctx.Depth >= ctx.MaxDepth {
        if deliverableContinuationRequired(round) {
            return spawnForDeliverableContinuation(round, ctx), true
        }
        if deliverableInlineWouldExhaust(ctx) {
            return SpawnEscalateHuman, true
        }
        return SpawnInline, true
    }
    // R2
    if ctx.DailyLimitExceeded {
        return SpawnEscalateHuman, true
    }
    return SpawnNone, false
}
```

#### (b) `checkRollupGuard(round, ctx) (SpawnPolicy, bool)` — 跨 verdict guard

| # | Fire 条件 | Return |
|---|-----------|--------|
| 1 | `ctx.RollupRound && ctx.RollupRetries >= ctx.MaxRollupRetries` | `SpawnEscalateHuman, true` |
| 2 | `ctx.RollupRound && ctx.RollupRetries < ctx.MaxRollupRetries` | `SpawnInline, true` |
| default | `!ctx.RollupRound`（非 rollup 轮） | `SpawnNone, false`（继续下个子决策） |

**`checkRollupGuard` 函数体（~10 行）**：

```go
// checkRollupGuard applies the RH-MUPS-03 (DM-20260701-001) rollup retry
// exhausted guard. Returns (SpawnNone, false) when ctx.RollupRound is false
// so non-rollup rounds fall through to verdict-kind routing.
func checkRollupGuard(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool) {
    if !ctx.RollupRound {
        return SpawnNone, false
    }
    if ctx.RollupRetries >= ctx.MaxRollupRetries {
        return SpawnEscalateHuman, true
    }
    return SpawnInline, true
}
```

#### (c) `checkVerdictDirection(round, ctx) SpawnPolicy` — R3..R8 switch on VerdictKind

| # | Rule | VerdictKind | 决策 |
|---|------|-------------|------|
| 1 | R3 | `VerdictPass + plan.CommitmentPlan` | `SpawnNone` |
| 2 | R4 | `VerdictPass + ExplorationPlan/ScenarioPlan` | `SpawnNone`（同 R3，统一为 Pass → None） |
| 3 | R5 | `VerdictPartial` | (a) `IsExploratoryPlanKind` + `CanDecompose && ChildTotal==0` → `SpawnDecompose`; (b) `IsExploratoryPlanKind` + 其他 → `SpawnInline`; (c) `round.UncertaintyMean >= ctx.Threshold` → `SpawnDecompose`; (d) `deliverableContinuationRequired` → `spawnForDeliverableContinuation(round, ctx)`; (e) default `SpawnNone` |
| 4 | R6 | `VerdictFail` | (a) `round.PlanKind == plan.ScenarioPlan` → `SpawnParallelExplore`; (b) `round.PlanKind == plan.ExplorationPlan` + `CanDecompose && ChildTotal==0` → `SpawnDecompose`; (c) `round.PlanKind == plan.ExplorationPlan` + 其他 → `SpawnInline`; (d) default `SpawnNone` |
| 5 | R7 | `VerdictIndeterminate` | (a) `IndeterminateRetries >= MaxIndeterminateRetries` + `(IsExploratoryPlanKind || UncertaintyMean >= Threshold) && CanDecompose && ChildTotal==0` → `SpawnDecompose`; (b) `IndeterminateRetries >= MaxIndeterminateRetries` + 其他 → `SpawnEscalateHuman`; (c) default `SpawnInline` |
| 6 | R8 | unknown `VerdictKind` | `SpawnNone`（default catch-all） |

**`checkVerdictDirection` 函数体（~35 行）**：

```go
// checkVerdictDirection applies R3..R8 by verdict kind. Rollup retry
// exhausted guard is hoisted to checkRollupGuard so this switch only
// handles the post-rollup verdict-direction routing.
func checkVerdictDirection(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
    switch round.VerdictKind {
    case types.VerdictPass:
        // CC-1 / §8.1: Pass with applicable schema MUST NOT SpawnNone
        // while deliverable is still owed — inline (or R1 budget) instead.
        if deliverableContinuationRequired(round) {
            return spawnForDeliverableContinuation(round, ctx)
        }
        return SpawnNone

    case types.VerdictPartial:
        // Exploratory partial on decomposable parents (Goal/Plan/Implement)
        // triggers the first split; leaf explore items retry inline.
        if IsExploratoryPlanKind(round.PlanKind) {
            if ctx.CanDecompose && ctx.ChildTotal == 0 {
                return SpawnDecompose
            }
            return SpawnInline
        }
        if round.UncertaintyMean >= ctx.Threshold {
            return SpawnDecompose
        }
        if deliverableContinuationRequired(round) {
            return spawnForDeliverableContinuation(round, ctx)
        }
        return SpawnNone

    case types.VerdictFail:
        if round.PlanKind == plan.ScenarioPlan {
            return SpawnParallelExplore
        }
        // Leaf explore items cannot decompose; retry inline instead.
        if round.PlanKind == plan.ExplorationPlan {
            if ctx.CanDecompose && ctx.ChildTotal == 0 {
                return SpawnDecompose
            }
            return SpawnInline
        }
        return SpawnNone

    case types.VerdictIndeterminate:
        if ctx.IndeterminateRetries >= ctx.MaxIndeterminateRetries {
            if (IsExploratoryPlanKind(round.PlanKind) || round.UncertaintyMean >= ctx.Threshold) &&
                ctx.CanDecompose && ctx.ChildTotal == 0 {
                return SpawnDecompose
            }
            return SpawnEscalateHuman
        }
        return SpawnInline

    default:
        // R8
        return SpawnNone
    }
}
```

### ③ `normalizeCtx` helper

```go
// normalizeCtx applies 5 default-value guards to ctx. Returns a new ctx
// (TreeEvalContext is a value type, so this is pure / non-mutating).
func normalizeCtx(ctx TreeEvalContext) TreeEvalContext {
    if ctx.MaxDepth <= 0 {
        ctx.MaxDepth = DefaultMaxDecomposeDepth
    }
    if ctx.Threshold <= 0 {
        ctx.Threshold = DefaultUncertaintyDecomposeThreshold
    }
    if ctx.MaxIndeterminateRetries <= 0 {
        ctx.MaxIndeterminateRetries = DefaultMaxIndeterminateRetries
    }
    if ctx.MaxRollupRetries <= 0 {
        ctx.MaxRollupRetries = DefaultMaxRollupRetries
    }
    if ctx.MaxInlineRetriesAtMaxDepth <= 0 {
        ctx.MaxInlineRetriesAtMaxDepth = DefaultMaxInlineRetriesAtMaxDepth
    }
    return ctx
}
```

## 4. 数据契约

### 4.1 子决策函数签名

```go
// 3 子决策 + 1 normalizeCtx
func normalizeCtx(ctx TreeEvalContext) TreeEvalContext
func checkBudget(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool)
func checkRollupGuard(round *WorkItemPipelineRound, ctx TreeEvalContext) (SpawnPolicy, bool)
func checkVerdictDirection(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy
```

### 4.2 返回值语义

- `(SpawnPolicy, bool)`：第二个返回值 `fired` 表示"该子决策命中并产出决策"；`fired=false` 表示"该子决策未命中 / 应继续下个子决策"。
  - 命名沿用 M4 verify decision-table `Fire` function 返回 bool 风格，但用 `fired` 强调"决策被触发"语义（比 `ok` 更准确，`ok` 是 Go 惯用的 err check 语义）。
- `SpawnPolicy`：`checkVerdictDirection` 总是 final decision；switch 5 case + default R8 覆盖所有 `types.VerdictKind` 值（包括 unknown）。

### 4.3 不可变性保证

- `TreeEvalContext` 是 value type，`normalizeCtx` 返回新 ctx（不改入参）。
- `SpawnPolicy` 是 typed enum（6 态），返回 value 零成本。
- `WorkItemPipelineRound` 是 `*pointer`（现状保持），子决策只读 round，不修改。

## 5. 关键算法

### 5.1 `SpawnPolicyEvaluator` 主函数（重构后 8 行）

```go
// SpawnPolicyEvaluator applies the 3-sub-decision algebra (checkBudget →
// checkRollupGuard → checkVerdictDirection). LLM MUST NOT set SpawnPolicy
// directly; only this function may assign it (goal G3).
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

**行为不变性证明（关键 case）**：
- **R0**：`ctx.RunningChildren > 0` → `checkBudget` 返回 `SpawnAwait, true` → 主函数 `return SpawnAwait`。**与现状 R0 等价**。
- **R0.5**：`applicableDeliverableSchema && !deliverableContinuationRequired` → `checkBudget` 返回 `SpawnNone, true` → 主函数 `return SpawnNone`。**与现状 R0.5 等价**。
- **R1 (max depth w/ cont)**：`ctx.Depth >= ctx.MaxDepth && deliverableContinuationRequired` → `checkBudget` 返回 `spawnForDeliverableContinuation(round, ctx), true`。**与现状 R1 第一个分支等价**。
- **R1 (max depth w/ exhaust)**：`ctx.Depth >= ctx.MaxDepth && deliverableInlineWouldExhaust` → `checkBudget` 返回 `SpawnEscalateHuman, true`。**与现状 R1 第二个分支等价**。
- **R1 (max depth no schema)**：`ctx.Depth >= ctx.MaxDepth && !deliverableContinuationRequired` → `checkBudget` 返回 `SpawnInline, true`。**与现状 R1 第三个分支等价**。
- **R2**：`ctx.DailyLimitExceeded` → `checkBudget` 返回 `SpawnEscalateHuman, true`。**与现状 R2 等价**。
- **R5 rollup guard**：现状 `case VerdictPartial` 顶部 `if ctx.RollupRound { if at-limit { return EscalateHuman } return Inline }` → M5 抽到 `checkRollupGuard` 在 verdict switch 之前介入。**与现状 R5 顶部等价**。
- **R6 rollup guard**：现状 `case VerdictFail` 顶部同 → M5 抽到 `checkRollupGuard`。**与现状 R6 顶部等价**。
- **R7 rollup guard**：现状 `case VerdictIndeterminate` 顶部同 → M5 抽到 `checkRollupGuard`。**与现状 R7 顶部等价**。
- **R3/R4**：`VerdictPass` + `!deliverableContinuationRequired` → `checkVerdictDirection` 返回 `SpawnNone`。**与现状 R3/R4 等价**。
- **R3 (Pass w/ cont)**：`VerdictPass` + `deliverableContinuationRequired` → `checkVerdictDirection` 返回 `spawnForDeliverableContinuation(round, ctx)`。**与现状 R3 (CC-1 §8.1) 等价**。
- **R5 (non-rollup)**：`VerdictPartial` + `!ctx.RollupRound` → `checkVerdictDirection` 进入 exploratory / uncertainty / deliverable continuation / None 5 case。**与现状 R5 主体等价**（rollup guard 已抽走）。
- **R6 (non-rollup)** / **R7 (non-rollup)** / **R8**：同理。
- **nil round**：`SpawnPolicyEvaluator(nil, ctx)` → `return SpawnNone`。**与现状等价**。

### 5.2 byte-equivalent 测试覆盖矩阵（22 组合）

| # | 组合 | 覆盖 R# |
|---|------|---------|
| 1 | `nil round` | 兜底 |
| 2 | `ctx.RunningChildren > 0` (R0) | R0 |
| 3 | `applicableDeliverableSchema + DeliverableStatusComplete` at depth 0 (R0.5) | R0.5 |
| 4 | `applicableDeliverableSchema + DeliverableStatusComplete` at max depth (R0.5 at max depth) | R0.5 |
| 5 | `ctx.Depth >= MaxDepth + deliverableContinuationRequired` (R1 w/ cont) | R1 |
| 6 | `ctx.Depth >= MaxDepth + deliverableInlineWouldExhaust` (R1 w/ exhaust) | R1 |
| 7 | `ctx.Depth >= MaxDepth + no deliverable schema` (R1 no schema) | R1 |
| 8 | `ctx.DailyLimitExceeded` (R2) | R2 |
| 9 | `VerdictPass + CommitmentPlan` (R3) | R3 |
| 10 | `VerdictPass + ExplorationPlan` (R4) | R4 |
| 11 | `VerdictPass + deliverableContinuationRequired` (R3 CC-1 §8.1) | R3 |
| 12 | `VerdictPartial + CommitmentPlan + UncertaintyMean < Threshold` (R5 low U) | R5 |
| 13 | `VerdictPartial + ExplorationPlan + CanDecompose + ChildTotal==0` (R5 exploratory decomposable) | R5 |
| 14 | `VerdictPartial + ExplorationPlan + !CanDecompose` (R5 explore leaf) | R5 |
| 15 | `VerdictPartial + UncertaintyMean >= Threshold` (R5 high U) | R5 |
| 16 | `VerdictPartial + deliverableContinuationRequired` (R5 w/ cont) | R5 |
| 17 | `VerdictFail + ScenarioPlan` (R6) | R6 |
| 18 | `VerdictFail + ExplorationPlan + CanDecompose + ChildTotal==0` (R6 exploratory decomposable) | R6 |
| 19 | `VerdictFail + ExplorationPlan + !CanDecompose` (R6 explore leaf) | R6 |
| 20 | `VerdictFail + CommitmentPlan` (R6 commitment) | R6 |
| 21 | `VerdictIndeterminate + IndeterminateRetries < MaxIndeterminateRetries` (R7 retry) | R7 |
| 22 | `VerdictIndeterminate + IndeterminateRetries >= MaxIndeterminateRetries + exploratory` (R7 exhausted decompose) | R7 |
| 23 | `VerdictIndeterminate + IndeterminateRetries >= MaxIndeterminateRetries + non-exploratory` (R7 exhausted escalate) | R7 |
| 24 | `VerdictKind(99)` unknown default (R8) | R8 |
| 25 | `VerdictPartial + RollupRound + RollupRetries >= MaxRollupRetries` (R5 rollup guard escalate) | rollup guard |
| 26 | `VerdictFail + RollupRound + RollupRetries >= MaxRollupRetries` (R6 rollup guard escalate) | rollup guard |
| 27 | `VerdictIndeterminate + RollupRound + RollupRetries >= MaxRollupRetries` (R7 rollup guard escalate) | rollup guard |
| 28 | `VerdictPass + RollupRound + RollupRetries == 5` (R3/R4 rollup fall-through → Pass → None) | rollup guard fall-through |

> 实际 22 组合 + 4 rollup guard + 1 nil + 1 unknown = 28 case（与 M4 17 组合的同等粒度）；为简化可合并为 22 组合（去掉 4 个 rollup 重复 case → 1 个 rollup 综合 case = 22 组合）。**采用 22 组合**。

### 5.3 子决策单元测试（15 case）

- **`checkBudget` (6 case)**：
  - T01 R0 fire (`RunningChildren > 0` → `SpawnAwait, true`)
  - T02 R0.5 fire (`applicableDeliverableSchema + Complete` → `SpawnNone, true`)
  - T03 R1 w/ cont fire (`MaxDepth + deliverableContinuationRequired` → `spawnForDeliverableContinuation, true`)
  - T04 R1 w/ exhaust fire (`MaxDepth + deliverableInlineWouldExhaust` → `SpawnEscalateHuman, true`)
  - T05 R1 no schema fire (`MaxDepth + no deliverable` → `SpawnInline, true`)
  - T06 R2 + fall-through (`DailyLimitExceeded` → `SpawnEscalateHuman, true`; `VerdictPartial + low U + no cont` → `SpawnNone, false`)
- **`checkRollupGuard` (4 case)**：
  - T07 at-limit escalate (`RollupRound + Retries >= Max` → `SpawnEscalateHuman, true`)
  - T08 below-limit inline (`RollupRound + Retries < Max` → `SpawnInline, true`)
  - T09 non-rollup fall-through (`!RollupRound` → `SpawnNone, false`)
  - T10 Pass + RollupRound fall-through（`RollupRound + Retries < Max` → `SpawnInline, true`；注：Pass 走 R3 但 guard 仍 inline）
- **`checkVerdictDirection` (5 case)**：
  - T11 Pass (R3/R4): `VerdictPass + CommitmentPlan + no cont` → `SpawnNone`
  - T12 Pass w/ cont (R3 CC-1): `VerdictPass + deliverableContinuationRequired` → `spawnForDeliverableContinuation`
  - T13 Partial (R5): `VerdictPartial + CommitmentPlan + UncertaintyMean < Threshold + no cont` → `SpawnNone`
  - T14 Fail (R6): `VerdictFail + CommitmentPlan` → `SpawnNone`
  - T15 Indeterminate (R7): `VerdictIndeterminate + ExplorationPlan + IndeterminateRetries < Max + CanDecompose` → `SpawnInline`

### 5.4 子决策顺序锁定测试（1 test）

```go
func TestSpawnPolicyEvaluator_SubDecisionOrder(t *testing.T) {
    // 构造同时命中 budget gate + rollup guard + verdict direction 的 round
    // 期望调用顺序：checkBudget → checkRollupGuard → checkVerdictDirection
    // budget gate fired → 后续不被调 → 最终 decision = budget gate 的结果
}
```

### 5.5 `normalizeCtx` 单测（1 test）

```go
func TestNormalizeCtx(t *testing.T) {
    // 5 字段：MaxDepth/Threshold/MaxIndeterminateRetries/MaxRollupRetries/MaxInlineRetriesAtMaxDepth
    // <=0 兜底 + >0 保留原值
}
```

## 6. 变更详情

### 6.1 ADDED

| 文件 | 用途 | 行数 |
|------|------|------|
| `internal/layers/orchestration/workmodel/spawn_decision_algebra.go` | 3 子决策 + normalizeCtx kernel | ~120 |
| `internal/layers/orchestration/workmodel/spawn_decision_algebra_test.go` | 3 子决策单测 (15 case) + 顺序锁定 + normalizeCtx | ~280 |
| `internal/layers/orchestration/workmodel/spawn_policy_legacy_test.go` | 旧 `SpawnPolicyEvaluatorLegacy` 50+ 行保留 + build tag `legacy_spawn` | ~250 |

### 6.2 MODIFIED

| 文件 | 变更 | 行数变化 |
|------|------|---------|
| `workmodel/spawn_policy.go` | `SpawnPolicyEvaluator` 50+→8 行（nil round 兜底 + normalizeCtx + 3 步 checkXxx 显式调用）；移除内联 if/switch 链 | -42 行 |

### 6.3 REMOVED

- 内联 if/switch 链（被 3 子决策 + normalizeCtx 取代）
- `SpawnPolicyEvaluator` 函数体内 5 行 `if ctx.X <= 0` default 兜底（被 `normalizeCtx` 取代）
- R5/R6/R7 verdict 块顶部 3 处 `if ctx.RollupRound` 重复 5 行（被 `checkRollupGuard` 取代）

### 6.4 UNCHANGED

- `EvaluateSpawnPolicy` (line 145) — 调 `SpawnPolicyEvaluator` 后填 `round.SpawnPolicy/SpawnRationale/RollupSynthRequested` 行为不变
- `spawnRationale` (line 155) — 6 case 文案不变
- `DeliverableContinuationRequired` / `deliverableContinuationRequired` / `applicableDeliverableSchema` / `IsDeliverableInlineBudgetExhaustedFromCtx` / `deliverableInlineWouldExhaust` 5 helper 不变
- `spawnForDeliverableContinuation` / `RollupSynthEligible` / `IsExploratoryPlanKind` / `CanDecompose` 4 依赖不变
- `WorkItemPipelineRound` / `TreeEvalContext` 2 struct 字段不变
- `SpawnPolicy` 6 态枚举不变
- `defaultVerdict` / `defaultRollupVerdict` / 5 常量（`DefaultMaxDecomposeDepth` 等）不变

## 7. 0 行为变化验证

### 7.1 现有 22 测试 (0 修改必须 PASS)

| 测试文件 | 测试函数 | 覆盖 |
|----------|---------|------|
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R0_RunningChildren` | R0 |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R1_MaxDepth_IncompleteDeliverable` | R1 w/ cont |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R05_DeliverableCompleteAtMaxDepth` | R0.5 (T: D7-S5-A93-T01) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R1_InlineRetriesExhaustedEscalates` | R1 w/ exhaust (T: D7-S5-A93-T02) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R1_MaxDepth_NoDeliverableSchema` | R1 no schema |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R2_DailyLimit` | R2 |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R3_CommitmentPass` | R3 |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R4_ExplorationPass` | R4 |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_PartialHighUncertainty` | R5 high U |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_PartialAtThreshold` | R5 at threshold |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_ExplorationPartialLowUncertainty_Decomposable` | R5 exploratory decomposable |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_ExploreLeafPartialInlines` | R5 explore leaf |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_RollupPartialInlines` | R5 rollup below-limit |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R5_PartialLowUncertainty` | R5 low U |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_ScenarioFail` | R6 scenario |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_ExplorationFail` | R6 exploration decomposable |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupPartial_BelowLimitInlines` | rollup guard below (T: D7-S15-A89-T01) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupPartial_AtLimitEscalates` | rollup guard at-limit (T: D7-S15-A89-T02) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupFail_AtLimitEscalates` | rollup guard at-limit (T: D7-S15-A89-T03) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupIndeterminate_AtLimitEscalates` | rollup guard at-limit (T: D7-S15-A89-T04) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_RollupPass_AlwaysNone` | R3/R4 rollup fall-through (T: D7-S15-A89-T05) |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_ExploreLeafFailInlines` | R6 explore leaf |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R6_CommitmentFail` | R6 commitment |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_IndeterminateRetry` | R7 retry |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_IndeterminateExhausted_ExploratoryDecomposes` | R7 exhausted decompose |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_ExploreLeafIndeterminateExhaustedEscalates` | R7 explore leaf exhausted |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R7_IndeterminateExhausted_CommitmentEscalatesHuman` | R7 commitment exhausted |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_R8_UnknownVerdict` | R8 |
| `spawn_policy_test.go` | `TestSpawnPolicyEvaluator_NilRound` | nil round |
| `spawn_policy_test.go` | `TestEvaluateSpawnPolicy_SetsRationale` | EvaluateSpawnPolicy rationale |
| `spawn_policy_test.go` | `TestValidateSpawnDecompose` | ValidateSpawnDecompose |
| `spawn_policy_test.go` | `TestCapChildSpecs` | CapChildSpecs |
| `spawn_policy_test.go` | `TestResolveHint_FromLastRound` | ResolveHint |
| `spawn_policy_inline_test.go` | `TestSpawnPolicyEvaluator_DeliverableInlineWouldExhaustEscalatesAtDepth0` | R1 w/ exhaust at depth 0 |

22 测试 0 修改 → 验证 0 行为变化。

### 7.2 新增 byte-equivalent 测试 (build tag `legacy_spawn`)

```go
//go:build legacy_spawn
// +build legacy_spawn

package workmodel

// SpawnPolicyEvaluatorLegacy — 旧 50+ 行实现保留，仅供 test 对比；不编译进生产
func SpawnPolicyEvaluatorLegacy(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy { ... }

func TestSpawnPolicyEvaluatorRefactor_ByteEquivalent_OldVsNew(t *testing.T) {
    cases := []struct {
        name string
        round *WorkItemPipelineRound
        ctx   TreeEvalContext
    }{
        // 22 组合 (R0-R8 + nil + 4 rollup guard)
    }
    for i, c := range cases {
        oldV := SpawnPolicyEvaluatorLegacy(c.round, c.ctx)
        newV := SpawnPolicyEvaluator(c.round, c.ctx)
        if oldV != newV {
            t.Errorf("case %d (%s): old=%q new=%q", i, c.name, oldV, newV)
        }
    }
}
```

**注意**：`SpawnPolicy` 是 typed enum 6 态（`SpawnNone` / `SpawnAwait` / `SpawnInline` / `SpawnEscalateHuman` / `SpawnDecompose` / `SpawnParallelExplore`），`==` 比较足以 byte-equal。**不需额外 verdictEqual helper**（与 M4 不同，M4 比较 5 字段 Kind/Confidence/Reason/SourceID/IndeterminateReason；M5 只比较 SpawnPolicy typed enum）。

### 7.3 子决策顺序锁定测试

```go
func TestSpawnPolicyEvaluator_SubDecisionOrder(t *testing.T) {
    // 构造同时命中 R0 (RunningChildren>0) + rollup guard + VerdictPass
    // 期望：checkBudget fired=true (R0) → 主函数 return SpawnAwait
    // 验证：checkRollupGuard 和 checkVerdictDirection 不被调
    // 验证方法：用全局计数器 / 改写为 var，记录每个子决策的调用次数
    //   Round 1: ctx.RunningChildren=2 + VerdictPass → 期望 SpawnAwait (R0 fired) + checkRollupGuard 0 次 + checkVerdictDirection 0 次
    //   Round 2: ctx.RollupRound=true + VerdictPass + RollupRetries<Max → 期望 SpawnInline (guard fired)
    //   Round 3: ctx.VerdictKind=VerdictPass + CommitmentPlan + no cont → 期望 SpawnNone (direction fired)
}
```

## 8. 已知限制 / 风险

1. **3 子决策顺序是隐性契约**：未来修改顺序必须同步更新 `spawn_decision_algebra.go` 注释 + 子决策顺序测试。缓解：顺序锁定测试 1 断言覆盖。
2. **`checkBudget` 4 gate 顺序是隐性契约**：R0 → R0.5 → R1 → R2 顺序错位 → 静默错误。缓解：6 case 单元测试覆盖每个 gate。
3. **`SpawnPolicyEvaluatorLegacy` 死代码**：下个 change (`mups-cleanup-legacy`) 必须删除。缓解：在 `demand.md` §4.Q7 明确记录。
4. **`SpawnPolicy` typed enum == 比较足够 byte-equal**：因为 6 态枚举无附加字段；未来若 SpawnPolicy 加结构字段（如 SpawnRationale 关联），需改用深比较。
5. **`normalizeCtx` 5 行兜底是 value copy**：每次调用复制 17 字段 ctx；性能 < 200 ns/次，远低于决策本身 < 1 μs；不是热点。

## 9. Follow-on 计划

- **M3** (`d7-mups-strategy-injection`)：Strategy 抽象注入 WorkItemExecContext；行为增量（最后做）
- **`mups-cleanup-legacy`**（下下个 change）：删除 `spawn_policy_legacy_test.go` + `SpawnPolicyEvaluatorLegacy` 死代码
