# D7 Orchestration Span 注册表

**Domain:** D7 Orchestration
**Version:** 2.2.0
**Status:** Active (2026-06-16)
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

---

## 跨域调用语义（DM-020）

D7 是 Turn 编排 Owner：**调用方**是 D7，**被调用方**是 D2 / D3 / D4。

| 步骤 | 调用关系 | 说明 |
|------|----------|------|
| 入站 | D1 → D7 | Gateway 路由到 `ProcessMessage` |
| 取上下文 | **D7 → D2** | Prepare / ToolRound / Persist（D2 不主动调 D3/D4） |
| 简单任务 | **D7 → D3** | FastPath Turn：`LLM_Invoke` → `D3_S3_LLM_Stream` |
| 复杂任务 | **D7 → D4** | OrchestratePath：Wave 调度 Worker → `D4_S4_Agent_Run`（不经 D3 Turn 主循环） |

**注意：** D2 的 `Query_Loop_*` span 属于 Legacy `engine.Process` 路径；DM-020 Turn 路径中 D7 直接持有 LLM 调用权，Jaeger 中 D3 应挂在 `D7_S2_Orchestration_LLM_Invoke` 下，而非 D2 Query Loop。

---

## Operations

### Orchestrator — Session / Turn（D7-S2，7 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_S2_Orchestration_Session_Process` | INTERNAL | orchestrator | 2.2.0 | session_id, message.len, orchestration.route |
| `D7_S2_Orchestration_Intent_Classify` | INTERNAL | orchestrator | 2.2.0 | orchestration.intent.*, orchestration.classify.source |
| `D7_S2_Orchestration_Turn_Run` | INTERNAL | orchestrator | 2.2.0 | session_id, turn.scope, turn.max_turns |
| `D7_S2_Orchestration_Turn_Iteration` | INTERNAL | orchestrator | 2.2.0 | session_id, turn.index |
| `D7_S2_Orchestration_LLM_Invoke` | CLIENT | orchestrator | 2.2.0 | session_id, turn.index, llm.purpose |
| `D7_S2_Orchestration_Orchestrate_Run` | INTERNAL | orchestrator | 2.2.0 | session_id |
| `D7_S3_Orchestration_Wave_Schedule` | INTERNAL | orchestrator | 2.1.0 | session_id, wave_id |
| `D7_S3_Orchestration_Wave_Task_Execute` | INTERNAL | orchestrator | 2.1.0 | task_id, worker_type |
| `D7_S3_Orchestration_Flow_Event_Publish` | INTERNAL | orchestrator | 2.1.0 | event_kind, worker_id, source |

---

## Trace Tree

### Fast 路径（D7 → D2 上下文 → D7 → D3）

```
D1_S13_Capture_Message_Receive
└── D1_S13_Dispatch_Route
    └── D7_S2_Orchestration_Session_Process  (route=fast)
        ├── D7_S2_Orchestration_Intent_Classify
        └── D7_S2_Orchestration_Turn_Run
            ├── D2_S2_Context_Process          ← D7→D2 prepare (context.caller=d7)
            └── D7_S2_Orchestration_Turn_Iteration
                ├── D7_S2_Orchestration_LLM_Invoke  ← D7→D3
                │   └── D3_S3_LLM_Stream
                └── D2_S5_Tool_Execute_Single     ← D7→D2 tools (optional)
```

### Orchestrate 路径（D7 → D2 上下文 → D7 → D4）

```
D1_S13_Capture_Message_Receive
└── D1_S13_Dispatch_Route
    └── D7_S2_Orchestration_Session_Process  (route=orchestrate)
        ├── D7_S2_Orchestration_Intent_Classify
        └── D7_S2_Orchestration_Orchestrate_Run
            └── D7_S3_Orchestration_Wave_Schedule
                └── D7_S3_Orchestration_Wave_Task_Execute
                    ├── D2_S2_Context_Process   ← Wave ContextResolver (D7→D2)
                    └── D4_S4_Agent_Run         ← D7→D4 Worker
```

---

## 命名规范

| 场景 | 格式 | 示例 |
|------|------|------|
| Span Operation | `D7_S{scenario}_{Activity}_{Function}` | `D7_S2_Orchestration_LLM_Invoke` |
| D2 被 D7 调用 | 复用 D2 span + `context.caller=d7` | `D2_S2_Context_Process` |

---

## 关联文档

- D5 全局 Trace Tree：`openspec/specs/d5-observability/span-registry.md`
- D2 Context Engine：`openspec/specs/d2-context-engine/span-registry.md`
- 全局 Spans 索引：`openspec/spans-registry.md`
