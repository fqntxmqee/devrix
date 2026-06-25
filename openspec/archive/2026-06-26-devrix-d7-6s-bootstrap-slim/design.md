# Design: devrix-d7-6s-bootstrap-slim

**Change ID:** devrix-d7-6s-bootstrap-slim
**Status:** S7_Archived
**Priority:** P2
**Created:** 2026-06-26
**DM:** DM-20260626-007

---

## §1 背景与约束

### 1.1 起点

`internal/bootstrap/wire_coordinator.go` 当前 275 行 InitOrchestration，结构混杂：
- §2 config 加载 52 行（LoadConfigFile + coordCfg + tasksCfg + maxContextTokens + subagentCfg）
- §8 适配器定义 61 行（3 个 adapter + 4 个 util）
- 6 S 构造 inline（无 wire 包装）

### 1.2 约束

- **不破坏 InitOrchestration 外部接口**: 输入参数 6 个 + 输出 error
- **不破坏调用方 0 变化**: cmd/devrix + cmd/obs-verify + tests/testutil/d7_stack.go 0 变化
- **不引入新依赖**: 不引入 wire.go / fx 等 DI 框架
- **不重组子目录**: 保持 bootstrap/ 扁平结构
- **不删任何 wire 函数**: 7 wire 函数全部保留

### 1.3 目标

- InitOrchestration 主体 ≤ 200 行
- 6 S × WireFunc 命名一致（S1 0 wire, S2-S6 各 1 wire, 横切 0 wire）
- 3 个内嵌 adapter 函数拆到 `adapters.go`
- 4 个 util 函数拆到 `util.go`
- config 加载抽到 `loadOrchestratorConfigs()` 辅助

## §2 关键决策

### Decision 1: 不重组 bootstrap/ 子目录 (Option A)

- 22 个 .go + 14 个 _test.go 维持扁平结构
- 不引入 `bootstrap/d7/` 子目录
- **理由**: 22 个文件规模不够大，子目录化反而增加路径长度；当前扁平结构清晰

### Decision 2: 6 S × WireFunc 函数定义在 bootstrap/ 包内 (Option A)

- 新建 `internal/bootstrap/decision_planning.go` 定义 `WireDecisionPlanning`
- 新建 `internal/bootstrap/mups_pipeline.go` 定义 `WireMUPSPipeline`
- **理由**: 6 S wire 函数命名空间统一在 bootstrap/ 包内，与现有 `WireTurnInvoker` / `WireWaveScheduler` / `WireExecutionFlow` / `WireDelegate` 同位

### Decision 3: 3 个内嵌 adapter 全部拆到独立文件 adapters.go (Option A)

- 不分拆到 3 个独立文件（避免过度拆分）
- 合并到 `internal/bootstrap/adapters.go` (80 行)
- **理由**: 3 个 adapter 都是 InitOrchestration 的本地辅助，分散到 3 文件反而降低内聚性

### Decision 4: 4 个 util 函数拆到 util.go (Option A)

- `boolPtr` + `intPtr` + `strPtr` + `mapBackgroundStatus` 合并
- **理由**: 都是 1-3 行的 pointer helper 或 status 映射，独立文件过于碎片化

### Decision 5: config 加载抽到 `loadOrchestratorConfigs()` 辅助函数 (Option A)

- 同一文件内（wire_coordinator.go）抽辅助函数
- 返回 `*orchestratorConfigs` 结构体
- **理由**: config 加载 52 行可读性差，但抽到独立文件会增加 import 复杂度（config 包共享），同文件内抽辅助最简洁

### Decision 6: obsBridgeArg 类型断言抽到 `resolveObsBridge()` 辅助函数 (Option A)

- 4 行类型断言 → 1 行辅助调用
- **理由**: InitOrchestration 主体可读性 +1，且便于单测

## §3 实施步骤

### Step 1: util.go (NEW, 阶段 1)

```go
// internal/bootstrap/util.go
package bootstrap

import "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int { return &i }
func strPtr(s string) *string { return &s }

// mapBackgroundStatus converts BackgroundRegistry status string to coordinator TaskStatus.
// BackgroundRegistry uses "running" while the work model uses "in_progress"; all other
// values ("completed", "failed", "cancelled") match directly.
func mapBackgroundStatus(s string) orchtypes.TaskStatus {
    if s == "running" {
        return orchtypes.TaskStatusInProgress
    }
    return orchtypes.TaskStatus(s)
}
```

**验证**:
- `grep "func boolPtr\|func intPtr\|func strPtr\|func mapBackgroundStatus" internal/bootstrap/wire_coordinator.go` 0 命中
- `go build ./...` 0 错误

### Step 2: adapters.go (NEW, 阶段 2)

```go
// internal/bootstrap/adapters.go
package bootstrap

import (
    "context"
    "fmt"

    "github.com/devrix/devrix/internal/layers/communication/capture"
    "github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
    "github.com/devrix/devrix/internal/shared/contracts"
)

// contextEngineAdapter adapts contracts.IEngine → sessionorchestrator.IContextEngine.
type contextEngineAdapter struct { ... }
func newContextEngineAdapter(...) *contextEngineAdapter { ... }
func (a *contextEngineAdapter) Prepare(...) { ... }
func (a *contextEngineAdapter) Persist(...) { ... }
func (a *contextEngineAdapter) TokenCount(...) int { ... }

// turnOrchExecutor adapts sessionorchestrator.TurnOrchestrator → coordinator.TurnExecutor.
// DM-020 D-c: this replaces the legacy executor as the FastPath executor.
type turnOrchExecutor struct { ... }
func newTurnOrchExecutor(orch sessionorchestrator.TurnOrchestrator) *turnOrchExecutor { ... }
func (e *turnOrchExecutor) RunTurn(ctx context.Context, req sessionorchestrator.QueryRequest) (<-chan *contracts.EngineEvent, error) { ... }

// gatewayEventPublisher bridges EngineEvent → CommunicationGateway.
type gatewayEventPublisher struct { ... }
func newGatewayEventPublisher(gw *capture.CommunicationGateway) *gatewayEventPublisher { ... }
func (p *gatewayEventPublisher) Publish(ctx context.Context, ev *contracts.EngineEvent) { ... }
```

**注**: contextEngineAdapter 当前是 NewContextEngineAdapter 函数（外部使用）— 需要先 grep 确认是否在 initOrchestration 外部被使用。如有外部引用，保留 NewContextEngineAdapter 公开签名 + 在 adapters.go 中定义新内部类型。

**验证**:
- `grep "^func new" internal/bootstrap/wire_coordinator.go` 0 命中（newContextEngineAdapter + newTurnOrchExecutor + newGatewayEventPublisher 都已拆出）
- `go build ./...` 0 错误

### Step 3: decision_planning.go (NEW, 阶段 3a)

```go
// internal/bootstrap/decision_planning.go
package bootstrap

import "github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"

// WireDecisionPlanning wires the production LLM Task Decomposer for S5 (DecisionPlanning+Observe).
func WireDecisionPlanning(llmInvoker decisionplanning.LLMTaskDecomposer, defaultTier string) decisionplanning.LLMTaskDecomposer {
    return decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
        LLM:         llmInvoker,
        DefaultTier: defaultTier,
    })
}
```

**验证**:
- `go build ./...` 0 错误
- InitOrchestration 主体替换 `decisionplanning.NewLLMDecomposer(...)` 为 `WireDecisionPlanning(llmInvoker, llmStack.DefaultModel)`

### Step 4: mups_pipeline.go (NEW, 阶段 3b)

```go
// internal/bootstrap/mups_pipeline.go
package bootstrap

import (
    "github.com/devrix/devrix/internal/layers/observability"
    "github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
    "github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// MUPSPipelinesDeps wires the production S6 MUPS Pipeline (TurnToolExecutor + SubTurnRunner + PreparedTurnAdapter).
type MUPSPipelinesDeps struct {
    CtxAdapter        *contextEngineAdapter  // 从 adapters.go 引用
    OrchPath          *sessionorchestrator.OrchestratePath
    LLMInvoker        sessionorchestrator.LLMInvoker
    DefaultModel      string
    LoopFirst         bool
    ObsBridge         *observability.Bridge
    PlanMode          *workmodel.PlanMode
    SubagentCfg       SubagentConfig
    MaxContextTokens  int
    TaskManager       *workmodel.TaskManager
    ToolResultStore   ToolResultStorer
    WorkModel         *workmodel.LocalWorkModel
    FocusHintProvider FocusHintProviderIface
    ResolveAwaiter    ResolveAwaiterIface
}

// WireMUPSPipeline wires TurnToolExecutor + SubTurnRunner + PreparedTurnAdapter for S6.
// Returns: turnOrch (D7-S2-A06+A07) + subTurn (D7-S2-A06+A07 SubTurnRunner) + ctxPrep (D7-S2-A06 Prepare wrapper)
func WireMUPSPipeline(deps MUPSPipelinesDeps) (*sessionorchestrator.Orchestrator, *sessionorchestrator.SubTurnRunner, *sessionorchestrator.TurnPrepareWrapper, error) {
    toolExec := sessionorchestrator.NewTurnToolExecutor(deps.CtxAdapter, deps.OrchPath, deps.PlanMode, deps.LoopFirst)
    if deps.ObsBridge != nil {
        toolExec.SetTurnToolMetrics(sessionorchestrator.NewTurnToolMetrics(deps.ObsBridge.Meter()))
    }
    ctxPrep := &sessionorchestrator.TurnPrepareWrapper{Inner: deps.CtxAdapter, LoopFirst: deps.LoopFirst}

    turnOrch := sessionorchestrator.NewOrchestrator(sessionorchestrator.OrchestratorDeps{
        LLM:              deps.LLMInvoker,
        Context:          ctxPrep,
        Tools:            toolExec,
        Persist:          deps.CtxAdapter,
        MaxTurns:         0,
        DefaultModel:     deps.DefaultModel,
        MaxContextTokens: deps.MaxContextTokens,
        ObsBridge:        deps.ObsBridge,
        FocusHint:        deps.FocusHintProvider,
        ResolveAwait:     deps.ResolveAwaiter,
        ToolResultStore:  deps.ToolResultStore,
    })
    subTurn := sessionorchestrator.NewSubTurnRunner(turnOrch, sessionorchestrator.SubTurnConfig{
        DefaultMode:      deps.SubagentCfg.DefaultMode,
        LegacyMode:       deps.SubagentCfg.LegacyMode,
        MaxDepth:         deps.SubagentCfg.MaxDepth,
        MaxContextTokens: deps.MaxContextTokens,
    })
    return turnOrch, subTurn, ctxPrep, nil
}
```

**验证**:
- `go build ./...` 0 错误
- InitOrchestration 主体替换 30+ 行 S6 构造代码为 1 个 `WireMUPSPipeline(deps)` 调用

### Step 5: loadOrchestratorConfigs() (阶段 4a)

```go
// internal/bootstrap/wire_coordinator.go (新增辅助函数)
type orchestratorConfigs struct {
    coordCfg         config.CoordinatorConfig
    tasksCfg         config.TasksConfig
    subagentCfg      config.SubagentConfig
    maxContextTokens int
    contextEngineCfg *config.ContextEngineConfig
}

func loadOrchestratorConfigs(configFile string) (*orchestratorConfigs, error) {
    cfg := &orchestratorConfigs{
        coordCfg:         config.DefaultCoordinatorConfig(),
        tasksCfg:         config.DefaultTasksConfig(),
        subagentCfg:      config.DefaultSubagentConfig(),
        maxContextTokens: config.DefaultContextEngineConfig().MaxContextTokens,
    }
    if configFile == "" {
        return cfg, nil
    }
    fileCfg, err := config.LoadConfigFile(configFile)
    if err != nil {
        return cfg, nil  // 静默 fallback (与原行为一致)
    }
    cfg.coordCfg = config.BuildCoordinatorConfig(&fileCfg.Coordinator)
    if fileCfg.ContextEngine.Tasks.Mode != "" || fileCfg.ContextEngine.Tasks.StoreDir != "" {
        cfg.tasksCfg = fileCfg.ContextEngine.Tasks
    }
    _, _, _, ctxFileCfg, err := config.LoadConfig(configFile)
    if err == nil && ctxFileCfg != nil {
        cfg.tasksCfg = ctxFileCfg.Tasks
        cfg.subagentCfg = ctxFileCfg.Subagent.Normalized()
    }
    if fileCfg.ContextEngine.MaxContextTokens > 0 {
        cfg.maxContextTokens = fileCfg.ContextEngine.MaxContextTokens
    }
    return cfg, nil
}
```

**验证**:
- `grep "LoadConfigFile" internal/bootstrap/wire_coordinator.go` ≤ 1 命中（仅辅助函数内）
- `go build ./...` 0 错误

### Step 6: resolveObsBridge() (阶段 4b)

```go
// internal/bootstrap/wire_coordinator.go (新增辅助函数)
func resolveObsBridge(arg interface{}) *observability.Bridge {
    if b, ok := arg.(*observability.Bridge); ok {
        return b
    }
    return nil
}
```

**验证**:
- `go build ./...` 0 错误

### Step 7: InitOrchestration 主体重写 (阶段 4c)

参见 proposal.md §3.2。

**验证**:
- `wc -l internal/bootstrap/wire_coordinator.go` ≤ 250 行（含 helper + import + 注释）
- InitOrchestration 函数体 ≤ 200 行

## §4 跨包引用矩阵

| 新文件 | 引用包 | 引用类型 |
|--------|--------|----------|
| `util.go` | `orchtypes` | `TaskStatus` (1) |
| `adapters.go` | `capture` + `sessionorchestrator` + `contracts` | `CommunicationGateway` + `TurnOrchestrator` + `EngineEvent` (3) |
| `decision_planning.go` | `decisionplanning` | `LLMTaskDecomposer` + `LLMDecomposerDeps` (2) |
| `mups_pipeline.go` | `sessionorchestrator` + `workmodel` + `observability` | `Orchestrator` + `SubTurnRunner` + `PlanMode` + `TaskManager` + `Bridge` (5) |
| `wire_coordinator.go` (改) | (现有 import) | (保持) |

## §5 风险缓解

| 风险 | 缓解 |
|------|------|
| adapters.go 抽离后类型导出问题 | 用 grep 确认 contextEngineAdapter / turnOrchExecutor / gatewayEventPublisher 是否外部使用 — 当前都是 InitOrchestration 内部使用，可改为未导出小写 |
| mups_pipeline.go 参数多导致函数签名过长 | 用 `MUPSPipelinesDeps` 结构体聚合（类似 WaveSchedulerDeps 模式） |
| loadOrchestratorConfigs 静默 fallback 改变错误行为 | 保留原"LoadConfigFile 失败 → 静默用默认"的语义 |
| resolveObsBridge 改变 obsBridge 为 nil 时的行为 | 保留原"类型断言失败 → nil"语义 |
| 新增 wire 函数调用顺序 | 严格遵循原 InitOrchestration 内部调用顺序（详见 proposal §3.2） |

## §6 验证步骤

### 6.1 编译验证

```bash
go build ./...                                    # 0 错误
go vet ./...                                       # 0 警告
```

### 6.2 单元测试

```bash
go test -race -count=1 ./internal/bootstrap/...    # 全 PASS
```

### 6.3 集成测试

```bash
go test -race -count=1 ./internal/layers/orchestration/...  # 22/22 PASS
```

### 6.4 关键 LP 验证

```bash
# LP-1: Bayesian reputation (TestAutoClose_FullLP1Loop)
go test -race -count=1 ./internal/layers/orchestration/sessionorchestrator -run TestAutoClose_FullLP1Loop

# LP-2: 5 节点管道 (TestIntegration_5NodePipeline_End2End)
go test -race -count=1 ./internal/layers/orchestration/escape -run TestIntegration_5NodePipeline_End2End

# LP-5: Cross-session traceability
go test -race -count=1 ./internal/layers/orchestration/sessionorchestrator -run "TestSessionOrchestrator_BuildObserveRequest"
```

### 6.5 Baseline stability

```bash
git diff --stat hardening/ escape/circuit_breaker.go sessionorchestrator/autoclose.go
# 空 (0 变化)
```

### 6.6 调用方 0 变化

```bash
git diff --stat cmd/devrix/main.go cmd/obs-verify/main.go tests/testutil/d7_stack.go internal/layers/orchestration/
# 仅 internal/bootstrap/ 内部 + 4 文档
```

## §7 文档同步计划

### 7.1 openspec/specs/d7-orchestration/d7-domain.md v2.3.0 → v2.4.0

新增 §"Bootstrap Wire 拓扑" 章节：

```markdown
## Bootstrap Wire 拓扑 (v2.4.0)

| S 层 | 博弈角色 | Wire 函数 | 物理位置 |
|------|----------|-----------|----------|
| S1 WorkModel | State Authority | 0 wire (注入式构造) | InitOrchestration 内联 |
| S2 SessionOrchestrator | Mediator+Turn Leader | WireTurnInvoker | bootstrap/turn_wiring.go |
| S3 WaveScheduler | Mechanism Designer | WireWaveScheduler + BuildOrchestratePath | bootstrap/wire_wave.go |
| S4 ExecutionFlow+Verify | Costly Signaler+Certifier | WireExecutionFlow | bootstrap/execution_flow.go |
| S5 DecisionPlanning+Observe | Information Producer+Quantizer | WireDecisionPlanning (NEW) | bootstrap/decision_planning.go |
| S6 MUPS Pipeline | Pipeline Coordinator+Memory | WireMUPSPipeline (NEW) | bootstrap/mups_pipeline.go |
| 横切 Hardening | Discipline Keeper | 0 wire (Observability 自动注入) | (隐式) |

总入口: `InitOrchestration` (单点) ≤ 200 行, 6 S + 1 横切 6 wire 函数 + 3 adapter 拆到 adapters.go.
```

### 7.2 openspec/specs/d7-orchestration/design.md v4.3.0 → v4.4.0

§"Bootstrap" 章节展开: 6 S × WireFunc 函数清单 + 拓扑图 + 调用链。

### 7.3 openspec/specs/d7-orchestration/t-registry.md v4.5.0 → v4.6.0

新增 D7-S2-A51 章节 4 P0 T (T01-T04), 全部 IMPLEMENTED。

### 7.4 openspec/t-registry.md (root) v5.5.0 → v5.6.0

新增 DM-20260626-007 增量条目 + Statistics 更新 (Total 540→544, P0 350→354)。

## §8 后续 follow-up 状态

完成本 PR 后 v6.0.0 follow-up 序列 6 PR 收官：

- #1 spec (DM-20260626-001) S7_Archived ✅
- #2 mups (DM-20260626-002) S7_Archived ✅
- #3 hardening (DM-20260626-003) S7_Archived ✅
- #4 turn-merge (DM-20260626-004) S7_Archived ✅
- #5 verify-promotion (DM-20260626-005) S7_Archived ✅
- #5' observe-merge (DM-20260626-006) S1_Cancelled ❌
- **#6 bootstrap-slim (DM-20260626-007) S7_Archived (本次)**

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-26 | 初版：8 sections S3 设计，6 Decision 详细论证 + 7 Step 实施 + 4 文档同步 + v6.0.0 收官声明 |
