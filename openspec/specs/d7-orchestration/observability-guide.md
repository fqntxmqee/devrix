# D7 Orchestration — 可观测性与验收指南

**Capability:** d7-orchestration
**Status:** Active
**Version:** 2.1.0
**Last Updated:** 2026-06-26 (inner-spans + dedup-remove, PR #254)
**Parent:** `d7-domain.md` · `span-registry.md` · `t-registry.md`
**Complements:** `terminal-state-guide.md` · `../d5-observability/span-registry.md`

> **v6.0.0 6 S 精简（DM-20260626-001）：** 本文 §1 Canonical Span↔T 绑定 + §5 T 层验收矩阵按新 6 S + 1 横切 重归类（14 S → 6 S；S 编号变化，span operation 名保持稳定）。Span↔T 绑定细则见 `span-registry.md §Operations`（v6.0.0 已落地 23 ops + 9 sessionSpan attributes）。

---

## 0. 文档定位

本文档提供 **Span ↔ T 绑定视图、Trace 树、按 S 分组的 T 验收摘要与 P0 Runbook**。

| 本文档提供 | 权威 SoT 在其他文件 |
|-----------|-------------------|
| Canonical Span ↔ T 绑定矩阵 | Span operation 全表 → `span-registry.md` |
| Fast / Orchestrate Trace 树 | OTel 常量 → `telemetry/names.go` |
| 按 S 分组的 T 验收摘要 + P0 清单 | T 点全表（66 条）→ `t-registry.md` |
| FastPath SLA / D6 advisory 检查 | Gherkin 场景 → `spec.md` |
| 生产 Trace 检查清单 | 跨域边界 → `../d2-context-engine/d7-boundary.md` |

---

## 1. Canonical Span ↔ T 绑定

代码常量 SoT：`internal/layers/observability/instrument/telemetry/names.go`。

| Operation | S（v6.0.0 6 S） | 绑定 T（P0 加粗） |
|-----------|---|------------------|
| `D7_Orchestration_Session_Process` | **S2** | **D7-S2-T01**, **D7-D1-T01**, **D7-MIG-T01** |
| `D7_Orchestration_Intent_Classify` | **S2/S5** | **D7-S2-T02b**, **D7-S5-T03**, **D7-S5-T06**, **D7-S5-A01-T01** |
| `D7_Orchestration_Turn_Run` | **S2** | **D7-S2-A06-T01** … **T04** |
| `D7_Orchestration_Turn_Iteration` | **S2** | **D7-S2-A06-T03**（multi-turn tool loop） |
| `D7_Orchestration_LLM_Invoke` | **S2** | **D7-S2-A07-T01**, **D7-S2-A07-T02** |
| `D7_Orchestration_Orchestrate_Run` | **S2** | **D7-S2-T03**, **D7-S2-A01-T05** |
| `D7_Orchestration_Wave_Schedule` | **S3** | **D7-S3-T01** … **D7-S3-T10** |
| `D7_Orchestration_Wave_Task_Execute` | **S3** | **D7-S3-T04** … **T07** |
| `D7_Orchestration_Flow_Event_Publish` | **S4** | **D7-S4-T02**, **D7-S4-T03**, **D7-S4-T08**, **D7-S4-T09** |
| `D7_Orchestration_Observe_Quantize` | **S5**（原 S8） | **D7-S8-A15-T01..T06**（Phase 2 PR-A1 + PR-RF）|
| `D7_Orchestration_Plan_Generate` | **S5**（原 S8 PR-B1） | **D7-S8-A22-T01..T03**（Phase 2 PR-B1）|
| `D7_Orchestration_Execute_Artifact` | **S6**（原 S9） | **D7-S9-A25-T01..T04**（Phase 3 PR-C1）|
| `D7_Orchestration_Verify_Verdict` | **S4**（原 S10） | **D7-S10-A32-A35 T1..T08**（Phase 4）|
| `D7_Orchestration_Learn_Asset` | **S6**（原 S11） | **D7-S11-A36-A40 T1..T05**（Phase 5）|
| `D7_Orchestration_Observe_Request_WithPrior` | **S2/S5**（原 S12） | **D7-S12-A41-T01..T03**（Phase 6）|
| `D7_Orchestration_Verify_AutoClose` | **S2/S4**（原 S13） | **D7-S13-A47-T01..T03**（Phase 7）|
| `D7_Orchestration_Escape_Engine_Run` | **S2**（原 S14） | **D7-S14-A50 T1..T03**（Phase v5 PR-V5.0）|
| `D7_Orchestration_Resume_Session` | **S2**（原 S14） | **D7-S14-A51-T01..T03**（Phase v5 PR-V5.6）|
| `D7_Orchestration_Channel_Route` ⭐新 P0 | **S6** | （v6.0.0 新增，channel.route）|
| `D7_Orchestration_Memory_Persist` ⭐新 P0 | **S6** | （v6.0.0 新增，memory.persist）|
| `D7_Orchestration_System_Anomaly_Detect` ⭐新 P0 | **S4** | （v6.0.0 新增，system.anomaly_detect）|
| `D7_Orchestration_TaskGraph_Synthesize` ⭐新 P1 | **S5** | （v6.0.0 新增，taskgraph.synthesize）|
| `D7_Orchestration_Executor_Select` ⭐新 P1 | **S3** | （v6.0.0 新增，executor.select）|
| `D7_S1_Worktree_Op` ⭐DM-20260626-009 P1 | **S1** | **D7-S1-A52-T11** happy + **T12** nil-bridge fail-safe；ItemPipelineRunner 11 callsite |
| `D7_S1_SubWorktree_Run` ⭐DM-20260626-009 P2 | **S1** | **D7-S1-A53-T13** happy + **T14** nil-bridge fail-safe；session_turn_loop.RunParallelExplore 1 callsite |
| `D7_S5_SubTurn_Iteration` ⭐DM-20260626-009 P1 | **S5** | **D7-S5-A54-T15** happy + **T16** nil-bridge fail-safe；WorkItemExecutor ReAct loop per-iter + cap-hit（iter=max+1）|

> **v6.0.0 6 S 精简说明：** 14 S → 6 S 后 span operation 按新 S 重归类。Observe/Plan 归 **S5**（Information Producer + Quantizer），Execute/Learn/AutoClose 归 **S6**（Pipeline Coordinator + Memory Curator），Verify 归 **S4**（Certifier），AutoClose/Resume/EscapeEngine 入口 归 **S2**（Mediator + Error Recovery）。Span operation 名保持稳定（Jaeger 查询不破坏），仅 S 编号变化。5 个新 P0/P1 span（v6.0.0 新增）按 S 归类：channel.route / memory.persist → S6；system.anomaly_detect → S4；taskgraph.synthesize → S5；executor.select → S3。

### 关键 Span 属性

| Attribute | 出现位置 | 用途 |
|-----------|----------|------|
| `orchestration.route` | Session_Process | `fast` / `orchestrate` / `command` / `skip` |
| `orchestration.intent.*` | Intent_Classify | IntentKind + confidence |
| `orchestration.classify.source` | Intent_Classify | `rule` / `llm` / `command` |
| `session_id` | 全部 S2/S3 | 会话关联 |
| `turn.index` | Turn_Iteration / LLM_Invoke | Turn 内序号 |
| `llm.purpose` | LLM_Invoke | `turn` / `compress` / `decompose` |
| `context.caller=d7` | D2 span（被调侧） | 区分 D7 编排 vs Legacy D2 路径 |
| `event_kind` / `worker_id` | Flow_Event_Publish | Flow 生命周期 |
| `observation.kind` / `observation.strength` / `uncertainty.coord_kind` | Observe_Quantize | Observe 节点产出 |
| `plan.kind` / `plan.strength` / `source_observation_ids` | Plan_Generate | Plan 节点产出 |
| `artifact.kind` / `channel.kind` / `source_plan_id` | Execute_Artifact | Execute 节点产出 |
| `verdict.kind` / `exit_reason` / `source_artifact_id` | Verify_Verdict | Verify 节点产出 |
| `asset.class` / `asset.ttl` / `reputation.beta_alpha` / `reputation.beta_beta` | Learn_Asset | Learn 节点产出 |
| `prior.adaptive_kind` / `prior.beta_alpha` / `prior.beta_beta` / `prior.evidence_count` / `prior.cycle_count` / `prior.last_update` | Session_Process（Phase 7）+ Observe_Request_WithPrior | 6 prior attributes 闭环 |
| `resume.decision` / `resume.circuit_level` / `resume.user_choice` | Session_Process（V5.6）+ Resume_Session | 3 resume attributes |

> **DM-020：** Jaeger 中 D3 `D3_LLM_Stream` 应挂在 `D7_Orchestration_LLM_Invoke` 下，而非 D2 `Query_Loop_*`（Legacy 路径）。

---

## 2. Trace 树

### 2.1 IntentFast（D7 → D2 → D7 → D3）

```text
D1_Capture_Message_Receive
└── D1_Dispatch_Route {target=d7}
    └── D7_Orchestration_Session_Process  {route=fast}
        ├── D7_Orchestration_Intent_Classify
        └── D7_Orchestration_Turn_Run
            ├── D2_Context_Process          ← Prepare (context.caller=d7)
            └── D7_Orchestration_Turn_Iteration
                ├── D7_Orchestration_LLM_Invoke
                │   └── D3_LLM_Stream
                └── D2_Tool_Execute_Single    ← optional
```

### 2.2 IntentOrchestrate（D7 → S5 → S3 → D4/D2）

```text
D1_Capture_Message_Receive
└── D1_Dispatch_Route
    └── D7_Orchestration_Session_Process  {route=orchestrate}
        ├── D7_Orchestration_Intent_Classify
        └── D7_Orchestration_Orchestrate_Run
            └── D7_Orchestration_Wave_Schedule
                └── D7_Orchestration_Wave_Task_Execute
                    ├── D2_Context_Process
                    └── D4_Agent_Run
                        └── D7_Orchestration_Flow_Event_Publish
                            └── D1_Signal_Task / worker_progress
```

### 2.3 IntentCommand（零 LLM）

```text
D7_Orchestration_Session_Process  {route=command}
└── (无 Intent_Classify LLM span)
    └── CommandHandler → S1 PlanMode / WorkItem / S2 HandleInterrupt
```

### 2.4 MUPS 5 节点管道（Phase 2-7 + v5，2026-06-25；v6.0.0 6 S 归类）

```text
D7_Orchestration_Session_Process  {route=orchestrate}
├── D7_Orchestration_Observe_Quantize           ← D7-S5 Observe（原 S8）
│   └── D7_Orchestration_Observe_Request_WithPrior  ← D7-S2/S5（原 S12，prior 注入）
├── D7_Orchestration_Plan_Generate               ← D7-S5 Plan（原 S8 PR-B1）
├── D7_Orchestration_Execute_Artifact            ← D7-S6 Execute（原 S9）
├── D7_Orchestration_Verify_Verdict              ← D7-S4 Verify（原 S10）
│   └── D7_Orchestration_Verify_AutoClose        ← D7-S2/S4（原 S13，若触发）
├── D7_Orchestration_Learn_Asset                 ← D7-S6 Learn（原 S11）
└── (下轮) D7_Orchestration_Observe_Quantize     ← ReputationEvidence 闭环
```

### 2.5 EscapeEngine Trace 树（Phase v5）

```text
D7_Orchestration_Session_Process
└── D7_Orchestration_Escape_Engine_Run  {circuit_level=L1..L5}
    └── D7_Orchestration_Resume_Session  {resume.decision=fall_through/force_exited/aborted}
```

### 2.6 WorkItem Inner Layer Trace 树（DM-20260626-009 follow-up，2026-06-26）

5-node MUPS 根 span 之外，工作树每次 mutation + WorkItemExecutor ReAct 每次 iteration 在 trace 显形。否则"16s session 慢在哪一步 / 哪一步把 round_phase 改到 X"必须读代码。

```text
D7_Orchestration_Session_Process  {route=orchestrate}
└── D7_Orchestration_Execute_Artifact  (MUPS Execute, S6)
    └── (imSink → ItemPipelineRunner span)
        ├── D7_S1_Worktree_Op[set_round_phase]   (S1, op="set_round_phase", phase_or_status=observe/plan/execute/verify/learn/decide) ×5
        ├── D7_S1_Worktree_Op[apply_pipeline_round]  (S1, op="apply_pipeline_round") ×1
        ├── D7_S1_Worktree_Op[update_status]   (S1, op="update_status") ×3-5
        ├── D7_S1_Worktree_Op[list_children]   (S1, op="list_children") ×0-1
        └── (WorkItemExecutor 进入 ReAct 循环)
            └── D7_S5_SubTurn_Iteration[iter=1..N]  (S5, finish_reason=stop/tool_calls/length, stop_reason=ok/final_answer/llm_error/...) ×N per ReAct loop, N≤MaxIters
                └── D7_S5_SubTurn_Iteration[iter=max+1, finish_reason=tool_calls, stop_reason=max_iters]  cap-hit 多发 1 span

# session_turn_loop.RunParallelExplore (S2 LoopDepthTracker v2)
D7_Orchestration_Session_Process
└── D7_S1_SubWorktree_Run[parent_id, child_id, spawned_by=parallel_explore]  (S1) ×N per parallel_explore batch
    └── D7_Orchestration_Session_Process (child session)
```

**finish_reason vs stop_reason 正交说明：**

- `subturn.finish_reason` (LLM 真实) — `stop` / `tool_calls` / `length` / `content_filter` / `function_call` / ...
- `subturn.stop_reason` (executor 自定义) — `final_answer` / `tool_error` / `tool_no_executor` / `tool_no_results` / `llm_error` / `max_iters` / `ok` / `llm_finish_<X>`

LLM 报告 `finish_reason=tool_calls` 时 executor 通常走 `stop_reason=ok`（继续 loop）或 `stop_reason=tool_no_executor`（首次循环 degrade）。cap-hit `stop_reason=max_iters` 隐含最近一次 finish_reason=`tool_calls`（否则不会循环到 cap），trace 上显式写成 `iter=max+1, finish_reason=tool_calls, stop_reason=max_iters`。

完整跨域树（含 D1 展示侧）见 `span-registry.md` §Trace Tree · `../d5-observability/span-registry.md`。

---

## 3. FastPath SLA 与性能 T

| 指标 | 目标 | T ID | 观测 |
|------|------|------|------|
| Classify 后 proxy 开销 | P99 ≤ 2ms | **D7-S2-T02a** | Session_Process 子 span |
| 规则 Classify | P99 ≤ 1ms | **D7-S2-T02b** | Intent_Classify |
| command-first 全栈 | P99 ≤ 2ms | **D7-S2-T02c** | 端到端 integration |

**硬约束：** FastPath **不得**调用 LLM Classify（D7-S5-T06）；`/plan` `/worktree` 走 command-first（D7-S5-A01-T02）。

---

## 4. Hub-Spoke Flow 与 D1 展示链

### 4.1 发布路径

```text
D4 Agent / D2 SubQuery / D7 Wave
    → S4-A04/A05 SpokeBridge
    → S4-A01 PublishFlowEvent (GlobalHub)
        ├─ workplan.Service.Apply
        ├─ executionflow (delegate-progress)
        └─ imsink.GatewaySink → D1 EngineEvent
            └─ D1-S15 PresentTaskProgress
```

### 4.2 测试 ↔ T ↔ 断言

| 场景 | 测试文件 | T ID |
|------|----------|------|
| Hub 双通道 WorkPlan + Queue + IM | `executionflow/hub/hub_test.go` | **D7-S4-T02** |
| FlowStarted → delegate-progress | `executionflow/hub/hub_test.go` | **D7-S4-T03** |
| AgentBridge success/error | `hubspoke/hubspoke_test.go` | **D7-S4-T08** |
| SubQueryBridge 三态 | `hubspoke/hubspoke_test.go` | **D7-S4-T09** |
| IMSink worker_progress | `executionflow/imsink/gateway_test.go` | **D7-S4-T05** |
| 禁止伪造 Task 进度 | `sessionorchestrator/orchestrator_test.go` | **D7-S2-A01-T03** |

---

## 5. T 层验收矩阵（按 S 摘要）

全表 180 条见 `t-registry.md`（180/180 IMPLEMENTED，2026-06-25 v4.9.0 闭环）。**DM-20260626-009 follow-up 新增 6 T → 186 条**（PR #253+#254 落地；t-registry v4.7.0）：S1 +4 (A52 T11/T12 + A53 T13/T14) / S5 +2 (A54 T15/T16)。

| S（v6.0.0 6 S） | T 数 | P0 数 | 覆盖重点 |
|---|------|-------|----------|
| **S1 WorkModel**（原 S1） | 10 | 5 | WorkItem 持久化、DAG、PlanMode、状态机（v4.3 Task flat-view 全删）+ 内层 worktree.op + subworktree.run 4 T（DM-20260626-009）|
| **S2 SessionOrchestrator**（原 S2 + S12 入口 + S13 入口 + S14 入口） | 18 | 14 | ProcessMessage、FastPath SLA、Interrupt、Turn Leader、Dispatch、AutoClose、Resume、Escape |
| **S3 WaveScheduler**（原 S3） | 11 | 8 | DAG 并发、Conflict、Context policy、Cancel |
| **S4 ExecutionFlow + Verify**（原 S4 + S10） | 9 | 7 | Hub 双通道、SpokeBridge、IM progress + Verify 4 态 Verdict + 14 ExitReason |
| **S5 DecisionPlanning + Observe**（原 S5 + S8） | 16 | 12 | Classify、Synthesize、SelectExecutor、command-first + Observe 4 类 + UncertaintyReport + 内层 subturn.iteration 2 T（DM-20260626-009）|
| **S6 MUPS Pipeline**（原 S7 + S9 + S11 + S12 E2E + S13 兜底） | 15 | 15 | Execute/Learn + AutoClose + ObserveLearner 闭环 + Pipeline Coordinator |
| **Cross-cutting Hardening**（原 S6 拆 2 A） | 2 | 2 | DM-20260622-001 5 fix + CircuitBreaker Monitor（横切硬化）|
| 契约/迁移 | 6 | 2 | D1 入口、D2 瘦身、D6 advisory |

> **v6.0.0 6 S 精简说明：** T 层按 14 S → 6 S + 1 横切 重归类（S 编号变化，T ID 保持稳定以便追溯）。具体 A/T 重映射见 `a-registry.md §v6.0.0 6 S 精简映射` + `t-registry.md`。

### P0 必跑清单

```bash
# D1→D7 入口 + FastPath 集成
go test ./tests/integration/d7/ -run 'D7Entry|FastPath' -v

# Session Orchestrator 核心
go test ./internal/layers/orchestration/sessionorchestrator/ -v

# Decision planning (Classify / Decompose)
go test ./internal/layers/orchestration/decisionplanning/ -v

# Turn Leader + LLM Invoker（DM-020）
go test ./internal/layers/orchestration/sessionorchestrator/ -v

# Wave DAG 调度
go test ./internal/layers/orchestration/wavescheduler/ -v

# Execution flow + Hub-Spoke tests
go test ./internal/layers/orchestration/executionflow/... ./internal/layers/orchestration/hubspoke/ -v

# WorkModel + PlanMode
go test ./internal/layers/orchestration/workmodel/ -v

# /stop 中断顺序
go test ./tests/integration/d7/ -run Interrupt -v
```

核心集成：`tests/integration/d7/d7_fastpath_test.go` · `d7_entry_test.go` · `d7_interrupt_test.go` · `d7_hub_flow_test.go`。

---

## 6. D6 Advisory 可观测性

| Metric | 含义 | T |
|--------|------|---|
| `orchestration.d6.validation.pass` | 校验通过 | D7-D6-T03 |
| `orchestration.d6.validation.fail` | 校验拒绝（advisory，不阻塞） | D7-D6-T01 |
| `orchestration.d6.validation.timeout` | 50ms 超时视为 pass | D7-D6-T02 |
| `orchestration.d6.validation.error` | panic-recovered | D7-D6-T05 |

测试：`sessionorchestrator/validation_metrics_test.go`。timeout_rate > 5% 触发 AlertHook（D7-D6-T04）。

---

## 7. 生产 Trace 检查清单

| 检查项 | 查询 / 条件 | 期望 |
|--------|------------|------|
| D7 入口 | `D7_Orchestration_Session_Process` 每用户消息 1 个 | 无 D1→D2 直连 |
| Fast 路由 | `orchestration.route=fast` | Intent_Classify source=rule |
| LLM 产权 | `D7_Orchestration_LLM_Invoke` → `D3_LLM_Stream` | 不在 D2 Query_Loop 下 |
| Orchestrate 链 | Orchestrate_Run → Wave_Schedule → Task_Execute | Flow_Event_Publish 存在 |
| Flow 到 D1 | D7 Flow_Event_Publish → D1_Signal_* | worker_id 一致 |
| Interrupt | `/stop` 后 Wave span 终止 | stopped EngineEvent |
| **MUPS 5 节点** | Session_Process 下 5 个子 span 齐全（Observe_Quantize + Plan_Generate + Execute_Artifact + Verify_Verdict + Learn_Asset）| 缺一个需 P1 告警 |
| **WorkItem 内层 span** | Execute_Artifact 子树中 Worktree_Op ≥ 1 + (SubTurn_Iteration ≥ 1 或 cap-hit iter=max+1) | 缺内层 span 时 P2 告警（内层未 wired）|
| **Worktree Op 完整** | Worktree_Op 序列含 set_round_phase(×5) + apply_pipeline_round + update_status + list_children | 缺任意 op 时 P2 告警（trace 路径不完整）|
| **Sub-Turn finish_reason** | SubTurn_Iteration.finish_reason ∈ {stop, tool_calls, length, content_filter, function_call} | 非空字符串值；空时 P2 告警（LLM 未上报 finish_reason）|
| **Prior 闭环** | Observe_Request_WithPrior 的 prior.beta_alpha = 上轮 Learn_Asset 的 reputation.beta_alpha | 闭环一致 |
| **Auto-Close** | Verify_AutoClose.auto_close.rule ∈ {R1, R2, R3, R4} | 4 规则对应 4 场景 |
| **EscapeEngine** | circuit_level ∈ {L0..L5} | L4/L5 触发 P1 告警 |

### 建议告警

| 告警 | 条件 | 严重度 |
|------|------|--------|
| D7 未 wired | RouteInbound 无 Session_Process | P0 |
| FastPath LLM 误触 | route=fast 且 classify.source=llm | P1 |
| Wave 卡死 | Task_Execute 无 terminal Flow 超 30min | P1 |
| D6 validation 超时率高 | timeout_rate > 5% / 5min | P2 |
| Breaker 阻断 Turn | LLM_Invoke error + breaker.open | P1 |
| **MUPS 5 节点缺失** | Session_Process route=orchestrate 但缺任意 5 节点 span | P1 |
| **EscapeEngine L4+** | circuit_level ≥ L4 触发 | P1 |
| **Prior 闭环断** | Observe_Request_WithPrior.prior.beta_alpha 与上轮 Learn_Asset 不一致 | P2 |
| **Auto-Close 误触** | auto_close.rule=R1（idle_timeout）但 last_activity < 60s | P2（怀疑 session state 异常）|

---

## 8. 已知缺口

| 缺口 | 现状 | 建议 |
|------|------|------|
| IntentSkip span | 无独立 operation | 合并在 Session_Process `{route=skip}` |
| S1 Task span | 无显式 OTel op | D5 补 `D7_WorkModel_*` |
| Orchestrate LLM Decompose | llm.purpose=decompose 无独立 span 文档 | span-registry 补登记 |
| BackgroundRun | QueryWorkPlan 可见，trace 未标准化 | S1-T07 扩展 integration trace |
| ~~Worktree Op 内层 span 缺失~~ | ~~ItemPipelineRunner 11 callsite 无显形，调试 worktree mutation 必须读代码~~ | ✅ **DM-20260626-009 P1 已补 D7_S1_Worktree_Op + D7_S1_SubWorktree_Run**（PR #253+#254） |
| ~~WorkItemExecutor ReAct iter span 缺失~~ | ~~16s session 慢在哪次 iter 无法定位~~ | ✅ **DM-20260626-009 P1 已补 D7_S5_SubTurn_Iteration + cap-hit 多发 1 span**（PR #253+#254，follow-up PR #255 thread LLM finishReason） |
| **worktree.op 全 op 覆盖** | 当前仅 set_round_phase / apply_pipeline_round / update_status / list_children 4 类；新增 mutator 需补 op 标签 | v2.1.0 后 guard: grep "r.Tasks.Tree().*()" callsite 必须配套 EmitWorktreeOp |

---

## 10. D5 Dashboard 过滤规则变更（DM-20260622-001）

> 5 fix 中 **A1**（metric 命名 spec/code 对齐）直接影响 D5 Grafana dashboard 的 metric 过滤规则。

### 10.1 已废弃 singular key（0 流量，保留兼容 30 天）

| 旧 key（已废弃） | 新 key（spec/code 一致） | T ID |
|-----------------|------------------------|------|
| `worker_panic` | `worker_panics` | D7-S6-A14-T02 |
| `dispatch_wakeup` | `dispatch_loop_wakeups` | D7-S6-A14-T01 |

**迁移窗口：** 2026-06-22 → 2026-07-22 旧 key 仍可被 emit（caller 写 0），建议 D5 dashboard 维护者将 panel 全部切到新 key 后再关闭旧 key 兼容。

### 10.2 新增 backpressure 信号

`CommandHandler.emit` 改 `select-default` 后，**新信号** `slog.Warn("command_handler: out channel full, drop event", ...)` 可作为 consumer 端 backpressure 的早期指标：

| Log level | 查询条件 | 期望 |
|-----------|---------|------|
| WARN | `command_handler: out channel full, drop event` | 0（正常）；持续出现 → consumer stall 或 IM 展示端拖慢 |

**建议告警：** drop rate > 1/min 持续 5min 触发 P2 通知（飞书 / 邮件），不阻塞业务。

### 10.3 跨域 metric 归属澄清

| Metric | 实际 emitter | 备注 |
|--------|------------|------|
| `sandbox_exit_failed` | **D4** `multiagent/execute/worker.go::recordSandboxExitFailed` | D7 spec D7-S6-A12-T01 标 OBSOLETE 2026-06-22；D5 dashboard 必须从 D4 域 panel 取数（详见 D7-S6-A14-T03） |

---

## 9. 关联文档

| 文档 | 关系 |
|------|------|
| `span-registry.md` | Span operation 登记 SoT |
| `t-registry.md` | T 点全表 SoT |
| `terminal-state-guide.md` | IntentKind 四链与时序 |
| `d7-domain.md` | 领域 SoT |
| `spec.md` | Gherkin 验收 |
| `../d1-communication/observability-guide.md` | D1 展示侧对称指南 |

---

## 修订记录

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-16 | 初版：Span↔T、Trace 树、Hub Flow、T 分组摘要、P0 Runbook、D6 metric |
| **1.1.0** | **2026-06-22** | **DM-20260622-001 D5 Dashboard 过滤规则变更**：metric 名 plural 对齐（`worker_panic` → `worker_panics`，`dispatch_wakeup` → `dispatch_loop_wakeups`），详见 §10；新增 CommandHandler emit backpressure 观测信号 |
| **1.2.0** | **2026-06-25** | **MUPS v4.3 5 节点管道 + v5 EscapeEngine 可观测性扩展**：(1) §1 Canonical Span ↔ T 表新增 9 ops × S8-S14 绑定（S8 Observe / S8 PR-B1 Plan / S9 Execute / S10 Verify / S11 Learn / S12 跨域 / S13 Auto-Close / S14 EscapeEngine + Resume）；(2) §1 关键 Span 属性新增 7 类（observation/plan/artifact/verdict/asset + 6 prior + 3 resume）；(3) §2 Trace 树新增 2.4 MUPS 5 节点 + 2.5 EscapeEngine；(4) §5 T 层验收矩阵 S1-S5 → S1-S14，66 T → 180 T；(5) §7 生产 Trace 检查清单新增 4 行（MUPS 5 节点 / Prior 闭环 / Auto-Close / EscapeEngine），建议告警新增 4 条 |
| **2.0.0** | **2026-06-26** | **6 S 精简（DM-20260626-001）**：§1 Canonical Span↔T 表 14 S → 6 S 重归类（Observe/Plan 归 S5；Execute/Learn 归 S6；Verify 归 S4；AutoClose/Resume/Escape 入口 归 S2；Observe_Request_WithPrior 归 S2/S5；Verify_AutoClose 归 S2/S4）；新增 5 个 v6.0.0 新 P0/P1 span 绑定（channel.route / memory.persist → S6；system.anomaly_detect → S4；taskgraph.synthesize → S5；executor.select → S3）；§2.4 MUPS Trace 树注释 S 编号重归类；§5 T 层验收矩阵 14 S → 6 S + 1 横切 重写（每个 S 标注原 14 S 归属）。Span operation 名保持稳定（Jaeger 查询不破坏），仅 S 编号变化 |
| **2.1.0** | **2026-06-26** | **DM-20260626-009 follow-up 内层 observability span + dedup 删除（PR #253+#254 落地，follow-up PR #255 待开）**：(1) §1 Canonical Span↔T 表新增 3 ops × S1+S5 绑定（D7_S1_Worktree_Op → D7-S1-A52-T11/T12；D7_S1_SubWorktree_Run → D7-S1-A53-T13/T14；D7_S5_SubTurn_Iteration → D7-S5-A54-T15/T16）；(2) §2 Trace 树新增 2.6 WorkItem Inner Layer Trace 树（ItemPipelineRunner 11 callsite + WorkItemExecutor ReAct iter + cap-hit + RunParallelExplore 1 callsite，含 finish_reason vs stop_reason 正交说明）；(3) §5 T 层验收矩阵 S1 8→10 + S5 14→16（180→186 T）；(4) §7 生产 Trace 检查清单新增 3 行（WorkItem 内层 span + Worktree Op 完整 + Sub-Turn finish_reason），建议告警覆盖；(5) §8 已知缺口关闭 2 项（Worktree Op 内层 span 缺失 + WorkItemExecutor ReAct iter span 缺失），新增 1 项（worktree.op 全 op 覆盖 guard） |
