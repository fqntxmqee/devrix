# Design: MUPS Verify 节点决策表化 (M4)

**Change ID:** `mups-verify-table-driven`  
**Demand:** DM-20260705-005
**Status:** S3_Design
**Template:** `openspec/docs/methodology/detail-design-framework.md`（六段式 lite-mode）

## 1. 架构目标

### ① 业务目标

- **消除 trigger 散落**：12 trigger（nil / execute error / max_iters+tool / SideEffectStatus / user-gate / scope-only / deliverable contract / rollup child stats / empty summary / rollup contract）当前散落在 3 个 `verify*` 函数体中，无单一权威位置。
- **0 行为变化（M4）**：13 现有测试 + 7 byte-equivalent 测试全 PASS；trigger 顺序、置信度、Reason 文案、SourceID 全部 1:1 保留。
- **可读性 / 可维护性**：新增 trigger = 1 个新 `detectXxx` 函数 + 1 行表注册；trigger 顺序、置信度集中在 `VerdictTemplate`。

### ② 技术目标（量化指标）

| 指标 | 目标值 |
|------|--------|
| `verifyArtifact` 函数体行数 | ≤ 15（含表构造 + applyDecisionTable） |
| `verifyArtifactForWorkItemWithContract` 函数体行数 | ≤ 25（含 ctx 构造 + 表追加 + applyDecisionTable） |
| `verifyRollupArtifact` 函数体行数 | ≤ 15 |
| Detector 命名函数数量 | 12（11 artifact-level + 1 deliverable-incomplete；rollup 复用 3 个 + 加 3 个 rollup-specific） |
| 决策表 trigger 顺序 | 显式有序（顺序错位 → 静默错误） |
| 测试覆盖 | 13 现有 + 7 byte-equivalent + 12 detector 单元测试 + 1 detector 顺序锁定 |

### ③ 约束条件

- Go 1.22+（泛型 + struct tag）
- Pure 不可变 + `With*` builder（与 M1/M2 一致）
- 不修改 `workmodel.Verdict` 4 字段
- 不修改 `types.VerdictKind` 4 态枚举
- 不复活 ChannelRouter 4 个 channel 文件
- M4 阶段 0 行为变化；M3 阶段是行为增量（最后做）

## 2. 架构原则

### ① 设计原则

1. **Trigger 命名函数（Single Source of Truth）**：每个 trigger = 1 个 `detectXxx(art, ctx) bool` 命名函数 + 1 个 `VerdictTemplate` struct（Kind/Confidence/Reason/IndeterminateReason）；trigger 集中注册在 `VerifyDecisionTable` 数组。
2. **决策表是有序数组**：`applyDecisionTable(table, art, ctx) Verdict` 从头遍历，第一个 `fire==true` 的 trigger 应用其 template 返回；都不 fire → default verdict (Pass 0.9)。trigger 顺序是**显式声明**的契约。
3. **不可变 ctx**：`verifyContext` 是 build-time 构造的不可变 struct（含 art/item/pl/contract/stats/id），不修改 ctx；detector 只读 ctx。
4. **失败即 panic（设计 bug）**：决策表 trigger 重复注册 / detector 返回 verdict 但未设置 fire / table 为空 → compile-time panic (`var` 初始化失败)。
5. **0 行为变化优先（refactor before increment）**：M4 是 pure refactor；byte-equivalent 测试保证。

### ② 命名规范

- **detector 函数**：`detectXxx(art *wavescheduler.Artifact, ctx *verifyContext) bool`（无 verdict 返回，template 在表里绑定）
- **VerdictTemplate**：`{Kind types.VerdictKind, Confidence float64, Reason string, IndeterminateReason string, ReasonFunc func(art) string?}`
- **Trigger struct**：`{Name string, Fire func(*wavescheduler.Artifact, *verifyContext) bool, Template VerdictTemplate}`
- **决策表**：`type VerifyDecisionTable []VerifyTrigger`（有序 slice）
- **applyDecisionTable**：包级函数，输入 table + art + ctx → verdict

### ③ 代码风格

- 函数 < 50 行，文件 < 800 行
- `verify_decision_table.go` 目标 < 250 行（含 12 detector + table + apply + helpers + 注释）
- 不可变：`applyDecisionTable` 返回新 Verdict；不改 ctx
- 错误码：无（决策表是 build-time var，不运行时出错）

## 3. 业务流程

### ① 核心用例时序（verifyArtifact 走表）

```
[ItemPipeline] → verifyArtifact(art)
  ↓
  buildArtifactTable() VerifyDecisionTable  // 6 trigger 表
  ↓
  ctx = &verifyContext{art, id}
  ↓
  applyDecisionTable(table, art, ctx) Verdict
    for i, trigger := range table:
      if trigger.Fire(art, ctx):
        return buildVerdict(trigger.Template, art, ctx)
    return defaultVerdict  // Pass 0.9
```

### ② 决策表 3 类

#### (a) artifactDecisionTable（6 trigger，verifyArtifact 用）

| # | Trigger Name | Fire 条件 | VerdictTemplate |
|---|--------------|-----------|-----------------|
| 1 | `nil-artifact` | `art == nil` | Kind=Indeterminate, Confidence=0, Reason="missing artifact", IndeterminateReason="env_limited" |
| 2 | `execute-error` | `art.Error != ""` 或 `art.ExitCode != 0`（作为 gate；max_iters 由 trigger 3 细分） | （fire=true 但不返回；让 trigger 3/4 决定）— **实际不参与决策，只作为 gating** |
| 3 | `max-iters-partial` | trigger 2 fire && `Metadata["stop_reason"]=="max_iters"` && `Metadata["tool_calls"]>0` | Kind=Partial, Confidence=0.55, Reason="iteration cap with partial progress" |
| 4 | `execute-fail` | trigger 2 fire | Kind=Fail, Confidence=0.9, Reason="execute failed: %s" (art.Error) |
| 5 | `side-effect-rolledback` | `art.SideEffectStatus == SideEffectRolledBack` | Kind=Fail, Confidence=0.85, Reason="side effect rolled back" |
| 6 | `side-effect-uncertain` | `art.SideEffectStatus == SideEffectUnknown` 或 `SideEffectInflight` | Kind=Partial, Confidence=0.6, Reason="side effect uncertain" |
| default | — | 都不 fire | Kind=Pass, Confidence=0.9, Reason=art.Summary |

**注意**：trigger 2 (execute-error) 实际是 gating；它 fire=true，但**不返回 verdict** — 它的存在是为了让 trigger 3/4 在其 fire 之后做细分。`applyDecisionTable` 检测到 trigger 2 fire 后继续遍历 trigger 3，如果 trigger 3 也 fire 就返回 trigger 3 的 verdict；否则继续到 trigger 4。

但**决策表的"第一个 fire" 语义与 trigger 2 的 gating 角色冲突**。两种选择：
- **方案 1**：trigger 2 拆为 `execute-error-no-maxiters` (Error/ExitCode + !max_iters → Fail) 和 `max-iters-toolcalls` (max_iters + tool_calls >0 → Partial)；trigger 顺序：3 → 4
- **方案 2**：决策表允许"继续遍历"语义，trigger 2 fire 后不返回，继续下一个 trigger

**采用方案 1**（更清晰，避免"fire 但不返回"的反直觉）。trigger 2 重写为：
- `detectExecuteErrorNoMaxIters`：`Error != "" || ExitCode != 0`，且 `stop_reason != "max_iters"` 或 `tool_calls == 0` → Fire；Template: Fail(0.9, "execute failed: %s")
- `detectMaxItersPartial`：`Error != "" || ExitCode != 0` 且 `stop_reason == "max_iters"` 且 `tool_calls > 0` → Fire；Template: Partial(0.55, "iteration cap with partial progress")

顺序：`nil-artifact` → `max-iters-partial` → `execute-fail` → `side-effect-rolledback` → `side-effect-uncertain` → default Pass

#### (b) workItemDecisionTable（artifactDecisionTable + 3 overlay trigger，verifyArtifactForWorkItemWithContract 用）

| # | Trigger Name | Fire 条件 | VerdictTemplate |
|---|--------------|-----------|-----------------|
| 1-6 | 同 artifactDecisionTable | 同 | 同 |
| 7 | `user-gate` | `artifactAwaitingUserGate(art)` | Kind=Partial, Confidence=0.85, Reason="interactive user gate not allowed in pipeline execute" |
| 8 | `scope-only` | `workmodel.CanDecompose(item.Kind)` && `pl.Kind == plan.ExplorationPlan` && `isScopeOnlyDeliverable(art, item)` | Kind=Partial, Confidence=0.8, Reason="scope contract emitted without deliverable; decompose required" |
| 9 | `deliverable-incomplete` | `contract.ContractApplicable()` && `VerifyDeliverableContract(contract, art).Status == DeliverableStatusIncomplete` && **当前 verdict 是 Pass** | Kind=Partial, Confidence=0.65, Reason=deliverableReason(deliverable) |

**trigger 9 的特殊语义**：fire 条件包含"当前 verdict 是 Pass" — 因为 Pass → Partial 是 downgrade；Partial/Fail/Indeterminate 已经更严重，不需再降级。

**实现选择**：trigger 9 在 `applyDecisionTable` 之外处理（因为它需要看前序 trigger 的 verdict）。或者：决策表支持 `BuildOnPrevious bool` 字段。

**采用方案**：trigger 9 是 `detectDeliverableIncomplete` detector，返回 `(verdict, fire)` 元组，fire 条件包含"前序 verdict 是 Pass"；但前序 verdict 怎么知道？→ **简化方案**：把 trigger 9 单独处理（在 `verifyArtifactForWorkItemWithContract` 函数体里），不走 `applyDecisionTable`。理由：trigger 9 是**组合判定**（deliverable incomplete AND current verdict == Pass），不是简单 trigger；走表会引入"前序 verdict 上下文"，增加表复杂度。

**最终方案**：
- `verifyArtifact` 和 `verifyRollupArtifact`：完全走 `applyDecisionTable`（6 + 6 trigger 表）
- `verifyArtifactForWorkItemWithContract`：先调 `applyDecisionTable(artifactDecisionTable, art, ctx)` 拿 base verdict，然后叠加 3 overlay layer（user-gate / scope-only / deliverable-incomplete）— overlay layer 写成 3 个 `overlayXxx` 函数，每个 fire 就 modify verdict 并 return；最后一个 layer 改完直接返回（**不写表**，因为只有 3 个 overlay，重复利用 decision table 抽象成本 > 收益）

#### (c) rollupDecisionTable（6 trigger，verifyRollupArtifact 用）

| # | Trigger Name | Fire 条件 | VerdictTemplate |
|---|--------------|-----------|-----------------|
| 1 | `rollup-nil-or-failed` | `art == nil` 或 `art.Error != ""` 或 `art.ExitCode != 0` | （fire=true 但**不返回**，调 `verifyArtifact(art)` 返回其 verdict） |
| 2 | `rollup-all-failed` | `stats.Total > 0` && `stats.Failed == stats.Total` | Kind=Fail, Confidence=0.95, Reason="all %d rollup children failed; refusing Pass" (stats.Failed) |
| 3 | `rollup-mixed-running` | `stats.Total > 0` && `stats.Failed > 0` && `stats.Running > 0` | Kind=Partial, Confidence=0.8, Reason="rollup synthesized with %d failed + %d running children" (stats.Failed, stats.Running) |
| 4 | `rollup-empty-summary` | `strings.TrimSpace(art.Summary) == ""` | Kind=Fail, Confidence=0.9, Reason="rollup summary empty" |
| 5 | `rollup-contract-satisfied` | `workmodel.VerifyDeliverableContract(RollupDeliverableContract, summary, "").Status == DeliverableStatusComplete` | Kind=Pass, Confidence=0.9, Reason=summary |
| 6 | `rollup-contract-fail` | (catch-all 当 5 不 fire 且非 default) | Kind=Fail, Confidence=0.85, Reason="rollup deliverable contract not satisfied" |
| default | — | 都不 fire | Kind=Fail, Confidence=0.85, Reason="rollup deliverable contract not satisfied" |

**trigger 1 特殊**：fire=true 但不返回 → 委托给 `verifyArtifact(art)`（即走 artifactDecisionTable）。**实现**：fire 函数返回 false（不算 fire），但 `applyDecisionTable` 检测到 trigger 1 命中后**短路**：直接调 `verifyArtifact(art)` 返回。

**简化方案**：trigger 1 fire 函数返回 false，但作为表头 guard — 在 `applyDecisionTable` 之前手工 check：

```go
if art == nil || art.Error != "" || art.ExitCode != 0 {
    return verifyArtifact(art)
}
v := applyDecisionTable(rollupDecisionTable, art, ctx)
```

这样 `verifyRollupArtifact` 函数体就是 7 行：
```go
func verifyRollupArtifact(art *wavescheduler.Artifact, stats workmodel.ChildOutcomeStats) workmodel.Verdict {
    if art == nil || art.Error != "" || art.ExitCode != 0 {
        return verifyArtifact(art)
    }
    ctx := &verifyContext{art: art, stats: stats, id: art.TaskID}
    return applyDecisionTable(rollupDecisionTable, art, ctx)
}
```

## 4. 数据契约

### 4.1 `verifyContext` 不可变结构体

```go
type verifyContext struct {
    art      *wavescheduler.Artifact
    item     *workmodel.WorkItem          // 可选；workItemDecisionTable 用
    pl       *plan.Plan                   // 可选；workItemDecisionTable 用
    contract workmodel.DeliverableContract // 可选；workItemDecisionTable 用
    stats    workmodel.ChildOutcomeStats  // 可选；rollupDecisionTable 用
    id       string                       // art.TaskID 兜底 "artifact_unknown"
}
```

不可变：所有字段都是 value，调用方不能修改 ctx；detector 只读 ctx。

### 4.2 `VerdictTemplate` struct

```go
type VerdictTemplate struct {
    Kind                types.VerdictKind
    Confidence          float64
    Reason              string                       // 静态文案
    ReasonFunc          func(art *wavescheduler.Artifact) string  // 动态文案（注入 stats / Error）
    IndeterminateReason string                       // 可选
}
```

- `Reason` 优先；若 `ReasonFunc != nil` → 调 ReasonFunc 生成（用于"execute failed: %s" 之类动态文案）
- `IndeterminateReason` 仅在 `Kind == VerdictIndeterminate` 时有意义

### 4.3 `VerdictTrigger` struct

```go
type VerdictTrigger struct {
    Name     string
    Fire     func(art *wavescheduler.Artifact, ctx *verifyContext) bool
    Template VerdictTemplate
}
```

### 4.4 `VerifyDecisionTable` 类型

```go
type VerifyDecisionTable []VerdictTrigger
```

`applyDecisionTable` 顺序遍历，第一个 `Fire==true` 的 trigger 应用其 template 返回；都不 fire → 返回 `defaultVerdict(ctx)` (Pass 0.9)。

### 4.5 不可变性保证

- `verifyContext` 所有字段 value 类型，detector 不能修改 ctx（Go 编译期保证，因为 `ctx` 是 `*verifyContext` 指针但 detector 不持有 ctx 字段的指针）
- `VerdictTemplate` 是 value，传给 `buildVerdict` 复制一份
- `applyDecisionTable` 返回新 Verdict（走 `workmodel.Verdict` 现有 `With*` builder）

## 5. 关键算法

### 5.1 `applyDecisionTable` 伪代码

```go
func applyDecisionTable(table VerifyDecisionTable, art *wavescheduler.Artifact, ctx *verifyContext) workmodel.Verdict {
    for _, trigger := range table {
        if trigger.Fire(art, ctx) {
            v := buildVerdictFromTemplate(trigger.Template, art, ctx)
            return v.WithSourceID(ctx.id)  // 1:1 保留 SourceID 行为
        }
    }
    return defaultVerdict(ctx)
}

func buildVerdictFromTemplate(t VerdictTemplate, art *wavescheduler.Artifact, ctx *verifyContext) workmodel.Verdict {
    reason := t.Reason
    if t.ReasonFunc != nil {
        reason = t.ReasonFunc(art)
    }
    v := workmodel.Verdict{
        Kind:       t.Kind,
        Confidence: t.Confidence,
        Reason:     reason,
    }
    if t.IndeterminateReason != "" {
        v = v.WithIndeterminateReason(t.IndeterminateReason)
    }
    return v
}

func defaultVerdict(ctx *verifyContext) workmodel.Verdict {
    return workmodel.Verdict{
        Kind:       types.VerdictPass,
        Confidence: 0.9,
        Reason:     ctx.art.Summary,
    }.WithSourceID(ctx.id)
}
```

### 5.2 12 Detector 函数签名

```go
// artifactDecisionTable (6 detector)
func detectNilArtifact(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectMaxItersPartial(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectExecuteFail(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectSideEffectRolledBack(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectSideEffectUncertain(art *wavescheduler.Artifact, ctx *verifyContext) bool

// workItem overlay (3 detector, 仅 verifyArtifactForWorkItemWithContract 调)
func detectUserGate(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectScopeOnlyDeliverable(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectDeliverableIncomplete(art *wavescheduler.Artifact, ctx *verifyContext) bool

// rollupDecisionTable (3 detector + 1 delegation guard)
func detectRollupAllFailed(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectRollupMixedFailedRunning(art *wavescheduler.Artifact, ctx *verifyContext) bool
func detectRollupContractSatisfied(art *wavescheduler.Artifact, ctx *verifyContext) bool
// rollup-empty-summary 由 detectRollupContractSatisfied catch-all 隐式处理
// rollup nil/Error/ExitCode 由 verifyRollupArtifact 函数体手工 guard
```

### 5.3 决策表构造（包级 var）

```go
var artifactDecisionTable = VerifyDecisionTable{
    {
        Name: "nil-artifact",
        Fire: detectNilArtifact,
        Template: VerdictTemplate{
            Kind: types.VerdictIndeterminate, Confidence: 0,
            Reason: "missing artifact",
            IndeterminateReason: "env_limited",
        },
    },
    {
        Name: "max-iters-partial",
        Fire: detectMaxItersPartial,
        Template: VerdictTemplate{
            Kind: types.VerdictPartial, Confidence: 0.55,
            Reason: "iteration cap with partial progress",
        },
    },
    {
        Name: "execute-fail",
        Fire: detectExecuteFail,
        Template: VerdictTemplate{
            Kind: types.VerdictFail, Confidence: 0.9,
            ReasonFunc: func(art *wavescheduler.Artifact) string {
                return fmt.Sprintf("execute failed: %s", art.Error)
            },
        },
    },
    {
        Name: "side-effect-rolledback",
        Fire: detectSideEffectRolledBack,
        Template: VerdictTemplate{
            Kind: types.VerdictFail, Confidence: 0.85,
            Reason: "side effect rolled back",
        },
    },
    {
        Name: "side-effect-uncertain",
        Fire: detectSideEffectUncertain,
        Template: VerdictTemplate{
            Kind: types.VerdictPartial, Confidence: 0.6,
            Reason: "side effect uncertain",
        },
    },
}
```

```go
var rollupDecisionTable = VerifyDecisionTable{
    {
        Name: "all-failed",
        Fire: detectRollupAllFailed,
        Template: VerdictTemplate{
            Kind: types.VerdictFail, Confidence: 0.95,
            ReasonFunc: func(art *wavescheduler.Artifact) string {
                // 注：ctx.stats 需要在 buildVerdictFromTemplate 注入；这里 ReasonFunc 只接受 art
                // 改方案：buildVerdictFromTemplate(ctx) 也传 ctx
                return fmt.Sprintf("all %d rollup children failed; refusing Pass", ???)
            },
        },
    },
    ...
}
```

**注意**：`ReasonFunc(art)` 只能访问 `art`，不能访问 `ctx.stats`。方案 1：把 `ReasonFunc` 签名改为 `func(art, ctx)`；方案 2：把 stats 塞到 `art.Metadata` 临时字段（不优雅）。**采用方案 1**：`ReasonFunc func(art *wavescheduler.Artifact, ctx *verifyContext) string`。

### 5.4 verifyArtifactForWorkItemWithContract 的 3 overlay

不走 `applyDecisionTable`；保留为顺序 if 链（仅 3 层，比之前 4 层更清晰）：

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
            Verdict:             applyDecisionTable(artifactDecisionTable, art, &verifyContext{art: art, id: "artifact_unknown"}),
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

行数 ~30 行（含函数签名 + Deliverable 计算）。比现状 54 行减 44%。

## 6. 变更详情

### 6.1 ADDED

| 文件 | 用途 | 行数 |
|------|------|------|
| `internal/layers/orchestration/sessionorchestrator/verify_decision_table.go` | 决策表 kernel | ~250 |
| `internal/layers/orchestration/sessionorchestrator/verify_decision_table_test.go` | 12 detector 单测 + 3 verify 字节级 + 顺序锁定 | ~280 |
| `internal/layers/orchestration/sessionorchestrator/verify_legacy_test.go` | 旧实现保留 + build tag `legacy_verify` | ~80 |

### 6.2 MODIFIED

| 文件 | 变更 | 行数变化 |
|------|------|---------|
| `sessionorchestrator/item_verify.go` | `verifyArtifact` 49→15 行；`verifyArtifactForWorkItemWithContract` 54→30 行；移除内联 if/switch，调 `applyDecisionTable` + 3 overlay detector | -58 行 |
| `sessionorchestrator/rollup_verify.go` | `verifyRollupArtifact` 47→10 行；nil/Error/ExitCode guard 1 行 + applyDecisionTable 1 行 | -37 行 |

### 6.3 REMOVED

- 内联 if/switch 链（被 `applyDecisionTable` + detector 取代）
- **未移除** `artifactAwaitingUserGate` / `isScopeOnlyDeliverable` / `fileLineCitationRE` / `userGatePhrases` / `userGateToolRE` 5 个 helper（被 detector 调用）
- **未移除** `WithIndeterminateReason` / `WithSourceID` 现有 builder（被 `applyDecisionTable` / `buildVerdictFromTemplate` 调用）

## 7. 0 行为变化验证

### 7.1 现有 13 测试 (0 修改必须 PASS)

| 测试文件 | 测试函数 | 覆盖 |
|----------|---------|------|
| `item_verify_test.go` | `TestVerifyArtifact_MaxItersWithToolsIsPartial` | trigger 3 (max-iters-partial) |
| `item_verify_test.go` | `TestVerifyArtifact_MaxItersNoToolsIsFail` | trigger 4 (execute-fail) |
| `item_verify_test.go` | `TestVerifyArtifactForWorkItem_UserGateIsPartial` | overlay 7 (user-gate) |
| `item_verify_test.go` | `TestVerifyArtifactForWorkItem_ScopeOnlyIsPartial` | overlay 8 (scope-only) |
| `deliverable_verify_test.go` | `TestVerifyArtifactForWorkItemWithSchema_should_downgrade_pass_when_incomplete` | overlay 9 (deliverable-incomplete) |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_Pass` | rollup contract satisfied |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_TooShort` | rollup contract fail (catch-all) |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_PlanningDenylist` | rollup contract fail |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_PhantomToolCallMarkup` | rollup contract fail |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_AllChildrenFailed_RefusesPass` | rollup trigger 2 (all-failed) |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_FailedAndRunning_Partial` | rollup trigger 3 (mixed-running) |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_MixedFailure_Passes` | rollup trigger 5 (contract-satisfied) |
| `item_pipeline_rollup_test.go` | `TestVerifyRollupArtifact_NoChildren_LegacyShapeCheck` | rollup trigger 5 (no children) |

13 测试 0 修改 → 验证 0 行为变化。

### 7.2 新增 7 字节级 byte-equivalent 测试

```go
//go:build legacy_verify
// +build legacy_verify

package sessionorchestrator

import (...)

// verifyArtifactLegacy + verifyArtifactForWorkItemLegacy + verifyRollupArtifactLegacy
// 旧实现保留，仅供 test 对比；不编译进生产

func TestVerifyArtifactRefactor_ByteEquivalent_OldVsNew(t *testing.T) {
    cases := []*wavescheduler.Artifact{
        nil,
        {TaskID: "wi_1", Error: "x", Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3}},
        {TaskID: "wi_1", Error: "x", Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 0}},
        {TaskID: "wi_1", ExitCode: 1, Error: "boom"},
        {TaskID: "wi_1", SideEffectStatus: types.SideEffectRolledBack},
        {TaskID: "wi_1", SideEffectStatus: types.SideEffectInflight},
        {TaskID: "wi_1", Summary: "ok"},
    }
    for i, art := range cases {
        oldV := verifyArtifactLegacy(art)
        newV := verifyArtifact(art)
        if !verdictEqual(oldV, newV) {
            t.Errorf("case %d: old=%+v new=%+v", i, oldV, newV)
        }
    }
}

// 类似 TestVerifyArtifactForWorkItemWithContractRefactor_ByteEquivalent + TestVerifyRollupArtifactRefactor_ByteEquivalent
```

### 7.3 detector 顺序锁定测试

```go
func TestVerifyArtifact_DetectorOrder(t *testing.T) {
    // 调用顺序计数器
    var order []string
    // 构造一个特殊 artifact 同时命中 trigger 1+3+4
    art := &wavescheduler.Artifact{TaskID: "wi_1", Error: "x", Metadata: map[string]any{"stop_reason": "max_iters", "tool_calls": 3}}
    // 期望: order = ["nil-artifact", "max-iters-partial"] (trigger 3 fire → return; trigger 4 不被调)
    // ...
}
```

## 8. 已知限制 / 风险

1. **trigger 顺序是隐性契约**：未来修改 trigger 顺序必须同步更新 `applyDecisionTable` 注释 + detector 顺序测试。缓解：detector 顺序测试 11 断言。
2. **`ReasonFunc` 签名 `(art, ctx)` 不直观**：使用者必须知道 ctx 存在。缓解：detector 单元测试覆盖 ReasonFunc 输出。
3. **`verifyArtifactForWorkItemWithContract` 仍保留 3 overlay if 链**：因为 trigger 9 (deliverable incomplete) 是组合判定。缓解：3 overlay 改用 detector 函数（`detectUserGate` / `detectScopeOnlyDeliverable` / `detectDeliverableIncomplete`），与决策表 trigger 命名一致。
4. **`_legacy_test.go` 是死代码**：下个 change (`mups-cleanup-legacy`) 必须删除。缓解：在 `demand.md` §4.Q6 明确记录。
5. **rollup trigger 1 短路逻辑**：fire=false 但函数体提前 return — 容易让阅读者困惑。缓解：在 `verifyRollupArtifact` 函数体加注释 "rollup 的 art 错误路径由 verifyArtifact 接管（fire=false 表外 guard）"。

## 9. Follow-on 计划

- **M5** (`d7-spawn-decision-algebra`)：SpawnDecision R0-R8 嵌套 if 拆为 checkBudget/checkDirection/checkEscalation 3 个命名子决策；0 行为变化
- **M3** (`d7-mups-strategy-injection`)：Strategy 抽象注入 WorkItemExecContext；行为增量（最后做）
- **mups-cleanup-legacy**（下下个 change）：删除 `_legacy_test.go` + `verifyArtifactLegacy` 死代码
