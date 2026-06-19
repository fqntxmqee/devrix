# D7 Orchestration — 可观测性与验收指南

**Capability:** d7-orchestration
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-16
**Parent:** `d7-domain.md` · `span-registry.md` · `t-registry.md`
**Complements:** `terminal-state-guide.md` · `../d5-observability/span-registry.md`

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

| Operation | S | 绑定 T（P0 加粗） |
|-----------|---|------------------|
| `D7_Orchestration_Session_Process` | S2 | **D7-S2-T01**, **D7-D1-T01**, **D7-MIG-T01** |
| `D7_Orchestration_Intent_Classify` | S2/S5 | **D7-S2-T02b**, **D7-S5-T03**, **D7-S5-T06**, **D7-S5-A01-T01** |
| `D7_Orchestration_Turn_Run` | S2 | **D7-S2-A06-T01** … **T04** |
| `D7_Orchestration_Turn_Iteration` | S2 | **D7-S2-A06-T03**（multi-turn tool loop） |
| `D7_Orchestration_LLM_Invoke` | S2 | **D7-S2-A07-T01**, **D7-S2-A07-T02** |
| `D7_Orchestration_Orchestrate_Run` | S2 | **D7-S2-T03**, **D7-S2-A01-T05** |
| `D7_Orchestration_Wave_Schedule` | S3 | **D7-S3-T01** … **D7-S3-T10** |
| `D7_Orchestration_Wave_Task_Execute` | S3 | **D7-S3-T04** … **T07** |
| `D7_Orchestration_Flow_Event_Publish` | S4 | **D7-S4-T02**, **D7-S4-T03**, **D7-S4-T08**, **D7-S4-T09** |

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
    └── CommandHandler → S1 PlanMode / Task / S2 HandleInterrupt
```

完整跨域树（含 D1 展示侧）见 `span-registry.md` §Trace Tree · `../d5-observability/span-registry.md`。

---

## 3. FastPath SLA 与性能 T

| 指标 | 目标 | T ID | 观测 |
|------|------|------|------|
| Classify 后 proxy 开销 | P99 ≤ 2ms | **D7-S2-T02a** | Session_Process 子 span |
| 规则 Classify | P99 ≤ 1ms | **D7-S2-T02b** | Intent_Classify |
| command-first 全栈 | P99 ≤ 2ms | **D7-S2-T02c** | 端到端 integration |

**硬约束：** FastPath **不得**调用 LLM Classify（D7-S5-T06）；`/plan` `/task` 走 command-first（D7-S5-A01-T02）。

---

## 4. Hub-Spoke Flow 与 D1 展示链

### 4.1 发布路径

```text
D4 Agent / D2 SubQuery / D7 Wave
    → S4-A04/A05 SpokeBridge
    → S4-A01 PublishFlowEvent (GlobalHub)
        ├─ workplan.Service.Apply
        ├─ sessionqueue (delegate-progress)
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

全表 66 条见 `t-registry.md`（66/66 IMPLEMENTED）。

| S | T 数 | P0 数 | 覆盖重点 |
|---|------|-------|----------|
| S1 Work Model | 8 | 3 | Task 持久化、DAG、PlanMode、状态机 |
| S2 Session Orchestrator | 18 | 14 | ProcessMessage、FastPath SLA、Interrupt、Turn Leader、Dispatch |
| S3 Wave Scheduler | 11 | 8 | DAG 并发、Conflict、Context policy、Cancel |
| S4 Execution Flow | 9 | 7 | Hub 双通道、SpokeBridge、IM progress |
| S5 Decision & Planning | 14 | 10 | Classify、Synthesize、SelectExecutor、command-first |
| 契约/迁移 | 6 | 2 | D1 入口、D2 瘦身、D6 advisory |

### P0 必跑清单

```bash
# D1→D7 入口 + FastPath 集成
go test ./tests/integration/d7/ -run 'D7Entry|FastPath' -v

# Session Orchestrator 核心
go test ./internal/layers/orchestration/sessionorchestrator/ -v

# Decision planning (Classify / Decompose)
go test ./internal/layers/orchestration/decisionplanning/ -v

# Turn Leader + LLM Invoker（DM-020）
go test ./internal/layers/orchestration/turn/ -v

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

### 建议告警

| 告警 | 条件 | 严重度 |
|------|------|--------|
| D7 未 wired | RouteInbound 无 Session_Process | P0 |
| FastPath LLM 误触 | route=fast 且 classify.source=llm | P1 |
| Wave 卡死 | Task_Execute 无 terminal Flow 超 30min | P1 |
| D6 validation 超时率高 | timeout_rate > 5% / 5min | P2 |
| Breaker 阻断 Turn | LLM_Invoke error + breaker.open | P1 |

---

## 8. 已知缺口

| 缺口 | 现状 | 建议 |
|------|------|------|
| IntentSkip span | 无独立 operation | 合并在 Session_Process `{route=skip}` |
| S1 Task span | 无显式 OTel op | D5 补 `D7_WorkModel_*` |
| Orchestrate LLM Decompose | llm.purpose=decompose 无独立 span 文档 | span-registry 补登记 |
| BackgroundRun | QueryWorkPlan 可见，trace 未标准化 | S1-T07 扩展 integration trace |

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
