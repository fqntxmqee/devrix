# D7 Orchestration Span 注册表

**Domain:** D7 Orchestration
**Version:** 2.4.0
**Status:** Active (2026-06-22)
**Domain SoT:** `d7-domain.md`
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

> **Trace 树 / Span↔T / P0 Runbook：** 见 `observability-guide.md` §1–§7（本文仅登记 operation 名，不展开流程）。

---

## 跨域调用语义（DM-020）

D7 是 Turn 编排 Owner：**调用方**是 D7，**被调用方**是 D2 / D3 / D4。

| 步骤 | 调用关系 | 说明 |
|------|----------|------|
| 入站 | D1 → D7 | Gateway 路由到 `ProcessMessage` |
| 取上下文 | **D7 → D2** | Prepare / ToolRound / Persist（D2 不主动调 D3/D4） |
| 简单任务 | **D7 → D3** | FastPath Turn：`LLM_Invoke` → `D3_LLM_Stream` |
| 复杂任务 | **D7 → D4** | OrchestratePath：Wave 调度 Worker → `D4_Agent_Run`（不经 D3 Turn 主循环） |

**注意：** D2 的 `Query_Loop_*` span 属于 Legacy `engine.Process` 路径；DM-020 Turn 路径中 D7 直接持有 LLM 调用权，Jaeger 中 D3 应挂在 `D7_Orchestration_LLM_Invoke` 下，而非 D2 Query Loop。

---

## Operations

### Orchestrator — Session / Turn（D7-S2，7 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Session_Process` | INTERNAL | orchestrator | 2.2.0 | session_id, message.len, orchestration.route |
| `D7_Orchestration_Intent_Classify` | INTERNAL | orchestrator | 2.2.0 | orchestration.intent.*, orchestration.classify.source |
| `D7_Orchestration_Turn_Run` | INTERNAL | orchestrator | 2.2.0 | session_id, turn.scope, turn.max_turns |
| `D7_Orchestration_Turn_Iteration` | INTERNAL | orchestrator | 2.2.0 | session_id, turn.index |
| `D7_Orchestration_LLM_Invoke` | CLIENT | orchestrator | 2.2.0 | session_id, turn.index, llm.purpose |
| `D7_Orchestration_Orchestrate_Run` | INTERNAL | orchestrator | 2.2.0 | session_id |
| `D7_Orchestration_Wave_Schedule` | INTERNAL | orchestrator | 2.1.0 | session_id, wave_id |
| `D7_Orchestration_Wave_Task_Execute` | INTERNAL | orchestrator | 2.1.0 | task_id, worker_type |
| `D7_Orchestration_Flow_Event_Publish` | INTERNAL | orchestrator | 2.1.0 | event_kind, worker_id, source |

---

## Trace Tree

### Fast 路径（D7 → D2 上下文 → D7 → D3）

```
D1_Capture_Message_Receive
└── D1_Dispatch_Route
    └── D7_Orchestration_Session_Process  (route=fast)
        ├── D7_Orchestration_Intent_Classify
        └── D7_Orchestration_Turn_Run
            ├── D2_Context_Process          ← D7→D2 prepare (context.caller=d7)
            └── D7_Orchestration_Turn_Iteration
                ├── D7_Orchestration_LLM_Invoke  ← D7→D3
                │   └── D3_LLM_Stream
                └── D2_Tool_Execute_Single     ← D7→D2 tools (optional)
```

### Orchestrate 路径（D7 → D2 上下文 → D7 → D4）

```
D1_Capture_Message_Receive
└── D1_Dispatch_Route
    └── D7_Orchestration_Session_Process  (route=orchestrate)
        ├── D7_Orchestration_Intent_Classify
        └── D7_Orchestration_Orchestrate_Run
            └── D7_Orchestration_Wave_Schedule
                └── D7_Orchestration_Wave_Task_Execute
                    ├── D2_Context_Process   ← Wave ContextResolver (D7→D2)
                    └── D4_Agent_Run         ← D7→D4 Worker
```

---

## 命名规范

| 场景 | 格式 | 示例 |
|------|------|------|
| Span Operation | `D7_S{scenario}_{Activity}_{Function}` | `D7_Orchestration_LLM_Invoke` |
| D2 被 D7 调用 | 复用 D2 span + `context.caller=d7` | `D2_Context_Process` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- D2 Context Engine：`openspec/specs/d2-context-engine/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
- **Span↔T / Runbook：** `observability-guide.md`

---

## D7-S6-A14 硬化对 Span 的影响（DM-20260622-001）

> 本节登记 A14 修复对既有 span 行为的局部影响。**不新增 span operation**，仅说明 5 fix 如何改变既有 span 的子路径或 emit 行为。

| 既有 span | 受 A14 哪个 fix 影响 | 行为变化 |
|-----------|----------------------|---------|
| `D7_Orchestration_Wave_Task_Execute` | **A4**（`AllowAndRegister` 原子化） | 子路径由 `ConflictGuard.Allow` → `Register` 两段式合并为单 `AllowAndRegister`；span 内部 mutex 锁区间缩短 1 个（`g.mu` 仅持一次），子属性 `conflict.allowed=true/false` 不变 |
| `D7_Orchestration_Session_Process`（route=command） | **A5**（`CommandHandler.emit` select-default） | `command_reply` → `text` → `complete` 三事件 emit 在 consumer stall 时可能丢包；`slog.Warn("command_handler: out channel full, drop event", ...)` 出现频率可作为 backpressure 信号 |
| `D7_Orchestration_Wave_Schedule` | **A3**（`markWaveDone` 释放 state.cancels/handles） | span 结束前 `state.cancels = nil` + `state.handles = make(map)`，但 span 自身不变；D5 GC 指标可观察 wave state 内存 footprint 收敛 |
| 全部 7 ops | **A1**（metric plural） | `metric=worker_panics` / `metric=dispatch_loop_wakeups`（plural，与 spec 一致）；旧 singular key（`worker_panic` / `dispatch_wakeup`）从此 0 流量 |

**未变：** span operation 名、kind、component、key attributes 全部保持；A14 不引入新 span，纯粹对既有 span 的子路径与日志做收敛。
