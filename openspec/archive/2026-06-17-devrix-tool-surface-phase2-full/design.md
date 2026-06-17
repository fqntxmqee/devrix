# Design: devrix-tool-surface-phase2-full

**Change ID:** devrix-tool-surface-phase2-full
**Demand ID:** DM-20260617-008
**Status:** S3_Design
**Parent change:** devrix-tool-surface-contract (DM-20260617-007, PR #63 merged, S7_archived)

---

## 0. 范围声明

本 change **严格限定在父 change `design.md §2.8` "阶段 2 (PR #64)" 描述的工作**。不引入新 AC、不修改设计、不重命名已有标识符、不引入 DI 框架。仅完成：

1. 删除 5 个剩余 global singleton
2. 全量 caller 改构造期显式 dep 注入

**5 个待删 global**:

| ID | Global | 包位置 | 现有 caller 数 | 默认值 |
|----|--------|--------|---------------|--------|
| G1 | `transcript.GlobalWriter` | `internal/layers/communication/capture/transcript/wire.go` | 1 prod (gateway.go:811) + 2 test | `nil` |
| G2 | `flow.GlobalHub` | `internal/layers/orchestration/flow/hub.go` | 4 prod (delegate_tools.go:159, hubspoke/dispatch.go:69, subquery_fallback.go:30-31, bootstrap/execution_flow.go:36) | NoOpExecutionFlowHub{} |
| G3 | `sessionqueue.GlobalSessionQueue` | `internal/layers/orchestration/sessionqueue/session_queue.go` | 5 prod (context_engine.go:181, context_engine_builder.go:235, wire_wave.go:118, execution_flow.go:32, flow/hub.go:56) | `NewSessionQueue()` |
| G4 | `workmodel.GlobalTaskManager` | `internal/layers/orchestration/workmodel/task_manager.go` | 6+ prod (cli.go:56, command_handler.go:167, orchestrator.go:150,416, delegate_tools.go:171,181, wire_coordinator.go:95) | `init()` 调用 `NewTaskManager()` |
| G5 | `freefork.GlobalForker` (包内) | `internal/layers/multiagent/provision/freefork/wire.go` | 1 prod (freefork_injection.go:34) | `nil` |

---

## 1. 设计原则

### 1.1 父 design §2.8 阶段 2 已 lock-in 的策略

父 design §2.8 明确 "阶段 2 (PR #64)" 范围：

> - `git grep` 验证 6+ global var 零引用
> - 删除 global var + setter 函数
> - 全量单测 + E2E IM 验证
> - 灰度 1 周

父 design §2.8 明确 "阶段 2 回滚"：

> 阶段 2 回滚: revert PR #64, 但 global var 已被删 — 需用 `git revert` 然后从 git history 恢复; **不推荐阶段 2 后回滚**

本 change 不重新讨论回滚路径。

### 1.2 注入方式: 显式 ctor 参数

按父 design §1 "接口隔离" 原则 + Go "Accept interfaces, return structs" 模式:
- **不用** DI 框架 (wire / fx / dig)
- **不引入** service locator
- **不** 推迟初始化到 lazy getter
- **改** 显式构造期 ctor 参数 + struct 字段持有

每个 sub-commit 选用的注入方式因模块耦合度不同而异（详见 §2 各 sub-commit 设计）。

### 1.3 范围零外扩

- **不修改** 父 change 已归档的 22 AC
- **不修改** 父 change 的 surface / filter / ToolSpec / ToolResult 字段
- **不修改** D2/D3/D4/D5/D6 library 对外 API
- **不重命名** 任何 "SetGlobal*" / "Global*" 标识符 (仅删除 var + setter)
- **不引入** ToolSpec 风险等级字段 (PerRiskFilter 继续用既有 ToolSpec.Risk)

### 1.4 测试策略

- 每个 sub-commit 必须 `go test -race ./...` 100% 绿
- 既有测试改注入方式 (移除 `defer reset(global)` 反模式)
- 新增的"零引用"测试点 = `git grep` 静态验证 (作为 S5 验收脚本)

---

## 2. Sub-commit 详细设计

### 2.1 Sub-commit 1: `transcript.GlobalWriter` 删除

**改动文件**:
- `internal/layers/communication/capture/transcript/wire.go` — 删除 global var + setter + Append (3 函数 + 1 var)
- `internal/layers/communication/capture/gateway.go` — `Gateway` struct 加 `writer *transcript.Writer` 字段; `NewCommunicationGateway` 增 writer 参数
- `internal/layers/communication/capture/gateway.go:811` — `ExpireSession` 改用 `g.writer.Append(...)` 而非 `transcript.GlobalWriter()`
- `internal/bootstrap/context_engine.go:123` — 调用 `NewCommunicationGateway(..., tw)` 注入
- `internal/bootstrap/context_engine_builder.go:181` — 同上
- `internal/layers/communication/capture/session_store_transcript_test.go` — 改用 `NewCommunicationGateway(..., tw)` 测试构造

**注入方式**: struct 字段注入 + ctor 参数。`Gateway` 已有 `obsBridge *observability.Bridge` 字段先例, 模式一致。

**保留的 `Append` 入口**: 因有 6+ caller 跨库使用, 不删 `transcript.Append(sessionID, ev)`, 但 `Append` 内部不再读 global var, 而改接受 `*Writer` 显式参数 (或保留为 free function, 但要求 caller 传 writer)。

**决策**: 保留 `Append` 简写形式, 但签名改 `Append(w *Writer, sessionID string, ev Event) error`, 强制 caller 显式传 writer。

**测试改写**:
```go
// 旧:
prevW := transcript.GlobalWriter()
transcript.SetGlobalWriter(tw)
t.Cleanup(func() { transcript.SetGlobalWriter(prevW) })
gw := NewCommunicationGateway(...)

// 新:
gw := NewCommunicationGateway(..., tw)  // writer 注入到 ctor
```

### 2.2 Sub-commit 2: `flow.GlobalHub` 删除

**改动文件**:
- `internal/layers/orchestration/flow/hub.go` — 删除 `GlobalHub` var + `SetGlobalHub` 函数
- `internal/bootstrap/execution_flow.go:23,36` — 删除 `SetGlobalHub(nil)` / `SetGlobalHub(hub)` 调用 (已无 reader)
- `internal/bootstrap/delegate.go:57` — `nil` 占位 (注释说明 hub 由 deps 注入)
- `internal/layers/orchestration/hubspoke/dispatch.go:69` — `hub = flow.GlobalHub` 改 `hub = deps.Hub`
- `internal/layers/orchestration/delegatetools/delegate_tools.go:159` — `flow.GlobalHub.Snapshot(...)` 改 `deps.Hub.Snapshot(...)`
- `internal/layers/orchestration/delegatetools/subquery_fallback.go:30-31` — 改读 `deps.Hub`
- `internal/layers/orchestration/delegatetools/subquery_fallback_test.go` — 改 deps 注入

**注入方式**: `delegatetools.Deps` / `hubspoke.Deps` struct 加 `Hub contracts.ExecutionFlowHub` 字段 (已经存在, 已有 NoneOp 占位)。

**核心改动**: `flow.Hub` 已经是构造期 ctor (`NewHub(HubDeps)`), PR #63 阶段 1 已完成。本 sub-commit 仅删 global + 改 caller 走 deps.Hub, 不动 `Hub` 本身。

**特殊处理**: `delegate.go:57` 的 `nil, // uses flow.GlobalHub by default` 注释需要更新成 `nil, // uses NoOp by default (hub wired via deps.Hub)`, 避免误导。

### 2.3 Sub-commit 3: `sessionqueue.GlobalSessionQueue` 删除

**改动文件**:
- `internal/layers/orchestration/sessionqueue/session_queue.go` — 删除 `GlobalSessionQueue` var
- `internal/layers/orchestration/flow/hub.go:56` — `q = sessionqueue.GlobalSessionQueue` 改 `q = deps.Queue` (即 NewHub 接受 Queue 后, 已有 `deps.Queue` 字段)
- `internal/bootstrap/wire_wave.go:118` — `EngineDeps` 已是字段, 仅改 ctor 接受 queue
- `internal/bootstrap/context_engine_builder.go:235` — 同上
- `internal/bootstrap/context_engine.go:181` — 同上
- `internal/bootstrap/execution_flow.go:32` — 改传 `NewSessionQueue()` 局部实例

**注入方式**: `EngineDeps.SessionCommandQueue` 字段已存在 (父 change 阶段 1 完成), 5 caller 仅需把 `sessionqueue.GlobalSessionQueue` 替换为 `q := sessionqueue.NewSessionQueue()` 局部实例 (5 处独立局部 var)。

**设计决策**: 不引入新的 `bootstrap.SessionQueue` 共享单例, 因为每个 bootstrap 调用方可能配置不同的 queue (mode / size)。每个 caller 自行 `NewSessionQueue()` 创建, 跨 caller 不共享 (无破坏性: 父 change 阶段 1 已经按 SessionCommandQueue 字段走, GlobalSessionQueue 实际是死代码)。

### 2.4 Sub-commit 4: `workmodel.GlobalTaskManager` 删除

**改动文件**:
- `internal/layers/orchestration/workmodel/task_manager.go` — 删除 `GlobalTaskManager` var + 删除 `init()` 函数
- `internal/layers/orchestration/workmodel/task_manager.go` — `InitGlobalTaskManager` 函数保留, 但改返回 `*TaskManager` (而非写 global)
- `internal/bootstrap/wire_coordinator.go:95` — `coordinator.NewLocalWorkModel(workmodel.GlobalTaskManager)` 改 `coordinator.NewLocalWorkModel(tm)` 接受参数
- `internal/bootstrap/context_engine_builder.go:98,107` — 改持局部 `tm` 并注入
- `internal/bootstrap/context_engine.go:53,62` — 同上
- `internal/bootstrap/execution_flow.go:33` — `Tasks: workmodel.GlobalTaskManager` 改 `Tasks: tm`
- `internal/layers/orchestration/coordinator/command_handler.go:156,167` — `command_handler` 接受 `*TaskManager` 参数 (已有 `NewCommandHandler` ctor)
- `internal/layers/orchestration/coordinator/orchestrator.go:150,416` — `Orchestrator` struct 加 `tasks *TaskManager` 字段, `NewOrchestrator` 增参数
- `internal/layers/orchestration/delegatetools/delegate_tools.go:171,181` — `delegate_tools.Deps` 加 `Tasks *TaskManager` 字段
- `internal/layers/communication/channel/adapters/cli.go:56` — CLI adapter 接受 `*TaskManager` 参数

**注入方式**: 标准 ctor 注入 + struct 字段持有。

**`InitGlobalTaskManager` 改造**:
```go
// 旧:
func InitGlobalTaskManager(cfg config.TasksConfig, obsBridge *observability.Bridge) {
    GlobalTaskManager = NewTaskManagerFromConfig(cfg, obsBridge)
}

// 新:
func NewTaskManagerFromConfigFunc(cfg config.TasksConfig, obsBridge *observability.Bridge) *TaskManager {
    return NewTaskManagerFromConfig(cfg, obsBridge)
}
```

或保留 `InitGlobalTaskManager` 名字但返回 `*TaskManager` (语义更清楚):
```go
func NewTaskManagerFromConfigFunc(cfg config.TasksConfig, obsBridge *observability.Bridge) *TaskManager {
    return NewTaskManagerFromConfig(cfg, obsBridge)
}
```

**决策**: 重命名为 `NewTaskManagerFromConfig` 已经存在, 改 factory function 名字会污染 API。**保留** `InitGlobalTaskManager` 名字但改返回 `*TaskManager`, 标注 deprecated (S6 后删)。**6+ caller 改用 `workmodel.NewTaskManagerFromConfig` 直接构造**。

### 2.5 Sub-commit 5: `freefork.SetGlobalForker` 删除 (包内)

**改动文件**:
- `internal/layers/multiagent/provision/freefork/wire.go` — 删除 var + setter
- `internal/layers/multiagent/multi_agent.go:34` — `freefork.SetGlobalForker(...)` 删除 (由 `WireMultiAgent` 接受 Forker 改 return Forker)
- `internal/layers/multiagent/provision/freefork/freefork_injection.go:34` — `freeforkGlobalFunc` 改接受 `Forker` 参数
- `internal/bootstrap/delegate.go` (或新文件) — `WireMultiAgent(...)` 改返回 `freefork.Forker` 给 caller 显式持有

**注入方式**: `WireMultiAgent` 函数签名变 (return Forker), caller 显式持有并传入 `freeforkGlobalFunc`。

**特殊处理**: `freeforkGlobalFunc` 原本签名是 `func(...)`, 改 `func(f freefork.Forker, ...)`。所有调用点同步加 Forker 参数。

---

## 3. 风险评估

### 3.1 编译错误风险 (H)

- **原因**: 5 个 global 跨 12+ 文件引用, 删除后 5 个 sub-commit 任一阶段都可能漏改 caller 导致编译失败
- **缓解**: 每个 sub-commit 完成后 `go build ./...` + `go test -race ./...` 必须 100% 绿再开下一个 sub-commit

### 3.2 EngineDeps 扩字段 (H) — Sub-commit 3

- **原因**: `EngineDeps.SessionCommandQueue` 字段已存在, 但如果现有 caller 没正确传递会编译失败
- **缓解**: 子步骤 1.1 `go build ./...` 必须先通过 1 caller 改造; 子步骤 1.2 再扩 5 caller

### 3.3 Orchestrator 字段 (H) — Sub-commit 4

- **原因**: `Orchestrator.tasks` 字段新增涉及 `NewOrchestrator` ctor 改签名, 6+ 调用点同步改
- **缓解**: 同 3.1, 编译优先

### 3.4 test 文件的 `defer reset(global)` 反模式 (M)

- **原因**: 3 个 test 文件用 `t.Cleanup(func() { SetGlobalXxx(prev) })` 模式, 删除 global 后需重写
- **缓解**: 测试改用 ctor 注入 (模式更干净, 同时消除反模式)

### 3.5 灰度 (L) — 父 design §2.8 已 lock

- **原因**: 阶段 2 是最终态, 不存在 "v0.5 / v0.9" 灰度
- **缓解**: devrix binary 新版 100% 替换; 验证以 `go test -race ./...` + 手动 IM 验证为准

---

## 4. 验收标准重述

| ID | 父 change AC | 本 change 状态 |
|----|--------------|---------------|
| AC4 | 6+ global var 零引用 | **PARTIAL → PASS** (PR #63 删 3 + 本 change 删 5) |
| AC14 | SetGlobalXxx API 全删 | **PARTIAL → PASS** (5 setter 全删) |
| 其他 20 AC | — | 状态保持 (PR #63 阶段 2c 已固化) |

**No new ACs** — 父 change 的 22 AC 是权威, 本 change 完成后两者由 PARTIAL 转 PASS。

---

## 5. 任务拆解

详见 `tasks.md` (W1-W5, 按 sub-commit 组织, 每个 W 独立可编译 + 独立测试可绿)。
