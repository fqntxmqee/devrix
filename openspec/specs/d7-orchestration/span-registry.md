# D7 Orchestration Span 注册表

**Domain:** D7 Orchestration
**Version:** 4.0.0
**Status:** Active (2026-06-26)
**Domain SoT:** `d7-domain.md`
**Canonical Source:** `internal/layers/observability/instrument/telemetry/names.go` · `internal/layers/observability/diagnose/coverage/registry.go`

> **Trace 树 / Span↔T / P0 Runbook：** 见 `observability-guide.md` §1–§7（本文仅登记 operation 名，不展开流程）。
>
> **v6.0.0 6 S 精简 + 5 个新 Span 落地（2026-06-26）：** 14 S → 6 S Span 重归类 + 5 个新 P0/P1 Span（channel.route / memory.persist / system.anomaly_detect / taskgraph.synthesize / executor.select），共 23 ops + 9 sessionSpan attributes。详见 §Operations 与 §SessionSpan Attributes。

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

### 6 S Span 归类（v6.0.0，2026-06-26 落地）

> 14 S → 6 S 后 span operation 重新归类到 6 个 S + 横切 Hardening。共 **23 ops**（18 旧 + **5 新 P0/P1**）+ 9 sessionSpan attributes。

### D7-S1 WorkModel Operations（State Authority，2 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_WorkItem_Manage` | INTERNAL | workmodel | 4.0.0 | session_id, workitem.id, workitem.kind, workitem.status |
| `D7_Orchestration_Uncertainty_Update` | INTERNAL | workmodel | 4.0.0 | session_id, uncertainty.coord_kind, prior.adaptive_kind, prior.beta_alpha, prior.beta_beta |

### D7-S2 SessionOrchestrator Operations（Mediator + Turn Leader + Error Recovery，6 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Session_Process` | INTERNAL | orchestrator | 2.2.0 | session_id, message.len, orchestration.route |
| `D7_Orchestration_Turn_Run` | INTERNAL | orchestrator | 2.2.0 | session_id, turn.scope, turn.max_turns |
| `D7_Orchestration_Turn_Iteration` | INTERNAL | orchestrator | 2.2.0 | session_id, turn.index |
| `D7_Orchestration_LLM_Invoke` | CLIENT | orchestrator | 2.2.0 | session_id, turn.index, llm.purpose |
| `D7_Orchestration_Resume_Session` | INTERNAL | session | 3.0.0 | session_id, resume.decision, resume.circuit_level, resume.user_choice |
| `D7_Orchestration_Escape_Engine_Run` | INTERNAL | session | 3.0.0 | session_id, circuit_level, escape.signal_kind |

### D7-S3 WaveScheduler Operations（Mechanism Designer，3 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Wave_Schedule` | INTERNAL | wavescheduler | 2.1.0 | session_id, wave_id |
| `D7_Orchestration_Wave_Task_Execute` | INTERNAL | wavescheduler | 2.1.0 | task_id, worker_type |
| `D7_Orchestration_Executor_Select` ⭐新 P1 | INTERNAL | wavescheduler | 4.0.0 | session_id, candidates_count, selected_kind, score, policy |

### D7-S4 ExecutionFlow + Verify Operations（Costly Signaler + Certifier，5 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Flow_Event_Publish` | INTERNAL | executionflow | 2.1.0 | event_kind, worker_id, source |
| `D7_Orchestration_Flow_Hub_Aggregate` | INTERNAL | executionflow | 4.0.0 | session_id, workplan_id, flowevents.count |
| `D7_Orchestration_Verify_Verdict` | INTERNAL | verify | 3.0.0 | session_id, verdict.kind, exit_reason, source_artifact_id |
| `D7_Orchestration_Verify_AutoClose` | INTERNAL | verify | 3.0.0 | session_id, auto_close.rule, track_mode, exit_reason |
| `D7_Orchestration_System_Anomaly_Detect` ⭐新 P0 | INTERNAL | verify | 4.0.0 | session_id, anomaly.kind, severity, threshold, evidence_id |

### D7-S5 DecisionPlanning + Observe Operations（Information Producer + Quantizer，4 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Intent_Classify` | INTERNAL | decisionplanning | 2.2.0 | orchestration.intent.*, orchestration.classify.source |
| `D7_Orchestration_TaskGraph_Synthesize` ⭐新 P1 | INTERNAL | decisionplanning | 4.0.0 | session_id, taskgraph.node_count, taskgraph.edge_count, taskgraph.dag_depth, taskgraph.cycle_detected |
| `D7_Orchestration_Observe_Quantize` | INTERNAL | observe | 3.0.0 | session_id, observation.kind, observation.strength, uncertainty.coord_kind |
| `D7_Orchestration_Plan_Generate` | INTERNAL | observe | 3.0.0 | session_id, plan.kind, plan.strength, source_observation_ids |

### D7-S6 MUPS Pipeline Operations（Pipeline Coordinator + Memory Curator，3 ops）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Channel_Route` ⭐新 P0 | INTERNAL | mups | 4.0.0 | session_id, channel.kind, plan.kind, score, fallback |
| `D7_Orchestration_Memory_Persist` ⭐新 P0 | INTERNAL | mups | 4.0.0 | session_id, channel, asset.kind, ttl_ms, payload_size |
| `D7_Orchestration_Learn_Asset` | INTERNAL | mups | 3.0.0 | session_id, asset.class, asset.ttl, reputation.beta_alpha, reputation.beta_beta |

### Cross-cutting Hardening Operations（Discipline Keeper，非 S）

| Operation | Kind | Component | Since | Key Attributes |
|-----------|------|-----------|-------|----------------|
| `D7_Orchestration_Metrics_Emit` | INTERNAL | hardening | 4.0.0 | metric.name, metric.value, metric.kind |
| `D7_Orchestration_CircuitBreaker_Monitor` | INTERNAL | hardening | 4.0.0 | circuit_level, state.kind, transitions_count |

---

## SessionSpan Attributes（Phase 7 + V5.6 扩展，2026-06-25 落地）

> `D7_Orchestration_Session_Process` 父 span 注入 6 prior attributes（Phase 7）+ 3 resume attributes（V5.6），共 9 attributes。这些属性让 D5 dashboard 能直接通过 sessionSpan 字段过滤，无需进入子 span。

### 6 prior attributes（Phase 7, D7-S13-A49）

| Attribute | 取值 | 用途 |
|-----------|------|------|
| `prior.adaptive_kind` | `developer` / `operator` / `uniform` | 注入到 Observe 的先验类型 |
| `prior.beta_alpha` | float（α） | ReputationEvidence Beta 分布 α 参数 |
| `prior.beta_beta` | float（β） | ReputationEvidence Beta 分布 β 参数 |
| `prior.evidence_count` | int | 历史 ReputationEvidence 数量 |
| `prior.cycle_count` | int | Observe-Learner 闭环循环轮次 |
| `prior.last_update` | timestamp | 最近一次 ReputationEvidence 更新 |

### 3 resume attributes（V5.6, D7-S14-A52）

| Attribute | 取值 | 用途 |
|-----------|------|------|
| `resume.decision` | `fall_through` / `force_exited` / `aborted` | ResumeSession 决策路由 |
| `resume.circuit_level` | `L0`..`L5` | 触发 EscapeEngine 的 CircuitBreaker 层 |
| `resume.user_choice` | string | 用户原始 `/resume` 输入（审计追溯）|

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

## Cross-cutting Hardening 对 Span 的影响（DM-20260622-001 → v6.0.0 移入横切）

> 本节登记 A14 修复对既有 span 行为的局部影响。**不新增 span operation**，仅说明 5 fix 如何改变既有 span 的子路径或 emit 行为。v6.0.0 起 Hardening 不再占 S 位，改为 cross-cutting 角色。

| 既有 span | 受 Hardening 哪个 fix 影响 | 行为变化 |
|-----------|---------------------------|---------|
| `D7_Orchestration_Wave_Task_Execute` | **A4**（`AllowAndRegister` 原子化） | 子路径由 `ConflictGuard.Allow` → `Register` 两段式合并为单 `AllowAndRegister`；span 内部 mutex 锁区间缩短 1 个（`g.mu` 仅持一次），子属性 `conflict.allowed=true/false` 不变 |
| `D7_Orchestration_Session_Process`（route=command） | **A5**（`CommandHandler.emit` select-default） | `command_reply` → `text` → `complete` 三事件 emit 在 consumer stall 时可能丢包；`slog.Warn("command_handler: out channel full, drop event", ...)` 出现频率可作为 backpressure 信号 |
| `D7_Orchestration_Wave_Schedule` | **A3**（`markWaveDone` 释放 state.cancels/handles） | span 结束前 `state.cancels = nil` + `state.handles = make(map)`，但 span 自身不变；D5 GC 指标可观察 wave state 内存 footprint 收敛 |
| 全部 7 ops | **A1**（metric plural） | `metric=worker_panics` / `metric=dispatch_loop_wakeups`（plural，与 spec 一致）；旧 singular key（`worker_panic` / `dispatch_wakeup`）从此 0 流量 |

**未变：** span operation 名、kind、component、key attributes 全部保持；Hardening 不引入新 span，纯粹对既有 span 的子路径与日志做收敛。

---

## MUPS 5 节点管道 Trace 树（v6.0.0，6 S 归类）

### 5 节点管道完整 Trace（挂在 S5/S6/S4）

```text
D1_Capture_Message_Receive
└── D1_Dispatch_Route
    └── D7_Orchestration_Session_Process  {route=orchestrate}  ← D7-S2
        ├── D7_Orchestration_Intent_Classify                       ← D7-S5
        ├── D7_Orchestration_Observe_Quantize                      ← D7-S5 Observe 节点
        │   └── D7_Orchestration_TaskGraph_Synthesize  ⭐新 P1     ← D7-S5
        ├── D7_Orchestration_Plan_Generate                         ← D7-S5 Plan 节点
        ├── D7_Orchestration_Channel_Route  ⭐新 P0                ← D7-S6 通道选择
        ├── D7_Orchestration_Memory_Persist  ⭐新 P0               ← D7-S6 记忆持久化
        ├── D7_Orchestration_Verify_Verdict                        ← D7-S4 Verify 节点
        │   └── D7_Orchestration_System_Anomaly_Detect  ⭐新 P0    ← D7-S4 异常检测
        │   └── D7_Orchestration_Verify_AutoClose                  ← D7-S4 Auto-Close
        ├── D7_Orchestration_Executor_Select  ⭐新 P1               ← D7-S3 调度
        ├── D7_Orchestration_Learn_Asset                           ← D7-S6 Learn 节点
        └── (下轮) D7_Orchestration_Observe_Quantize                ← ReputationEvidence 注入
```

### EscapeEngine Trace 树（Phase v5，归 S2）

```text
D7_Orchestration_Session_Process  ← D7-S2
└── D7_Orchestration_Escape_Engine_Run  {circuit_level=L1..L5}    ← D7-S2 调度
    └── D7_Orchestration_Resume_Session  {resume.decision=...}     ← D7-S2 ResumeSession
```

### Cross-cutting Hardening Trace 树

```text
D7_Orchestration_Session_Process
└── D7_Orchestration_Metrics_Emit  ← Hardening（横切，metrics 汇总）
└── D7_Orchestration_CircuitBreaker_Monitor  ← Hardening（横切，熔断监控）
```

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 2.4.0 | 2026-06-22 | DM-20260622-001 D7 Metrics & Concurrency Hardening 对 span 子路径与日志的影响 |
| 3.0.0 | 2026-06-25 | MUPS v4.3 5 节点管道 + v5 EscapeEngine span 扩展：新增 §Operations MUPS 5 节点 + 跨域闭环 + Auto-Close + EscapeEngine 共 9 ops + 9 sessionSpan attributes；新增 §MUPS 5 节点管道 Trace 树 + EscapeEngine Trace 树 |
| **4.0.0** | **2026-06-26** | **6 S 精简 + 5 个新 Span 推进**（DM-20260626-001）：(1) 14 S → **6 S + 1 横切** span 重归类（按博弈角色 State Authority / Mediator+Turn Leader+Error Recovery / Mechanism Designer / Costly Signaler+Certifier / Info Producer+Quantizer / Pipeline Coord+Memory / 横切 Discipline Keeper）；(2) **新增 5 个 P0/P1 span ops**：`D7_Orchestration_Channel_Route`（P0/S6-A48）+ `D7_Orchestration_Memory_Persist`（P0/S6-A49）+ `D7_Orchestration_System_Anomaly_Detect`（P0/S4-A47）+ `D7_Orchestration_TaskGraph_Synthesize`（P1/S5-A33）+ `D7_Orchestration_Executor_Select`（P1/S5-A34）；(3) 移除旧 S6（横切 Hardening）占 S 位，改为 cross-cutting；(4) 移除旧 S7-S14 ops 命名（Observe_Quantize/Plan_Generate/Verify_Verdict/Learn_Asset 保留，Execute_Artifact → Channel_Route，Observe_Request_WithPrior 拆入 Uncertainty_Update）；(5) MUPS 5 节点管道 Trace 树重归类为 6 S；(6) Cross-cutting Hardening Trace 树新增 |
