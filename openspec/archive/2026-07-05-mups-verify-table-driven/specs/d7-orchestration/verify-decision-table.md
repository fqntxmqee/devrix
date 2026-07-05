# Delta: d7-orchestration — Verify decision-table kernel

**Change ID:** `mups-verify-table-driven`
**Demand:** DM-20260705-005
**Affects:** `internal/layers/orchestration/sessionorchestrator/verify_decision_table.go` (NEW), `internal/layers/orchestration/sessionorchestrator/{item_verify,rollup_verify}.go` (MOD)

## ADDED Requirements

### Requirement: `verifyContext` 不可变结构体

`sessionorchestrator` 包定义 `verifyContext` struct（包内未导出），含 6 字段：

```go
type verifyContext struct {
    art      *wavescheduler.Artifact
    item     *workmodel.WorkItem           // 可选
    pl       *plan.Plan                    // 可选
    contract workmodel.DeliverableContract // 可选
    stats    workmodel.ChildOutcomeStats   // 可选
    id       string                        // art.TaskID 兜底 "artifact_unknown"
}
```

不可变：所有字段是 value 类型；detector 函数只读 ctx，不修改 ctx。

#### Scenario: 构造 + 字段访问
- **GIVEN** `verifyContext{art: art, item: item, id: id}`
- **WHEN** detector 调 `ctx.item` / `ctx.pl` / `ctx.stats`
- **THEN** 字段值正确传递；ctx 本身不被修改

### Requirement: `VerdictTemplate` / `VerdictTrigger` / `VerifyDecisionTable` 3 struct

```go
type VerdictTemplate struct {
    Kind                types.VerdictKind
    Confidence          float64
    Reason              string                                          // 静态文案
    ReasonFunc          func(art *wavescheduler.Artifact, ctx *verifyContext) string  // 动态文案
    IndeterminateReason string                                          // 仅 Kind=Indeterminate 用
}

type VerdictTrigger struct {
    Name     string
    Fire     func(art *wavescheduler.Artifact, ctx *verifyContext) bool
    Template VerdictTemplate
}

type VerifyDecisionTable []VerdictTrigger  // 有序 slice
```

#### Scenario: Template 字段
- **GIVEN** `VerdictTemplate{Kind: VerdictFail, Confidence: 0.9, Reason: "execute failed: %s", ReasonFunc: func(art, ctx) string { return fmt.Sprintf("execute failed: %s", art.Error) }}`
- **WHEN** `buildVerdictFromTemplate` 调 template
- **THEN** ReasonFunc 优先于 Reason；返回 `fmt.Sprintf("execute failed: %s", art.Error)`

### Requirement: `applyDecisionTable` 顺序遍历 + default 兜底

`applyDecisionTable(table VerifyDecisionTable, art *wavescheduler.Artifact, ctx *verifyContext) workmodel.Verdict` 顺序遍历 table：

1. 对每个 `trigger`，调 `trigger.Fire(art, ctx)`
2. 第一个 `fire==true` 的 trigger：构造 verdict（`buildVerdictFromTemplate`） + `WithSourceID(ctx.id)` 返回
3. 所有 trigger 都不 fire：返回 default verdict (`Pass(0.9, art.Summary).WithSourceID(ctx.id)`)

#### Scenario: 第一个 fired trigger 返回
- **GIVEN** artifact 同时满足 trigger 2 (max-iters-partial) 和 trigger 3 (execute-fail)
- **WHEN** `applyDecisionTable(artifactDecisionTable, art, ctx)`
- **THEN** 返回 trigger 2 的 verdict（Partial 0.55），**不**调 trigger 3

#### Scenario: 都不 fire → default
- **GIVEN** artifact 是 happy path (`SideEffectStatus == ""`, `Summary = "ok"`)
- **WHEN** `applyDecisionTable(artifactDecisionTable, art, ctx)`
- **THEN** 返回 `Pass(0.9, "ok")` default verdict

### Requirement: 5 artifact detector

| # | Name | Fire 条件 | Template |
|---|------|-----------|----------|
| 1 | `detectNilArtifact` | `art == nil` | Indeterminate(0, "missing artifact", "env_limited") |
| 2 | `detectMaxItersPartial` | `Error != ""` 或 `ExitCode != 0`，且 `stop_reason=="max_iters"` 且 `tool_calls > 0` | Partial(0.55, "iteration cap with partial progress") |
| 3 | `detectExecuteFail` | `Error != ""` 或 `ExitCode != 0`（且非 max_iters+tool>0） | Fail(0.9, `ReasonFunc: "execute failed: %s"(art.Error)`) |
| 4 | `detectSideEffectRolledBack` | `SideEffectStatus == SideEffectRolledBack` | Fail(0.85, "side effect rolled back") |
| 5 | `detectSideEffectUncertain` | `SideEffectStatus == SideEffectUnknown` 或 `SideEffectInflight` | Partial(0.6, "side effect uncertain") |

#### Scenario: nil artifact
- **GIVEN** `art == nil`
- **WHEN** `detectNilArtifact(art, ctx)`
- **THEN** 返回 true；verdict 是 `Indeterminate(0, "missing artifact").WithIndeterminateReason("env_limited").WithSourceID(ctx.id)`

#### Scenario: max_iters + tool_calls
- **GIVEN** `art.Error="x"`, `Metadata: {"stop_reason":"max_iters","tool_calls":3}`
- **WHEN** `detectMaxItersPartial(art, ctx)`
- **THEN** 返回 true；Partial(0.55, "iteration cap with partial progress").WithSourceID(ctx.id)

### Requirement: 3 workItem overlay detector

| # | Name | Fire 条件 | Template |
|---|------|-----------|----------|
| 6 | `detectUserGate` | `artifactAwaitingUserGate(art)` | Partial(0.85, "interactive user gate not allowed in pipeline execute") |
| 7 | `detectScopeOnlyDeliverable` | `workmodel.CanDecompose(item.Kind)` && `pl.Kind == plan.ExplorationPlan` && `isScopeOnlyDeliverable(art, item)` | Partial(0.8, "scope contract emitted without deliverable; decompose required") |
| 8 | `detectDeliverableIncomplete` | `contract.ContractApplicable()` && `VerifyDeliverableContract(contract, art).Status == DeliverableStatusIncomplete` && **当前 verdict 是 Pass** | Partial(0.65, deliverableReason) |

**trigger 8 特殊语义**：fire 条件包含"当前 verdict 是 Pass" — Pass → Partial 是 downgrade；Partial/Fail/Indeterminate 已经更严重，不需再降级。

**实现**：`verifyArtifactForWorkItemWithContract` 函数体保留 3 overlay `if detectXxx(art, ctx) { v = ... }` 链（不走 `applyDecisionTable`），因为 trigger 8 是组合判定。

### Requirement: 3 rollup detector + 1 guard

| # | Name | Fire 条件 | Template |
|---|------|-----------|----------|
| 9 | `detectRollupAllFailed` | `stats.Total > 0` && `stats.Failed == stats.Total` | Fail(0.95, `ReasonFunc: "all %d rollup children failed; refusing Pass"(stats.Failed)`) |
| 10 | `detectRollupMixedFailedRunning` | `stats.Total > 0` && `stats.Failed > 0` && `stats.Running > 0` | Partial(0.8, `ReasonFunc: "rollup synthesized with %d failed + %d running children"(stats.Failed, stats.Running)`) |
| 11 | `detectRollupContractSatisfied` | `VerifyDeliverableContract(RollupDeliverableContract, summary, "").Status == DeliverableStatusComplete` | Pass(0.9, summary) |

**guard 1 (rollup nil/Error/ExitCode)**：`verifyRollupArtifact` 函数体顶部 `if art == nil || art.Error != "" || art.ExitCode != 0 { return verifyArtifact(art) }`；不走 decision table（与现状 if/switch 链 1:1 保留）。

**default (rollup)**：`applyDecisionTable` 都不 fire → Fail(0.85, "rollup deliverable contract not satisfied").WithSourceID(ctx.id)。这与现状 `verifyRollupArtifact` 的 default catch-all (`if got.Status != Complete` → Fail) 字节等价。

### Requirement: `verifyArtifact` 49→15 行

```go
func verifyArtifact(art *wavescheduler.Artifact) workmodel.Verdict {
    id := "artifact_unknown"
    if art != nil && art.TaskID != "" {
        id = art.TaskID
    }
    ctx := &verifyContext{art: art, id: id}
    return applyDecisionTable(artifactDecisionTable, art, ctx)
}
```

行数 5（含函数签名）。

#### Scenario: 0 行为变化
- **GIVEN** 7 artifact 组合（nil / max_iters+tool / max_iters+no_tool / exit=1+error / SideEffectRolledBack / SideEffectInflight / Pass-default）
- **WHEN** `verifyArtifact(art)` vs `verifyArtifactLegacy(art)`
- **THEN** 7/7 verdict 字节级等价 (Kind/Confidence/Reason/SourceID/IndeterminateReason 全等)

### Requirement: `verifyArtifactForWorkItemWithContract` 54→30 行

```go
func verifyArtifactForWorkItemWithContract(
    art *wavescheduler.Artifact,
    item *workmodel.WorkItem,
    pl *plan.Plan,
    contract workmodel.DeliverableContract,
) WorkItemVerifyOutcome {
    schema := workmodel.DeliverableSchemaNotApplicable
    if contract.ContractApplicable() {
        schema = workmodel.DeliverableSchema("legacy_contract")
    }
    if art == nil {
        return WorkItemVerifyOutcome{
            Verdict:             applyDecisionTable(artifactDecisionTable, nil, &verifyContext{id: "artifact_unknown"}),
            DeliverableContract: contract,
            DeliverableSchema:   schema,
        }
    }
    id := art.TaskID
    if id == "" {
        id = "artifact_unknown"
    }
    ctx := &verifyContext{art: art, item: item, pl: pl, contract: contract, id: id}
    v := applyDecisionTable(artifactDecisionTable, art, ctx)
    if detectUserGate(art, ctx) {
        v = workmodel.Verdict{Kind: types.VerdictPartial, Confidence: 0.85, Reason: "interactive user gate not allowed in pipeline execute", SourceID: id}
    }
    if detectScopeOnlyDeliverable(art, ctx) {
        v = workmodel.Verdict{Kind: types.VerdictPartial, Confidence: 0.8, Reason: "scope contract emitted without deliverable; decompose required", SourceID: id}
    }
    deliverable := VerifyDeliverableContract(contract, art)
    if contract.ContractApplicable() && deliverable.Status == workmodel.DeliverableStatusIncomplete && v.Kind == types.VerdictPass {
        v = workmodel.Verdict{Kind: types.VerdictPartial, Confidence: 0.65, Reason: deliverableReason(deliverable), SourceID: id}
    }
    return WorkItemVerifyOutcome{
        Verdict:             v,
        Deliverable:         deliverable,
        DeliverableContract: contract,
        DeliverableSchema:   schema,
    }
}
```

行数 ~28（含函数签名 + Deliverable 计算）。比现状 54 行减 48%。

#### Scenario: 0 行为变化
- **GIVEN** 4 overlay 组合 (user_gate / scope_only / deliverable_incomplete_with_pass / deliverable_incomplete_with_fail)
- **WHEN** `verifyArtifactForWorkItemWithContract(art, item, pl, contract)` vs legacy
- **THEN** 4/4 verdict 字节级等价

### Requirement: `verifyRollupArtifact` 47→10 行

```go
func verifyRollupArtifact(art *wavescheduler.Artifact, stats workmodel.ChildOutcomeStats) workmodel.Verdict {
    if art == nil || art.Error != "" || art.ExitCode != 0 {
        return verifyArtifact(art)
    }
    ctx := &verifyContext{art: art, stats: stats, id: art.TaskID}
    return applyDecisionTable(rollupDecisionTable, art, ctx)
}
```

行数 6（含函数签名）。

#### Scenario: 0 行为变化
- **GIVEN** 6 rollup 组合 (Pass / TooShort / PlanningDenylist / PhantomToolCallMarkup / AllChildrenFailed / FailedAndRunning / MixedFailure / NoChildren)
- **WHEN** `verifyRollupArtifact(art, stats)` vs `verifyRollupArtifactLegacy(art, stats)`
- **THEN** 8/8 verdict 字节级等价

## MODIFIED

| 文件 | 变更 |
|------|------|
| `internal/layers/orchestration/sessionorchestrator/item_verify.go` | `verifyArtifact` 49→5 行；`verifyArtifactForWorkItemWithContract` 54→28 行；移除内联 if/switch 链 |
| `internal/layers/orchestration/sessionorchestrator/rollup_verify.go` | `verifyRollupArtifact` 47→6 行；移除内联 if/switch 链 |

## ADDED

| 文件 | 用途 |
|------|------|
| `internal/layers/orchestration/sessionorchestrator/verify_decision_table.go` | 决策表 kernel（~250 行） |
| `internal/layers/orchestration/sessionorchestrator/verify_decision_table_test.go` | 12 detector 单测 + applyDecisionTable 测试 + detector 顺序断言（~280 行） |
| `internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go` | 旧 3 verify 函数保留 + build tag `legacy_verify`（~80 行） |

## REMOVED

- 内联 if/switch 链（被 `applyDecisionTable` + detector 取代）
- **未移除** `artifactAwaitingUserGate` / `isScopeOnlyDeliverable` / `fileLineCitationRE` / `userGatePhrases` / `userGateToolRE` 5 个 helper（被 detector 调用）
- **未移除** `WithIndeterminateReason` / `WithSourceID` 现有 builder（被 `applyDecisionTable` / `buildVerdictFromTemplate` 调用）

## Invariants

1. **trigger 顺序是隐性契约**：`applyDecisionTable` 顺序遍历 table；修改 trigger 顺序必须同步更新 `verify_decision_table.go` 注释 + detector 顺序测试。
2. **0 行为变化承诺**：13 现有测试 0 修改 PASS + 7 byte-equivalent 测试覆盖 7 artifact 组合 + 4 overlay 组合 + 6 rollup 组合。
3. **`ReasonFunc` 签名 `(art, ctx)` 不直观**：但因为只有 stats 注入需要 ctx，未来扩展保持一致。
4. **`_legacy_test.go` 是临时死代码**：下个 change (`mups-cleanup-legacy`) 必须删除。

## Test Points

| T ID | 描述 | L5 |
|------|------|-----|
| D7-S10-A101-T01 | 12 detector 命名函数 + 单测（fire true/false 1-2 case each） | L5-MUPS-VTD-01 |
| D7-S10-A101-T02 | `applyDecisionTable` 顺序遍历 + 第一个 fired 返回 + default 兜底 | L5-MUPS-VTD-02 |
| D7-S10-A101-T03 | `verifyArtifact` 7 组合字节级等价旧实现 | L5-MUPS-VTD-03 |
| D7-S10-A101-T04 | `verifyArtifactForWorkItemWithContract` 4 overlay 组合字节级等价 | L5-MUPS-VTD-04 |
| D7-S10-A101-T05 | `verifyRollupArtifact` 6 rollup 组合字节级等价 | L5-MUPS-VTD-05 |
| D7-S10-A101-T06 | 现有 13 测试 0 修改 PASS | L5-MUPS-VTD-06 |
| D7-S10-A101-T07 | detector 顺序锁定测试 11 detector 顺序断言 | L5-MUPS-VTD-07 |
