# Spec: d7-mups-strategy-injection — MUPS Strategy 抽象注入

**Change ID:** d7-mups-strategy-injection
**Demand ID:** DM-20260705-008
**Spec version:** v1.0
**Status:** s5_accepted
**Date:** 2026-07-05

---

## 目的 (Purpose)

MUPS Decide 节点引入 Strategy 抽象 (per-PlanKind behavior) 完成 5 节点重构总图 (M1-M5+cleanup)
行为增量最后一步。让 PlanKind (Commitment/Protocol/Scenario/Exploration) 路由策略从
mups/execute 内的隐式 ChannelRegistry 抽提为 workmodel 包 Strategy interface, 恢复
Phase 3 PR-C2 设计的 `ChannelRouter 4 PlanKind 路由` 的可观察性, 让 spawn policy /
plan proposer / verify 等下游节点能根据 PlanKind 显式选择 strategy.

## 契约 (Contract)

### C1 — Strategy interface (workmodel 包)

```go
type Strategy interface {
    // RouteChannel returns the channel name for the given PlanKind.
    RouteChannel(planKind plan.PlanKind) string

    // SpawnOverride returns a custom SpawnPolicy to override the
    // checkVerdictDirection 5-case default. The full round is passed so the
    // Strategy can inspect DeliverableSchema/DeliverableStatus (CC-1.4
    // deliverable continuation must take precedence over the per-PlanKind
    // terminal override for CommitmentPlan + Partial). Return ok=false to
    // fall through to the 5-case default.
    SpawnOverride(round *WorkItemPipelineRound) (SpawnPolicy, bool)

    // ShouldDecompose reports whether the plan kind supports child decomposition.
    ShouldDecompose(planKind plan.PlanKind) bool

    // IsReadOnly reports whether the plan kind has side effects.
    IsReadOnly(planKind plan.PlanKind) bool
}
```

### C2 — 4 PlanKind Strategy implementations

| Strategy | PlanKind | RouteChannel | ShouldDecompose | IsReadOnly | SpawnOverride (verdict → policy) |
|----------|----------|--------------|-----------------|------------|----------------------------------|
| commitmentStrategy | CommitmentPlan | "commit_channel" | false | true | Fail/Partial (no deliverable) → SpawnNone |
| protocolStrategy | ProtocolPlan | "protocol_channel" | true | true | (none, safe default) |
| scenarioStrategy | ScenarioPlan | "scenario_channel" | false | false | Fail → SpawnNone |
| explorationStrategy | ExplorationPlan | "exploration_channel" | true | true | Pass → SpawnDecompose |

### C3 — DefaultStrategy registry

```go
var defaultStrategies = map[plan.PlanKind]Strategy{
    plan.CommitmentPlan:  commitmentStrategy{},
    plan.ProtocolPlan:    protocolStrategy{},
    plan.ScenarioPlan:    scenarioStrategy{},
    plan.ExplorationPlan: explorationStrategy{},
}

func LookupStrategy(planKind plan.PlanKind) Strategy
func RegisterStrategy(planKind plan.PlanKind, s Strategy)
```

- `LookupStrategy(planKind)` — returns bound Strategy or `protocolStrategy{}` (safe default)
- `RegisterStrategy(planKind, s)` — extension point for tests / future PlanKinds
- `init()` validates exactly 4 PlanKind bindings (panics otherwise)

### C4 — WorkItemExecContext integration

```go
type WorkItemExecContext struct {
    // ... existing 7 fields ...
    Strategy workmodel.Strategy
}

func WithWorkItemExecContext(ctx context.Context, ec WorkItemExecContext) context.Context {
    if ec.Strategy == nil {
        ec.Strategy = workmodel.LookupStrategy(plan.KindUnset) // protocolStrategy
    }
    return context.WithValue(ctx, workItemExecCtxKey{}, ec)
}
```

### C5 — spawn_decision_algebra integration

```go
func checkVerdictDirection(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
    // ... switch computes default policy ...
    if p, ok := LookupStrategy(round.PlanKind).SpawnOverride(round); ok {
        return p
    }
    return policy
}
```

## 行为矩阵 (Behavior Matrix)

4 PlanKind × 5 VerdictKind = 20 combinations:

| PlanKind | Verdict | 5-case default | M3 result | Change |
|----------|---------|----------------|-----------|--------|
| CommitmentPlan | Pass | SpawnNone | SpawnNone | no-op |
| CommitmentPlan | Fail | SpawnNone | SpawnNone | no-op (locked) |
| CommitmentPlan | Partial (low U, no deliv) | SpawnNone | SpawnNone | no-op (locked) |
| CommitmentPlan | Partial (high U) | SpawnDecompose | SpawnNone | **M3** |
| CommitmentPlan | Partial (incomplete deliv) | spawnForDelivCont | spawnForDelivCont | CC-1.4 wins |
| CommitmentPlan | Indeterminate | SpawnInline | SpawnInline | no-op |
| CommitmentPlan | Unknown | SpawnNone | SpawnNone | no-op |
| ProtocolPlan | (all 5) | varies | same as default | 0 change (safe default) |
| ScenarioPlan | Pass | SpawnNone | SpawnNone | no-op |
| ScenarioPlan | Fail | SpawnParallelExplore | SpawnNone | **M3** |
| ScenarioPlan | Partial | varies | varies (R5 default) | 0 change |
| ScenarioPlan | Indeterminate | SpawnInline | SpawnInline | no-op |
| ScenarioPlan | Unknown | SpawnNone | SpawnNone | no-op |
| ExplorationPlan | Pass | SpawnNone | SpawnDecompose | **M3** |
| ExplorationPlan | Fail | SpawnDecompose/Inline | same | 0 change |
| ExplorationPlan | Partial | varies | varies (R5 exploratory) | 0 change |
| ExplorationPlan | Indeterminate | SpawnInline | SpawnInline | no-op |
| ExplorationPlan | Unknown | SpawnNone | SpawnNone | no-op |

**M3 行为增量 (4/20)**: Commitment+Partial (high U), Scenario+Fail, Exploration+Pass,
and the no-op Commitment+Fail (locked terminal). All other 16 combinations maintain
the 5-case default behavior.

## 约束 (Invariants)

- **I1** — L1 (mups/execute) MUST NOT depend on workmodel package (no cycle).
  Bridge is `WorkItemExecContext.Strategy` (sessionorchestrator package).
- **I2** — `protocolStrategy` is the safe default for unknown PlanKinds. LookupStrategy
  returns protocolStrategy for KindUnset / unknown values.
- **I3** — 4 PlanKind Strategy implementations MUST be 1:1 bound in defaultStrategies
  at init() time. `len(defaultStrategies) != 4` → panic.
- **I4** — CC-1.4 deliverable continuation takes precedence over commitment terminal
  override. `commitmentStrategy.SpawnOverride` returns `ok=false` when
  `deliverableContinuationRequired(round)`.
- **I5** — `SpawnOverride` MUST NOT mutate round or ctx (pure function, value semantics).

## T 层测试点 (Test Points)

T 点编号遵循 DSAFT 标准 (D{X}-S{X}-A{XX}-T{XX}):

| T ID | 测试 | 文件 |
|------|------|------|
| D7-SX-AXX-T01-T14 | Strategy.SpawnOverride 4×5 + 兜底 | workmodel/strategy_test.go |
| D7-SX-AXX-T15-T19 | DefaultStrategy registry + LookupStrategy | workmodel/strategy_default_test.go |
| D7-SX-AXX-T20-T23 | M3 4 PlanKind × 5 Verdict 集成 | workmodel/spawn_decision_algebra_test.go |
| D7-SX-AXX-T24 | M3 行为增量 summary (4 cases) | workmodel/spawn_decision_algebra_test.go |
| D7-SX-AXX-T25 | M3 行为增量对齐 (R4/R6) | workmodel/spawn_policy_test.go |

## 依赖 (Dependencies)

- **Prerequisite**: M1 (Observe go-struct 化), M2 (Plan go-struct 化), M4 (Verify 决策表化),
  M5 (SpawnDecision 3 子决策代数化), cleanup (_legacy_test.go 死代码清理) — all S7_archived.
- **Depends on**: workmodel.SpawnPolicy type, plan.PlanKind type, types.VerdictKind type,
  WorkItemPipelineRound type.
- **Depended on by**: (future) plan proposer (Strategy.SpawnOverride for budget planning),
  (future) verify strategy (Strategy.IsReadOnly for verify scope), (future) channel router
  (Strategy.RouteChannel for PlanKind-aware channel selection).

## 演进路径 (Evolution Path)

- **E1** — Add 5th PlanKind (e.g., DelegationPlan): implement `delegationStrategy{}`,
  bind in `defaultStrategies` registry.
- **E2** — Plan proposer consumes `Strategy.SpawnOverride` for budget planning
  (predict spawn cost from round context).
- **E3** — Verify uses `Strategy.IsReadOnly` to scope verify contract
  (read-only plans → lighter verify).

## 变更日志 (Change Log)

- **v1.0** (2026-07-05) — M3 实施 (DM-20260705-008). 19 NEW tests + 5 MODIFIED files.
  Interface signature evolution: `(planKind, verdictKind)` → `(round)` for CC-1.4
  deliverable continuation precedence.
