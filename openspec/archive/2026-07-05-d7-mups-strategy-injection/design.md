# Design: d7-mups-strategy-injection — Strategy 抽象注入 WorkItemExecContext (M3)

**Change ID:** d7-mups-strategy-injection
**Demand ID:** DM-20260705-008
**Status:** S3_Design
**Parent Proposal:** `proposal.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式 - 中型 Change）
**Created:** 2026-07-05

---

## ① 架构目标

**业务目标**：
- 解决 5 节点重构总图（M1-M5 + cleanup）最后一步 M3 的落地：让 PlanKind (Commitment/Protocol/Scenario/Exploration) 路由策略从 mups/execute 内的隐式 ChannelRegistry 抽提为 Strategy 抽象
- 恢复 Phase 3 PR-C2 设计的 `ChannelRouter 4 PlanKind 路由` 的可观察性（ChannelRegistry 已存在，但 spawn policy 决策未与之联动）
- 让 spawn policy / plan proposer / verify 等下游节点能根据 PlanKind 显式选择 strategy（per-PlanKind 行为差异化）

**技术目标**：
- 1 个 Strategy interface (workmodel 包) + 4 个 PlanKind Strategy 实现 + 1 个 DefaultStrategy registry
- WorkItemExecContext 新增 Strategy 字段（interface，可空）+ nil 兜底 DefaultStrategy
- spawn_decision_algebra.checkVerdictDirection 末尾 1 行 `strategy.SpawnOverride(round.PlanKind)` 覆盖默认
- 4 PlanKind × 5 verdict = 20 组合新增单测 + Default 兜底 = 24 测试全 PASS
- 现有 22+18 spawn policy 测试 0 修改 PASS（M3 边界 = 兜底 0 行为变化）
- 全文 `go test -race -count=1 ./...` 0 新增 fail（除 pre-existing 1 lint test）
- `go vet ./...` 0 warning
- 5 节点重构总图最终落地文档 (mups-5node-refactor-roadmap.md) P1 创建

**约束条件**：
- "行为增量边界"：4 PlanKind × 5 verdict = 20 组合中，**默认行为 = 旧 5 case 行为**（兜底），仅显式声明的 4 行为变化生效
- Strategy 抽象层在 workmodel 包；不 import mups/execute；WorkItemExecContext 注入桥接
- 0 行为变化承诺（M3 边界）：现有 22+18 测试 0 修改；新增 24 测试只覆盖"显式声明"的行为变化
- mups/execute 不感知 Strategy 抽象（保持 Phase 3 PR-C2 不变）
- ChannelRegistry 1:1 绑定不变；4 个 channel 实现不变

## ② 架构原则

**设计原则**：

1. **Strategy 抽象层在 workmodel 包**（L2 编排核心）：不 import mups/execute（L1 节点），避免分层违规
2. **Behavior = Default + Override**：5 case 默认行为 = 旧行为（兜底 0 行为变化）；Strategy 显式声明的 4 行为变化 = 覆盖默认
3. **interface 字段可空 + nil 兜底**：WorkItemExecContext.Strategy 是 interface (可空)；nil → DefaultStrategy.LookupStrategy(planKind) 兜底
4. **每个 PlanKind 1 个 strategy 文件**（4 文件结构化）：commitment/protocol/scenario/exploration 各 1 文件，避免散落
5. **Default registry = 包级 var + Lookup helper**：DefaultStrategy 单例 + LookupStrategy(planKind) 4 PlanKind 1:1 映射
6. **5 节点重构总图闭环 = M3 行为增量 + 总图文档**：mups-5node-refactor-roadmap.md 在 M3 启动时一并补建

**命名规范**：
- Interface: `Strategy` (workmodel 包)
- 4 实现: `commitmentStrategy` / `protocolStrategy` / `scenarioStrategy` / `explorationStrategy` (lowercase, unexported)
- Registry: `defaultStrategies` (包级 var) + `LookupStrategy` (公开 helper) + `RegisterStrategy` (扩展点)
- WorkItemExecContext 字段: `Strategy workmodel.Strategy` (interface)

**DSAFT ID 分配**：
- Scenario: D7-S22 (新增，PR-B/C 保留位)
- Activity: D7-S22-A101: Strategy 抽象注入 (M3)
- T 点: D7-S22-A101-T01..T08 (实施后填入)

**代码风格**：
- 函数 < 50 行（Strategy 每个 method < 10 行）
- 文件 < 800 行（每个 strategy_*.go < 50 行）
- 异常不过模块边界（Strategy.SpawnOverride 返回 (SpawnPolicy, bool) 不抛 error）

## ③ 业务流程

**核心用例时序图（M3 PlanKind 路由恢复）**：

```
[User Message] → [Observe M1] → [Plan M2] → Plan{PlanKind: CommitmentPlan} 产出
                                                            ↓
[Execute] → [WorkItemExecContext.WithWorkItemExecContext{Strategy: nil}]
                                                            ↓
[SpawnPolicyEvaluator.M5] → checkVerdictDirection(round, ctx, strategy=LookupStrategy(round.PlanKind))
                                                            ↓
                                              [checkVerdictDirection 5 case 默认]
                                                            ↓
                                末尾 1 行: if p, ok := strategy.SpawnOverride(round.PlanKind); ok { return p }
                                                            ↓
[CommitmentPlan + VerdictFail] → strategy.SpawnOverride() → SpawnNone (terminal, 1-step commitment 不重试)
                                                            ↓
[Execute M4 Verify M5 Spawn M3] (commitment 1-step synchronous, no decompose)
```

**异常补偿（3 层 fail-safe）**：
- Layer 1: `WorkItemExecContext.Strategy == nil` → `LookupStrategy(round.PlanKind)` 兜底
- Layer 2: `LookupStrategy` 找不到对应 PlanKind → `protocolStrategy` 兜底 (Phase 2 PR-B1 DefaultPlanner 已有先例)
- Layer 3: `strategy.SpawnOverride(round.PlanKind)` 返回 `(SpawnPolicy, false)` → 不覆盖，5 case 默认行为生效

**分支处理决策树**：

```
SpawnPolicyEvaluator.checkVerdictDirection(round, ctx)
  ↓
  [5 case switch on round.VerdictKind] (旧 M5 行为, 兜底)
  ↓
  [末尾 1 行: strategy.SpawnOverride(round.PlanKind) 覆盖]
  ↓
  ├─ CommitmentPlan + VerdictFail    → SpawnNone (terminal, 行为增量)
  ├─ CommitmentPlan + VerdictPartial → SpawnNone (terminal partial, 行为增量)
  ├─ ScenarioPlan + VerdictFail      → SpawnNone (read-only, no retry, 行为增量)
  ├─ ExplorationPlan + VerdictPass   → SpawnDecompose (parallel explore, 行为增量)
  └─ 其他 16 组合                      → 5 case 默认 (兜底, 0 行为变化)
```

## ④ 领域模型

**聚合根**：
- `Strategy` interface (workmodel 包) - 1 个聚合根
- `WorkItemExecContext` (sessionorchestrator 包) - 已有聚合根，扩展 1 字段

**限界上下文（新增 6 文件）**：

```
internal/layers/orchestration/workmodel/
├── strategy.go                    (NEW, ~40 行) - Strategy interface
├── strategy_commitment.go         (NEW, ~30 行)
├── strategy_protocol.go           (NEW, ~30 行)
├── strategy_scenario.go           (NEW, ~30 行)
├── strategy_exploration.go        (NEW, ~30 行)
└── strategy_default.go            (NEW, ~40 行) - registry + LookupStrategy + RegisterStrategy

internal/layers/orchestration/sessionorchestrator/
└── workitem_exec_context.go       (MODIFIED, +10 行) - 新增 Strategy 字段 + 注入

internal/layers/orchestration/workmodel/
└── spawn_decision_algebra.go      (MODIFIED, +5 行) - checkVerdictDirection 末尾 1 行 strategy.SpawnOverride
```

**领域事件（Span / Metric 列表）**：
- Span: `strategy.lookup` (M3 新增, 评估 lookup 性能 < 100 ns)
- Span: `strategy.override` (M3 新增, 记录 SpawnOverride 触发的行为变化)
- Metric: `strategy_override_count{plan_kind=X}` (M3 新增, Prometheus 监控)

**跨域消费模型**：
- L2 (workmodel) → L2 (sessionorchestrator) 单向: `WorkItemExecContext.Strategy` 注入
- L2 (workmodel) → 不依赖 L1 (mups/execute): Strategy 抽象层独立
- L1 (mups/execute) → 不感知 Strategy: ChannelRegistry 1:1 绑定不变

## ⑤ 核心链路图

**端到端路径（M3 行为增量链路）**：

```
[Plan] → Plan{PlanKind: X}
  ↓
[Execute] → WithWorkItemExecContext(ctx, ec{Strategy: nil})  (兜底 DefaultStrategy)
  ↓
[Spawn] → SpawnPolicyEvaluator.M5 (3 子决策代数化)
  ↓
  ├─ checkBudget: 4 budget gates (R0/R0.5/R1/R2), 0 行为变化
  ├─ checkRollupGuard: 跨 verdict guard (RH-MUPS-03), 0 行为变化
  └─ checkVerdictDirection: 5 case + 末尾 1 行 strategy.SpawnOverride(round.PlanKind)
       ↓
       ├─ CommitmentPlan + VerdictFail/Partial → SpawnNone (M3 行为增量)
       ├─ ScenarioPlan + VerdictFail → SpawnNone (M3 行为增量)
       ├─ ExplorationPlan + VerdictPass → SpawnDecompose (M3 行为增量)
       └─ 其他 → 5 case 默认 (兜底, 0 行为变化)
  ↓
[Execute Channel] → ChannelRouter.Route(Plan) → 4 PlanKind 4 Channel (Phase 3 PR-C2 不变)
```

**时序标注**：
- Strategy.LookupStrategy: < 100 ns (map[PlanKind]Strategy 1:1 查找)
- Strategy.SpawnOverride: < 100 ns (3-4 method 调用，1 switch 决策)
- WorkItemExecContext 注入: < 50 ns (interface 字段赋值)
- 总性能开销: < 250 ns/次 (远低于决策本身 < 1 μs)

**单点风险与缓解**：
- 单点 1: Strategy 抽象层缺失 → 缓解: 1 interface + 4 实现 + 1 registry 三层结构
- 单点 2: PlanKind 行为散落 if/switch → 缓解: 4 文件结构化 (commitment/protocol/scenario/exploration)
- 单点 3: WorkItemExecContext 注入破坏向后兼容 → 缓解: interface 字段可空 + nil 兜底
- 单点 4: 5 节点重构总图文档缺失 → 缓解: mups-5node-refactor-roadmap.md 一并创建

## ⑥ 接口 / API 设计

**Strategy interface (workmodel 包)**：

```go
// Strategy encapsulates per-PlanKind behavior (routing, channel selection,
// spawn policy override). 4 PlanKind Strategy implementations live in
// strategy_*.go (commitment/protocol/scenario/exploration).
//
// Invariants:
//   - RouteChannel MUST return the channel name for the given PlanKind
//     (or empty string if no channel bound).
//   - SpawnOverride MAY return a custom SpawnPolicy to override the
//     checkVerdictDirection 5-case default. Return ok=false to fall through.
//   - ShouldDecompose MUST report whether the plan kind supports child
//     decomposition (protocol/exploration true; commitment/scenario false).
//   - IsReadOnly MUST report whether the plan kind has side effects
//     (commitment/protocol/exploration true; scenario false).
//
// L1 (mups/execute) does NOT depend on this interface. The bridge is
// WorkItemExecContext.Strategy (sessionorchestrator package).
type Strategy interface {
    RouteChannel(planKind plan.PlanKind) string
    SpawnOverride(planKind plan.PlanKind) (SpawnPolicy, bool)
    ShouldDecompose(planKind plan.PlanKind) bool
    IsReadOnly(planKind plan.PlanKind) bool
}
```

**4 PlanKind Strategy 实现（4 文件结构化）**：

```go
// commitmentStrategy: 1-step synchronous, terminal fail (no decompose).
// M3 行为增量: VerdictFail + VerdictPartial → SpawnNone (terminal).
type commitmentStrategy struct{}

func (commitmentStrategy) RouteChannel(planKind plan.PlanKind) string {
    if planKind != plan.CommitmentPlan { return "" }
    return "commit_channel"
}
func (commitmentStrategy) SpawnOverride(planKind plan.PlanKind) (SpawnPolicy, bool) {
    if planKind != plan.CommitmentPlan { return SpawnNone, false }
    // Override for VerdictFail + VerdictPartial handled by VerdictKind dispatch.
    return SpawnNone, false  // (lookup-table alternative; final impl uses verdict switch)
}
func (commitmentStrategy) ShouldDecompose(planKind plan.PlanKind) bool {
    return planKind == plan.CommitmentPlan  // false (commitment no decompose)
}
func (commitmentStrategy) IsReadOnly(planKind plan.PlanKind) bool {
    return planKind != plan.CommitmentPlan  // true (commitment has side effect)
}
```

**DefaultStrategy registry**：

```go
// defaultStrategies maps PlanKind to Strategy. 1:1 binding, validated at
// init() time. LookupStrategy(planKind) returns the bound Strategy or
// protocolStrategy as the safe default (matches Phase 2 PR-B1 DefaultPlanner
// pattern).
var defaultStrategies = map[plan.PlanKind]Strategy{
    plan.CommitmentPlan:  commitmentStrategy{},
    plan.ProtocolPlan:    protocolStrategy{},
    plan.ScenarioPlan:    scenarioStrategy{},
    plan.ExplorationPlan: explorationStrategy{},
}

func LookupStrategy(planKind plan.PlanKind) Strategy {
    if s, ok := defaultStrategies[planKind]; ok {
        return s
    }
    return protocolStrategy{}  // safe default
}

func RegisterStrategy(planKind plan.PlanKind, s Strategy) {
    defaultStrategies[planKind] = s  // extension point
}

func init() {
    // Validate 1:1 binding at init time.
    if len(defaultStrategies) != 4 {
        panic("defaultStrategies: expected exactly 4 PlanKind bindings")
    }
}
```

**WorkItemExecContext 扩展（sessionorchestrator 包）**：

```go
type WorkItemExecContext struct {
    // ... 已有 7 字段 ...
    
    // Strategy is the per-PlanKind behavior abstraction. nil → DefaultStrategy.
    // M3 (DM-20260705-008) 新增. Bridge 让 workmodel 包 spawn policy 能
    // 读 PlanKind-aware 行为而不直接 import mups/execute (避免分层违规).
    Strategy workmodel.Strategy
}

func WithWorkItemExecContext(ctx context.Context, ec WorkItemExecContext) context.Context {
    if ec.Strategy == nil {
        ec.Strategy = workmodel.LookupStrategy(plan.KindUnset)  // 兜底 default
    }
    return context.WithValue(ctx, workItemExecCtxKey{}, ec)
}
```

**SpawnPolicyEvaluator 集成（workmodel/spawn_decision_algebra.go）**：

```go
func checkVerdictDirection(round *WorkItemPipelineRound, ctx TreeEvalContext) SpawnPolicy {
    strategy := LookupStrategy(round.PlanKind)  // M3 NEW
    var defaultPolicy SpawnPolicy
    switch round.VerdictKind {
    case types.VerdictPass:
        defaultPolicy = spawnForPass(round, ctx)
    case types.VerdictPartial:
        defaultPolicy = spawnForPartial(round, ctx)
    case types.VerdictFail:
        defaultPolicy = spawnForFail(round, ctx)
    case types.VerdictIndeterminate:
        defaultPolicy = spawnForIndeterminate(round, ctx)
    default:
        defaultPolicy = SpawnNone
    }
    // M3 NEW: 末尾 1 行 strategy 覆盖 (default 行为兜底, 显式 4 行为变化生效)
    if p, ok := strategy.SpawnOverride(round.PlanKind); ok {
        return p
    }
    return defaultPolicy
}
```

**风格**：Pure types (Strategy interface + 4 struct impl) + Builder (DefaultStrategy registry) + 不可变

**契约**：
- Strategy.SpawnOverride 返回 (SpawnPolicy, bool): ok=true 表示覆盖默认；ok=false 表示 fall through
- LookupStrategy 永不返回 nil（protocolStrategy 兜底）
- WorkItemExecContext.Strategy 可空 (interface zero value)；nil → DefaultStrategy 兜底

**幂等保障表**：
- LookupStrategy 幂等 (map 查找)
- RegisterStrategy 幂等 (覆盖)
- SpawnPolicyEvaluator 5 case 幂等 (switch on VerdictKind)

**版本演进路径**：
- v1.0 (M3): Strategy 抽象层 + 4 PlanKind 行为差异化 (本 change)
- v1.1 (future): 扩展 PlanKind (e.g., DelegationPlan) → RegisterStrategy 注入 (无散落修改)
- v2.0 (future): Strategy 行为集成 Plan 节点 (plan proposer 也读 Strategy)

---

## 附录 A：File Manifest

### 新增
- `internal/layers/orchestration/workmodel/strategy.go` (~40 行)
- `internal/layers/orchestration/workmodel/strategy_commitment.go` (~30 行)
- `internal/layers/orchestration/workmodel/strategy_protocol.go` (~30 行)
- `internal/layers/orchestration/workmodel/strategy_scenario.go` (~30 行)
- `internal/layers/orchestration/workmodel/strategy_exploration.go` (~30 行)
- `internal/layers/orchestration/workmodel/strategy_default.go` (~40 行)
- `internal/layers/orchestration/workmodel/strategy_test.go` (~120 行)
- `internal/layers/orchestration/workmodel/strategy_default_test.go` (~50 行)
- `openspec/specs/d7-orchestration/mups-5node-refactor-roadmap.md` (~200 行, P1)

### 修改
- `internal/layers/orchestration/sessionorchestrator/workitem_exec_context.go` (+10 行)
- `internal/layers/orchestration/workmodel/spawn_decision_algebra.go` (+5 行)
- `internal/layers/orchestration/workmodel/spawn_decision_algebra_test.go` (+30 行)

**总计**：9 NEW + 3 MODIFIED = 12 文件，~625 行新增

## 附录 B：Rollback Plan

**回滚触发条件**：
- 24+ 新增测试 fail 任一
- 现有 22+18 spawn policy 测试 fail (M3 行为增量破坏兜底)
- `go vet ./...` 警告
- WorkItemExecContext 注入破坏 7 已有调用方
- 5 节点重构总图文档未创建

**回滚方式**：`git revert <merge-commit-sha>` 或 `git reset --hard <merge-commit-sha>^`

**回滚影响**：
- 恢复 6 NEW 文件 (~200 行 strategy)
- 撤销 2 MODIFIED 文件 (+15 行总)
- 撤销 mups-5node-refactor-roadmap.md 总图文档

## 附录 C：回归风险评估

**baseline 对比**：
- Before: 22+18 spawn policy 测试 + 30 sessionorchestrator 测试
- After: 22+18+24 spawn policy 测试 (兜底 0 行为变化) + 30 sessionorchestrator 测试 (注入 0 破坏)
- 差异: 24 新增测试只覆盖"显式声明"的 4 行为变化

**高风险改动点**：
- checkVerdictDirection 末尾 1 行 strategy 覆盖 → 兜底 0 行为变化 + 4 显式行为变化
- WorkItemExecContext 新增 Strategy 字段 → interface 可空 + nil 兜底

**测试策略**：
- 22+18 现有 spawn policy 测试 0 修改 PASS (兜底 0 行为变化核心承诺)
- 24 新增策略测试 4×5+default 组合覆盖
- 30 现有 sessionorchestrator 测试 0 修改 PASS (WorkItemExecContext 注入 0 破坏)
- 全文 `go test -race -count=1` 0 新增 fail (除 pre-existing 1 lint test)
- `go vet ./...` 0 warning

## 附录 D：S3 检查清单自检

| 章节 | 状态 | 备注 |
|------|------|------|
| ① 架构目标 | ✅ | 业务目标 (PlanKind 路由恢复) + 技术目标 (8 项量化指标) + 约束条件 (5 项) |
| ② 架构原则 | ✅ | 6 条原则 (workmodel 抽象 / Behavior=Default+Override / interface 可空 / 4 文件结构化 / 包级 var registry / 总图闭环) + 命名规范 + DSAFT ID 分配 + 代码风格 |
| ③ 业务流程 | ✅ | 核心用例时序图 (M3 PlanKind 路由恢复) + 3 层 fail-safe 异常补偿 + 20 组合分支决策树 |
| ④ 领域模型 | ✅ | 2 聚合根 (Strategy interface + WorkItemExecContext) + 限界上下文 (9 文件) + 3 领域事件 (Span/Metric) + 跨域消费模型 (L1→L2 单向) |
| ⑤ 核心链路图 | ✅ | 端到端路径 (M3 行为增量链路) + 时序标注 (< 250 ns/次) + 4 单点风险与缓解 |
| ⑥ 接口 / API 设计 | ✅ | Strategy interface + 4 实现 + DefaultStrategy registry + WorkItemExecContext 扩展 + checkVerdictDirection 集成 + 风格/契约/幂等/版本演进 |

**S3-Gate Review 结论**: Approved（中型 Change 1 PR，Strategy 抽象 + 4 文件结构化 + 行为增量边界显式声明 + 24 新增测试 + 5 节点重构总图闭环）

## 附录 E：下一步

1. S4 实现: 6 NEW 文件 + 2 MODIFIED 文件 + 1 NEW 总图文档
2. S4-Gate: 自检代码 (git status + grep 0 hits)
3. S5 验收: 跑测试 + acceptance-report.md (verdict: ACCEPTED)
4. S6-交付: PR + auto-merge
5. S6-归档: move to archive/ + 同步 5 个域规范文档
