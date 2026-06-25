# Proposal: devrix-d7-6s-bootstrap-slim

**Change ID:** devrix-d7-6s-bootstrap-slim
**Status:** S7_Archived
**Priority:** P2
**Created:** 2026-06-26
**DM:** DM-20260626-007
**Related:** devrix-d7-six-s-simplification (DM-20260626-001) · devrix-d7-6s-package-merge (DM-20260626-004) · devrix-d7-6s-verify-promotion (DM-20260626-005) · devrix-d7-6s-observe-merge-cancel (DM-20260626-006)

---

## §1 概述

把 `internal/bootstrap/` 中的 D7 编排层 wire 拓扑从"6 S + 1 横切 散落 6+ 文件" 收口为 **6 S × 1 wire + 1 横切 × 0 wire = 7 入口** 的清晰拓扑；并完成 `InitOrchestration` 函数内部 30 行 config 加载 + 3 个内嵌 adapter 函数 + 3 个 util 函数的清理拆分工作。

本次是 v6.0.0 域升级 follow-up 序列的**最终收口 PR**：6 PR 序列 5 S7_Archived + 1 S1_Cancelled + 1 S7_Archived（本次）。

## §2 动机

### 2.1 现状问题

调研 `internal/bootstrap/wire_coordinator.go` (275 行 InitOrchestration) 当前结构：

| 段 | 行号 | 内容 | 大小 |
|----|------|------|------|
| §0 注释 | 1-29 | 文档头 | 29 行 |
| §1 函数签名 | 30-37 | InitOrchestration | 8 行 |
| §2 config 加载 | 38-89 | LoadConfigFile + coordCfg + tasksCfg + maxContextTokens + subagentCfg | **52 行** ⚠️ |
| §3 obs bridge 断言 | 90-94 | 类型断言 + 设置 | 5 行 |
| §4 S1 WorkModel | 95-131 | TaskManager + Registry + WorkModel + BackgroundProvider | 37 行 |
| §5 S2 SessionOrchestrator | 132-184 | ctxAdapter + llmInvoker + sink + orchPath + planMode + toolExec + ctxPrep + turnOrch + subTurn | 53 行 |
| §6 S2 编排入口 | 185-205 | executor + orch + entry + gw.SetOrchestrationEntry | 21 行 |
| §7 横切 Hardening | 206-213 | d7spans.SetBridge | 8 行 |
| §8 适配器定义 | 215-275 | 3 adapter 类型 + 3 util 函数 | 61 行 ⚠️ |

**两个清理点**：
- §2 config 加载 52 行可抽到辅助函数（InitOrchestration 主体可降到 ≤ 200 行）
- §8 适配器定义 61 行不在 InitOrchestration 主体但与 InitOrchestration 在同一文件，可拆到独立文件

### 2.2 wire 拓扑对标

当前 D7 wire 函数清单：

| Wire 函数 | 归属 | 文件 | 调用方 |
|-----------|------|------|--------|
| `WireTurnInvoker` | S2 (Mediator+Turn Leader) | `turn_wiring.go` | InitOrchestration |
| `WireWaveScheduler` | S3 (Mechanism Designer) | `wire_wave.go` | BuildOrchestratePath |
| `BuildOrchestratePath` | S3 helper | `wire_wave.go` | InitOrchestration |
| `WireExecutionFlow` | S4 (Costly Signaler+Certifier) | `execution_flow.go` | 4 子文件: execution_flow.go + freefork_injection.go + task_notif_injection.go + transcript_writer.go |
| `WireDelegate` | D7 通用 | `delegate.go` | 待接入 InitOrchestration |
| `newContextEngineAdapter` (隐式) | S2 internal | `wire_coordinator.go` | InitOrchestration |
| `newTurnOrchExecutor` (隐式) | S2 internal | `wire_coordinator.go` | InitOrchestration |
| `newGatewayEventPublisher` (隐式) | S2 internal | `wire_coordinator.go` | InitOrchestration |
| `newPlanLLMCompleter` (隐式) | S5 internal | `plan_llm_completer.go` | InitOrchestration |
| `WireExecutionFlow` 子包: `freefork_injection.go` + `task_notif_injection.go` + `transcript_writer.go` | S4 | 同名文件 | (待清理归位) |
| `d7spans.SetBridge` | 横切 Hardening | (调用 d7spans 包) | InitOrchestration |

**问题**：
1. 3 个内嵌 adapter 函数的"wire 命名风格"与 S2-S4 wire 函数不一致（无 `Wire` 前缀）
2. S5 决策规划部分 inline 在 InitOrchestration 主体（`decisionplanning.NewLLMDecomposer` + `newPlanLLMCompleter`），无 `WireDecisionPlanning` 包装
3. S6 MUPS Pipeline 部分也 inline（`NewTurnToolExecutor` + `NewSubTurnRunner` + `NewPreparedTurnAdapter`），无 `WireMUPSPipeline` 包装

### 2.3 业务价值

| 类别 | 收益 |
|------|------|
| **可读性** | InitOrchestration 主体从 275 行降到 ≤ 200 行，6 S 组合入口清晰 |
| **可测试性** | 3 个 adapter 抽到独立文件后，可单独写 unit test（当前 inline 无法单测） |
| **可演进性** | 6 S × WireFunc 命名一致后，新人看 bootstrap 立即理解 6 S 拓扑 |
| **可观测性** | 文档化 6 S wire 调用图，未来故障定位更快 |

## §3 方案

### 3.1 总体方案

保持 InitOrchestration 的输入/输出/外部行为 **100% 不变**，仅做内部结构调整 + 文件拆分。

**3 个新文件**：
- `internal/bootstrap/adapters.go` (NEW): 3 个 adapter 类型 + 3 个 adapter 构造器 + 3 个 util 函数
- `internal/bootstrap/util.go` (NEW): 3 个 pointer helper + mapBackgroundStatus
- `internal/bootstrap/decision_planning.go` (NEW): `WireDecisionPlanning(llmInvoker, defaultTier) decisionplanning.LLMTaskDecomposer`
- `internal/bootstrap/mups_pipeline.go` (NEW): `WireMUPSPipeline(sink, ctxAdapter, orchPath, llmInvoker, defaultTier, obsBridge, subagentCfg, maxContextTokens, tm, turnOrch) *mupsPipeline`

**2 个文件修改**：
- `internal/bootstrap/wire_coordinator.go`:
  - 引入 3 个新文件的 wire 函数
  - 提取 config 加载到 `loadOrchestratorConfigs()` 辅助
  - 提取 obs bridge 类型断言到 `resolveObsBridge()` 辅助
  - InitOrchestration 主体保留 6 S 组合入口（≤ 200 行）
- `internal/bootstrap/plan_llm_completer.go`:
  - 抽 `newPlanLLMCompleter` 到 `decision_planning.go`（可选）OR 保留独立文件

### 3.2 InitOrchestration 主体结构 (改后)

```go
// InitOrchestration 主体 (改后 ~150 行)
func InitOrchestration(configFile string, gw *capture.CommunicationGateway,
    ctxEngine contracts.IEngine, obsBridgeArg interface{},
    llmStack llmbridge.ContextLLMStack, agentToolReg *external.Registry) error {

    // 1. config 加载 (抽到辅助)
    cfg, err := loadOrchestratorConfigs(configFile)
    if err != nil { return err }
    if !cfg.coordCfg.Enabled {
        return fmt.Errorf("d7: disabled (d7.enabled=false)")
    }
    coordinatorCfg := cfg.buildCoordinatorCfg(...)
    maxContextTokens := cfg.maxContextTokens
    subagentCfg := cfg.subagentCfg

    // 2. obs bridge 解析 (抽到辅助)
    obsBridge := resolveObsBridge(obsBridgeArg)

    // 3. 横切 Hardening (隐式 0 wire)
    slog.Info("d7: initializing SessionOrchestrator", ...)

    // 4. S1 WorkModel (State Authority, 0 wire 注入式)
    tm := workmodel.NewTaskManagerFromConfig(cfg.tasksCfg, obsBridge)
    tm.SetRegistry(runregistry.NewRegistry("~/.devrix/runs"))
    wm := sessionorchestrator.NewLocalWorkModel(tm)
    wm.SetBackgroundProvider(...)

    // 5. S2 SessionOrchestrator (Mediator+Turn Leader)
    ctxAdapter := newContextEngineAdapter(gw, ctxEngine, llmStack.TokenCounter)
    llmInvoker := WireTurnInvoker(llmStack)
    sink := newGatewayEventPublisher(gw)

    // 6. S5 DecisionPlanning+Observe (Info Producer+Quantizer)
    llmDecomp := WireDecisionPlanning(llmInvoker, llmStack.DefaultModel)

    // 7. S3 WaveScheduler (Mechanism Designer)
    orchPath := BuildOrchestratePath(sink, llmDecomp, WaveSchedulerDeps{...})
    orchPath.SetTaskManager(tm)

    // 8. S6 MUPS Pipeline (Pipeline Coordinator+Memory)
    planMode := workmodel.NewPlanMode(newPlanLLMCompleter(llmInvoker, llmStack.DefaultModel), obsBridge)
    toolExec := sessionorchestrator.NewTurnToolExecutor(ctxAdapter, orchPath, planMode, ...)
    subTurn := sessionorchestrator.NewSubTurnRunner(turnOrch, ...)

    // 9. S2 编排入口
    turnOrch := sessionorchestrator.NewOrchestrator(OrchestratorDeps{...})
    orch := sessionorchestrator.NewSessionOrchestrator(...)
    gw.SetOrchestrationEntry(sessionorchestrator.NewEntry(orch))

    // 10. 横切 Hardening d7spans bridge
    d7spans.SetBridge(obsBridge)
    return nil
}
```

### 3.3 新文件骨架

#### `internal/bootstrap/adapters.go` (NEW, ~80 行)

```go
package bootstrap

import (
    "context"
    "github.com/devrix/devrix/internal/layers/communication/capture"
    "github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
    "github.com/devrix/devrix/internal/shared/contracts"
)

// contextEngineAdapter adapts contracts.IEngine to sessionorchestrator.IContextEngine.
type contextEngineAdapter struct { ... }
func newContextEngineAdapter(...) *contextEngineAdapter { ... }
func (a *contextEngineAdapter) Prepare(...) { ... }
func (a *contextEngineAdapter) Persist(...) { ... }
func (a *contextEngineAdapter) TokenCount(...) int { ... }

// turnOrchExecutor adapts sessionorchestrator.TurnOrchestrator to coordinator.TurnExecutor.
type turnOrchExecutor struct { ... }
func newTurnOrchExecutor(orch sessionorchestrator.TurnOrchestrator) *turnOrchExecutor { ... }
func (e *turnOrchExecutor) RunTurn(ctx context.Context, req sessionorchestrator.QueryRequest) (<-chan *contracts.EngineEvent, error) { ... }

// gatewayEventPublisher bridges EngineEvent → CommunicationGateway.
type gatewayEventPublisher struct { ... }
func newGatewayEventPublisher(gw *capture.CommunicationGateway) *gatewayEventPublisher { ... }
func (p *gatewayEventPublisher) Publish(ctx context.Context, ev *contracts.EngineEvent) { ... }
```

#### `internal/bootstrap/util.go` (NEW, ~30 行)

```go
package bootstrap

import "github.com/devrix/devrix/internal/layers/orchestration/orchtypes"

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int { return &i }
func strPtr(s string) *string { return &s }

func mapBackgroundStatus(s string) orchtypes.TaskStatus {
    if s == "running" { return orchtypes.TaskStatusInProgress }
    return orchtypes.TaskStatus(s)
}
```

#### `internal/bootstrap/decision_planning.go` (NEW, ~30 行)

```go
package bootstrap

import "github.com/devrix/devrix/internal/layers/orchestration/decisionplanning"

// WireDecisionPlanning wires the production LLM Decomposer for S5.
func WireDecisionPlanning(llmInvoker decisionplanning.LLMTaskDecomposer, defaultTier string) decisionplanning.LLMTaskDecomposer {
    return decisionplanning.NewLLMDecomposer(decisionplanning.LLMDecomposerDeps{
        LLM:         llmInvoker,
        DefaultTier: defaultTier,
    })
}
```

#### `internal/bootstrap/mups_pipeline.go` (NEW, ~80 行)

```go
package bootstrap

// WireMUPSPipeline wires TurnToolExecutor + SubTurnRunner + PreparedTurnAdapter for S6.
func WireMUPSPipeline(deps MUPSPipelines) *mupsPipeline {
    toolExec := sessionorchestrator.NewTurnToolExecutor(deps.CtxAdapter, deps.OrchPath, deps.PlanMode, deps.LoopFirst)
    if deps.ObsBridge != nil {
        toolExec.SetTurnToolMetrics(sessionorchestrator.NewTurnToolMetrics(deps.ObsBridge.Meter()))
    }
    ctxPrep := &sessionorchestrator.TurnPrepareWrapper{Inner: deps.CtxAdapter, LoopFirst: deps.LoopFirst}
    turnOrch := sessionorchestrator.NewOrchestrator(sessionorchestrator.OrchestratorDeps{
        LLM: deps.LLM, Context: ctxPrep, Tools: toolExec, Persist: deps.CtxAdapter, ...,
    })
    subTurn := sessionorchestrator.NewSubTurnRunner(turnOrch, sessionorchestrator.SubTurnConfig{...})
    return &mupsPipeline{...}
}
```

### 3.4 wire 拓扑对比

| 项目 | 改前 | 改后 |
|------|------|------|
| D7 Wire 函数 | 6 (WireTurnInvoker + WireWaveScheduler + BuildOrchestratePath + WireExecutionFlow + WireDelegate + d7spans.SetBridge) | 8 (新增 WireDecisionPlanning + WireMUPSPipeline) |
| InitOrchestration 行数 | 275 | ≤ 200 |
| 内嵌 adapter 函数 | 3 (newContextEngineAdapter + newTurnOrchExecutor + newGatewayEventPublisher) | 0 (已拆到 adapters.go) |
| 内嵌 util 函数 | 4 (boolPtr + intPtr + strPtr + mapBackgroundStatus) | 0 (已拆到 util.go) |
| config 加载 inline | 52 行 | 0 (已抽到 loadOrchestratorConfigs) |
| 文档同步 | 4 文档 version bump | 同 |

## §4 备选方案

### 方案 B：仅文档化，不动代码 (Rejected)

- 不抽 adapter 函数，不动 InitOrchestration 行数
- 仅写一份 "bootstrap 拓扑指南" markdown 文档
- **拒绝理由**: 不解决核心问题（InitOrchestration 275 行 + 内嵌 adapter 散落），仅"文档"无法改善代码可读性

### 方案 C：引入 wire.go 或 fx 框架 (Rejected)

- 用 Google wire 或 uber-go/fx 替代手写 wire
- **拒绝理由**: 引入大依赖（wire 是代码生成器，fx 是 reflection 容器），与 v6.0.0 精简理念冲突；当前 8 个 wire 函数规模太小不值得引入

### 方案 D：重组 bootstrap 到 `bootstrap/d7/` 子目录 (Rejected)

- 把 D7 相关的 wire 全部归类到 `internal/bootstrap/d7/` 子目录
- **拒绝理由**: 22 个 bootstrap 文件规模不够大，子目录化反而增加路径长度；当前扁平结构清晰

## §5 实施路径

### 5.1 阶段 1：辅助函数抽离 (1 PR, 0.5 天)

| 步骤 | 文件 | 改动 |
|------|------|------|
| 1 | `internal/bootstrap/util.go` (NEW) | 3 个 pointer helper + mapBackgroundStatus |
| 2 | `internal/bootstrap/wire_coordinator.go` | 删除原定义 + 改 import 引用 util.go |
| 3 | 验证 `go build ./...` | 0 错误 |

### 5.2 阶段 2：3 个 adapter 拆到 adapters.go (1 PR, 0.5 天)

| 步骤 | 文件 | 改动 |
|------|------|------|
| 1 | `internal/bootstrap/adapters.go` (NEW) | 3 个 adapter 类型 + 3 个 adapter 构造器 + adapter 方法 |
| 2 | `internal/bootstrap/wire_coordinator.go` | 删除原定义 + 改 import 引用 adapters.go |
| 3 | 验证 `go build ./...` + `go vet ./...` | 0 错误 0 警告 |

### 5.3 阶段 3：S5/S6 Wire 包装 (1 PR, 1 天)

| 步骤 | 文件 | 改动 |
|------|------|------|
| 1 | `internal/bootstrap/decision_planning.go` (NEW) | WireDecisionPlanning 包装 |
| 2 | `internal/bootstrap/mups_pipeline.go` (NEW) | WireMUPSPipeline 包装 |
| 3 | `internal/bootstrap/wire_coordinator.go` | InitOrchestration 主体调用新 wire 函数 |
| 4 | 验证 `go build/vet/test -race` | 22/22 PASS |

### 5.4 阶段 4：Config 加载抽取 + InitOrchestration 收尾 (1 PR, 0.5 天)

| 步骤 | 文件 | 改动 |
|------|------|------|
| 1 | `internal/bootstrap/wire_coordinator.go` | 抽 `loadOrchestratorConfigs()` + `resolveObsBridge()` 辅助 |
| 2 | InitOrchestration 主体降到 ≤ 200 行 | 6 S 组合入口清晰 |
| 3 | 验证 `go build/vet/test -race` | 22/22 PASS |

### 5.5 阶段 5：文档同步 + 验收 (1 天)

- 4 文档 version bump
- 4 个新 P0 T 注册
- 13 AC 验收
- S6 归档

## §6 关键文件

### 新建 (4 文件)

- `internal/bootstrap/adapters.go` (~80 行)
- `internal/bootstrap/util.go` (~30 行)
- `internal/bootstrap/decision_planning.go` (~30 行)
- `internal/bootstrap/mups_pipeline.go` (~80 行)

### 修改 (1 文件)

- `internal/bootstrap/wire_coordinator.go` (275 行 → ≤ 200 行)

### 不变 (3+ 文件)

- `internal/bootstrap/{turn_wiring,wire_wave,execution_flow,delegate,multi_agent,agent_tools,im_hosts,context_engine_v3}.go`
- `internal/bootstrap/{subturn_wire,plan_llm_completer,cli_events,observability,surfaces,transcript_writer,task_notif_injection,freefork_injection}.go`
- `cmd/devrix/main.go` + `cmd/obs-verify/main.go` + `tests/testutil/d7_stack.go` (调用方 0 变化)

### 文档同步 (4 文档)

- `openspec/specs/d7-orchestration/d7-domain.md` v2.3.0 → v2.4.0
- `openspec/specs/d7-orchestration/design.md` v4.3.0 → v4.4.0
- `openspec/specs/d7-orchestration/t-registry.md` v4.5.0 → v4.6.0
- `openspec/t-registry.md` (root) v5.5.0 → v5.6.0

## §7 T 层验证

### 4 个新 P0 T (D7-S2-A51)

| T ID | 名称 | 状态 |
|------|------|------|
| **D7-S2-A51-T01** | 6 S × WireFunc 命名一致 + 1 横切 × 0 wire | PLANNED → IMPLEMENTED |
| **D7-S2-A51-T02** | InitOrchestration 主体 ≤ 200 行 + 6 S 组合入口清晰 | PLANNED → IMPLEMENTED |
| **D7-S2-A51-T03** | 3 个内嵌 adapter 函数已拆到 adapters.go + 0 残留内嵌 | PLANNED → IMPLEMENTED |
| **D7-S2-A51-T04** | 22/22 orchestration packages go test -race PASS + LP-1/2/5 100% 兼容 + 调用方 0 变化 | PLANNED → IMPLEMENTED |

## §8 后续 follow-up 状态

| # | change-id | 状态 | PR |
|---|-----------|------|-----|
| #1 | devrix-d7-six-s-simplification | ✅ S7_Archived | #215 |
| #2 | devrix-d7-mups-package-migration | ✅ S7_Archived | #216+#217 |
| #3 | devrix-d7-hardening-cross-cutting | ✅ S7_Archived | #218+#219 |
| #4 | devrix-d7-6s-package-merge | ✅ S7_Archived | #220+#221 |
| #5 | devrix-d7-6s-verify-promotion | ✅ S7_Archived | #222+#223 |
| #5' | devrix-d7-6s-observe-merge-cancel | ❌ S1_Cancelled | (#224) |
| **#6** | **devrix-d7-6s-bootstrap-slim** | **⏳ S2_Proposal (本次)** | **—** |

完成本 PR 后，v6.0.0 follow-up 6 PR 序列 5/6 S7_Archived + 1/6 S1_Cancelled + 1/1 S7_Archived（本次）收官。

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-26 | 初版：8 sections S2 提案，4 备选 (A 选, B/C/D rejected), 5 阶段 4 PR 实施路径 |
