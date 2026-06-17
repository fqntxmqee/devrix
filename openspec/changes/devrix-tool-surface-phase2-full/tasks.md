# Tasks: devrix-tool-surface-phase2-full

**Change ID:** devrix-tool-surface-phase2-full
**Demand ID:** DM-20260617-008
**Status:** S4_Implementation
**估算参考（仅供参考，非承诺）:** 5 sub-commit × 0.5-1.0 day, ~+150 LOC (净减少 ~30 LOC)

---

> **DSAFT Activity 一览**
>
> 本 change 是父 change (DM-20260617-007) 的执行级 followup, **不引入新
> Activity 节点**。每个 sub-commit 沿用父 change 的活动节点 (D1-S2-A02 /
> D2-S6-A03 / D4-S11-A02 / D7-S5-A01) 完成 global 注入化。
>
> **5 sub-commit 顺序**: 按 caller 数量从小到大 + 风险从低到高执行
> (transcript → sessionqueue → freefork → flow → workmodel),
> 任一 sub-commit 失败可独立 revert 不影响其他 4 个。

## W1 — Sub-commit 1: 删除 `transcript.GlobalWriter` (1 prod caller + 2 test)

### W1.1 — `Gateway` struct 加 `Writer` 字段

- **文件 1:** `internal/layers/communication/capture/gateway.go` (MODIFY, +6 行)
  - `CommunicationGateway` struct 加 `writer *transcript.Writer` 字段
  - `NewCommunicationGateway(...)` 增最后一个参数 `writer *transcript.Writer`
  - `ExpireSession` (line ~811) 改 `g.writer.Append(...)` 替代 `transcript.GlobalWriter()`
- **依赖:** 无
- **AC:** AC4 (部分), AC14 (部分)
- **T:** TOOL-SURFACE-1-T15
- **估时参考:** 30 min

### W1.2 — 2 个 bootstrap caller 改 ctor 注入

- **文件 1:** `internal/bootstrap/context_engine.go` (MODIFY, +1 行)
  - `NewCommunicationGateway(..., tw)` 注入 writer (从 `ctxCfg.MainTranscript` 派生)
- **文件 2:** `internal/bootstrap/context_engine_builder.go` (MODIFY, +1 行)
  - 同上
- **依赖:** W1.1
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T15
- **估时参考:** 15 min

### W1.3 — 测试改写 (`defer reset(global)` 反模式 → ctor 注入)

- **文件 1:** `internal/layers/communication/capture/session_store_transcript_test.go` (MODIFY, -8 / +4 行)
  - 2 处 `t.Cleanup(func() { transcript.SetGlobalWriter(prevW) })` 改用 `NewCommunicationGateway(..., tw)` 注入
  - 删除 2 处 `prevW := transcript.GlobalWriter()` 备份代码
- **依赖:** W1.2
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T15
- **估时参考:** 15 min

### W1.4 — 删除 `transcript.GlobalWriter` var + setter + `Append` 全局快捷方法

- **文件 1:** `internal/layers/communication/capture/transcript/wire.go` (MODIFY, -38 行)
  - 删除 `globalMu`, `globalW`, `SetGlobalWriter`, `GlobalWriter`, `Append` (5 entity)
  - 文件可保留 `sync` import (如果有其他用途) 或删
- **依赖:** W1.3
- **AC:** AC4, AC14
- **T:** TOOL-SURFACE-1-T15
- **验证:** `git grep -n "GlobalWriter\|SetGlobalWriter" internal/` 仅命中注释
- **估时参考:** 15 min

### W1 验证

```bash
go build ./...
go test -race ./internal/layers/communication/... ./internal/bootstrap/...
git grep -n "GlobalWriter\|SetGlobalWriter" internal/
# 期望: 仅命中注释 (e.g. "Default is NoOp" 之类), 无 production-code 引用
```

**Sub-commit 1 总估时:** ~75 min (~0.2 day)

---

## W2 — Sub-commit 2: 删除 `flow.GlobalHub` (4 prod caller + 1 test)

### W2.1 — `delegatetools.Deps` 加 `Hub` 字段, 3 caller 走 deps

- **文件 1:** `internal/layers/orchestration/delegatetools/delegate_tools.go` (MODIFY, +3 行)
  - `Deps` struct 加 `Hub contracts.ExecutionFlowHub` 字段
  - `flow.GlobalHub.Snapshot(sc.SessionID)` 改 `deps.Hub.Snapshot(sc.SessionID)`
- **文件 2:** `internal/layers/orchestration/delegatetools/subquery_fallback.go` (MODIFY, +2 行)
  - `deps.FlowReporter == nil && flow.GlobalHub != nil` 改 `deps.FlowReporter == nil && deps.Hub != nil`
  - `hubspoke.NewFlowReporter(flow.GlobalHub)` 改 `hubspoke.NewFlowReporter(deps.Hub)`
- **文件 3:** `internal/layers/orchestration/hubspoke/dispatch.go` (MODIFY, +1 行)
  - `hub = flow.GlobalHub` 改 `hub = deps.Hub` (Deps 已有 Hub 字段)
- **依赖:** 无 (字段已存在, 阶段 1 完成)
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T16
- **估时参考:** 30 min

### W2.2 — 2 个 bootstrap caller 改不调 SetGlobalHub

- **文件 1:** `internal/bootstrap/execution_flow.go` (MODIFY, -2 行)
  - 删除 `flow.SetGlobalHub(nil)` 和 `flow.SetGlobalHub(hub)` 调用
  - 改 `wireFlowEvent` 函数把 hub 注入到下游 caller
- **文件 2:** `internal/bootstrap/delegate.go` (MODIFY, -1 / +1 行)
  - 注释 `nil, // uses flow.GlobalHub by default` 改 `nil, // uses NoOp (hub wired via deps.Hub)`
- **依赖:** W2.1
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T16
- **估时参考:** 15 min

### W2.3 — 测试改写

- **文件 1:** `internal/layers/orchestration/delegatetools/subquery_fallback_test.go` (MODIFY, -4 / +3 行)
  - `prev := flow.GlobalHub; flow.SetGlobalHub(hub); defer flow.SetGlobalHub(prev)` 改 `deps := delegatetools.Deps{Hub: hub, ...}`
- **文件 2:** `internal/layers/orchestration/hubspoke/hubspoke_test.go` (MODIFY, -1 / +1 行)
  - 注释 `nil, // hub → defaults to flow.GlobalHub` 改 `nil, // hub → defaults to NoOp`
- **依赖:** W2.2
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T16
- **估时参考:** 15 min

### W2.4 — 删除 `flow.GlobalHub` var + setter

- **文件 1:** `internal/layers/orchestration/flow/hub.go` (MODIFY, -10 行)
  - 删除 `var GlobalHub contracts.ExecutionFlowHub = contracts.NoOpExecutionFlowHub{}` (line 70)
  - 删除 `SetGlobalHub(...)` 函数 (line 73-79)
- **依赖:** W2.3
- **AC:** AC4, AC14
- **T:** TOOL-SURFACE-1-T16
- **验证:** `git grep -n "flow.GlobalHub\|flow.SetGlobalHub" internal/` 仅命中注释
- **估时参考:** 15 min

### W2 验证

```bash
go build ./...
go test -race ./internal/layers/orchestration/... ./internal/bootstrap/...
git grep -n "GlobalHub\|SetGlobalHub" internal/layers/orchestration/flow/
# 期望: 0 命中
```

**Sub-commit 2 总估时:** ~75 min (~0.2 day)

---

## W3 — Sub-commit 3: 删除 `sessionqueue.GlobalSessionQueue` (5 prod caller)

### W3.1 — 5 caller 改 NewSessionQueue() 局部实例

- **文件 1:** `internal/bootstrap/context_engine.go` (MODIFY, +1 行)
  - `SessionCommandQueue: sessionqueue.GlobalSessionQueue` 改 `SessionCommandQueue: sessionqueue.NewSessionQueue()`
- **文件 2:** `internal/bootstrap/context_engine_builder.go` (MODIFY, +1 行)
  - 同上
- **文件 3:** `internal/bootstrap/wire_wave.go` (MODIFY, +1 行)
  - 同上 (在 `SessionCommandQueue: sessionqueue.NewSessionQueue()`)
- **文件 4:** `internal/bootstrap/execution_flow.go` (MODIFY, +1 行)
  - `Queue: sessionqueue.GlobalSessionQueue` 改 `Queue: sessionqueue.NewSessionQueue()`
- **文件 5:** `internal/layers/orchestration/flow/hub.go` (MODIFY, +1 行)
  - `q = sessionqueue.GlobalSessionQueue` 改 `q = sessionqueue.NewSessionQueue()` (deps.Queue nil 时)
- **依赖:** 无
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T17
- **估时参考:** 30 min

### W3.2 — 删除 `sessionqueue.GlobalSessionQueue` var

- **文件 1:** `internal/layers/orchestration/sessionqueue/session_queue.go` (MODIFY, -1 行)
  - 删除 `var GlobalSessionQueue = NewSessionQueue()` (line 20)
- **依赖:** W3.1
- **AC:** AC4, AC14
- **T:** TOOL-SURFACE-1-T17
- **验证:** `git grep -n "GlobalSessionQueue" internal/` 仅命中注释
- **估时参考:** 5 min

### W3 验证

```bash
go build ./...
go test -race ./internal/layers/orchestration/sessionqueue/... ./internal/bootstrap/...
git grep -n "GlobalSessionQueue" internal/
# 期望: 0 命中
```

**Sub-commit 3 总估时:** ~35 min (~0.1 day)

---

## W4 — Sub-commit 4: 删除 `workmodel.GlobalTaskManager` (6+ prod caller + 2 test)

### W4.1 — `InitGlobalTaskManager` 改返回 `*TaskManager` (非写 global)

- **文件 1:** `internal/layers/orchestration/workmodel/task_manager.go` (MODIFY, +8 / -4 行)
  - 函数签名改 `func NewTaskManagerFromConfigFunc(cfg config.TasksConfig, obsBridge *observability.Bridge) *TaskManager`
  - 内部实现 `return NewTaskManagerFromConfig(cfg, obsBridge)`
  - 保留旧名 `InitGlobalTaskManager` 作为 deprecated 包装器返回 `*TaskManager`, 加 `// Deprecated: use NewTaskManagerFromConfig directly` 注释
- **依赖:** 无
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T18
- **估时参考:** 15 min

### W4.2 — `Orchestrator` struct 加 `tasks` 字段 + `NewOrchestrator` 增参数

- **文件 1:** `internal/layers/orchestration/coordinator/orchestrator.go` (MODIFY, +5 行)
  - `Orchestrator` struct 加 `tasks *workmodel.TaskManager` 字段
  - `NewOrchestrator(...)` ctor 增 `tasks *workmodel.TaskManager` 参数
  - `o.tasks = tasks` 赋值
- **文件 2:** `internal/layers/orchestration/coordinator/command_handler.go` (MODIFY, +3 行)
  - `CommandHandler` struct 加 `tasks *workmodel.TaskManager` 字段
  - `NewCommandHandler(...)` 增 tasks 参数
  - `h.tasks = tasks` 赋值
- **依赖:** W4.1
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T18
- **估时参考:** 30 min

### W4.3 — 6 caller 改注入 (bootstrap + coordinator + cli + delegatetools)

- **文件 1:** `internal/bootstrap/context_engine.go` (MODIFY, +2 行)
  - `tm := workmodel.NewTaskManagerFromConfig(ctxCfg.Tasks, obsBridge)` 局部
  - `RegisterTaskTools(toolReg, ctxCfg, tm)` 注入
- **文件 2:** `internal/bootstrap/context_engine_builder.go` (MODIFY, +2 行)
  - 同上 (用 `b.ctxCfg.Tasks`, `b.obsBridge`)
- **文件 3:** `internal/bootstrap/execution_flow.go` (MODIFY, +1 行)
  - `Tasks: tm` (局部 tm)
- **文件 4:** `internal/bootstrap/wire_coordinator.go` (MODIFY, +2 行)
  - `tm := workmodel.NewTaskManagerFromConfig(...)` 局部
  - `coordinator.NewLocalWorkModel(tm)` 注入
- **文件 5:** `internal/layers/orchestration/coordinator/command_handler.go:156,167` (MODIFY, +1 行)
  - `cli := workmodel.NewCLICommands(h.tasks)` 改用 `h.tasks` (字段)
- **文件 6:** `internal/layers/orchestration/coordinator/orchestrator.go:150,416` (MODIFY, +1 行)
  - 同上 (用 `o.tasks`)
- **文件 7:** `internal/layers/orchestration/delegatetools/delegate_tools.go:171,181` (MODIFY, +2 行)
  - `Deps` struct 加 `Tasks *workmodel.TaskManager` 字段
  - `deps.Tasks.Create(...)` 替代 `workmodel.GlobalTaskManager.Create(...)`
- **文件 8:** `internal/layers/communication/channel/adapters/cli.go:56` (MODIFY, +1 行)
  - `NewCLIAdapter(...)` ctor 增 tasks 参数
- **依赖:** W4.2
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T18
- **估时参考:** 60 min

### W4.4 — 删除 `workmodel.GlobalTaskManager` var + `init()`

- **文件 1:** `internal/layers/orchestration/workmodel/task_manager.go` (MODIFY, -7 行)
  - 删除 `var GlobalTaskManager *TaskManager` (line 55)
  - 删除 `func init() { GlobalTaskManager = NewTaskManager() }` (line 57-59)
- **依赖:** W4.3
- **AC:** AC4, AC14
- **T:** TOOL-SURFACE-1-T18
- **验证:** `git grep -n "GlobalTaskManager" internal/` 仅命中注释
- **估时参考:** 5 min

### W4 验证

```bash
go build ./...
go test -race ./internal/layers/orchestration/workmodel/... ./internal/layers/orchestration/coordinator/... ./internal/layers/orchestration/delegatetools/... ./internal/layers/communication/channel/adapters/... ./internal/bootstrap/...
git grep -n "GlobalTaskManager" internal/
# 期望: 0 命中
```

**Sub-commit 4 总估时:** ~110 min (~0.3 day)

---

## W5 — Sub-commit 5: 删除 `freefork.SetGlobalForker` (1 prod caller)

### W5.1 — `freeforkGlobalFunc` 改接受 Forker 参数

- **文件 1:** `internal/layers/multiagent/provision/freefork/freefork_injection.go` (MODIFY, +2 行)
  - `freeforkGlobalFunc` 函数签名增 `f freefork.Forker` 参数
  - 内部 `freefork.GlobalForker()` 改用参数 `f`
- **文件 2:** `internal/layers/multiagent/multi_agent.go:34` (MODIFY, -1 / +1 行)
  - `freefork.SetGlobalForker(f)` 删除
  - 改返回 `f` 给 caller (隐式 through WireMultiAgent signature)
- **文件 3:** `internal/layers/multiagent/provision/freefork/wire.go` (MODIFY, -22 行)
  - 删除 `globalForkerMu`, `globalForker`, `SetGlobalForker`, `GlobalForker` (4 entity)
- **依赖:** 无
- **AC:** AC4, AC14
- **T:** TOOL-SURFACE-1-T19
- **验证:** `git grep -n "SetGlobalForker\|GlobalForker" internal/layers/multiagent/provision/freefork/` 仅命中注释
- **估时参考:** 30 min

### W5.2 — `WireMultiAgent` 改返回 Forker 给 caller 持有

- **文件 1:** `internal/layers/multiagent/multi_agent.go` (MODIFY, +2 行)
  - `WireMultiAgent(...)` 函数签名增返回 `freefork.Forker`
  - caller 显式持有 `forker` 变量
- **文件 2:** `internal/bootstrap/delegate.go` (MODIFY, +1 行)
  - 接 `_, forker := multiagent.WireMultiAgent(...)` 并把 `forker` 传到下游
- **依赖:** W5.1
- **AC:** AC4 (部分)
- **T:** TOOL-SURFACE-1-T19
- **估时参考:** 15 min

### W5 验证

```bash
go build ./...
go test -race ./internal/layers/multiagent/... ./internal/layers/multiagent/provision/freefork/... ./internal/bootstrap/...
git grep -n "SetGlobalForker\|GlobalForker" internal/
# 期望: 0 命中
```

**Sub-commit 5 总估时:** ~45 min (~0.1 day)

---

## W6 — 全量验证 (5 sub-commit 全部完成后)

### W6.1 — 静态验证: 5 global + 5 setter 全删

```bash
git grep -n "SetGlobal\|GlobalSessionQueue\|GlobalTaskManager\|GlobalHub\|GlobalWriter\|GlobalForker" internal/
# 期望: 仅命中注释 (e.g. "// used to be process-global" 历史说明), 无 production-code 引用
```

- **T:** TOOL-SURFACE-1-T20
- **估时参考:** 5 min

### W6.2 — 动态验证: go test -race 全量绿

```bash
go test -race ./...
# 期望: 100% PASS, 0 race condition
go vet ./...
# 期望: 0 warning
```

- **T:** TOOL-SURFACE-1-T21
- **估时参考:** 5 min (build time)

### W6.3 — 父 AC 重计: AC4 + AC14 PARTIAL → PASS

- 更新 `openspec/archive/2026-06-17-devrix-tool-surface-contract/acceptance-report.md` 状态
  - AC4: `PARTIAL → PASS` (12 → 3 → 0 global var)
  - AC14: `PARTIAL → PASS` (5 setter 全删)
- 更新 `openspec/demand-archive-index.md` DM-20260617-007 row
- **估时参考:** 10 min

---

## 总估时

| Sub-commit | 估时 | 累计 |
|-----------|------|------|
| W1 transcript | 75 min | 0.2 day |
| W2 flow | 75 min | 0.4 day |
| W3 sessionqueue | 35 min | 0.5 day |
| W4 workmodel | 110 min | 0.8 day |
| W5 freefork | 45 min | 0.9 day |
| W6 verify | 20 min | 0.95 day |

**总计**: ~0.95 day (1 day 完成) + S6 归档 0.5 day = 1.5 day total
父 design §2.8 估时 2-3 天, 本 tasks 略乐观 (基于 PR #63 已建立的 surface 化基础)。
