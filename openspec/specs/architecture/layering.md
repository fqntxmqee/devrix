# Devrix Domain Architecture Specification

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.0.0
**Last Updated:** 2026-06-12

---

## Overview

本文档定义 Devrix 的正式分层架构，使用 **DSAFT 五层编号系统**：

- **D (Domain)** — 顶层领域，对应 `internal/layers/` 下的一级目录
- **S (Scenario)** — 域内场景/模块，对应二级包目录
- **A (Activity)** — 可发起的业务动作，归属于 D-S 场景
- **F (Function Point)** — 最小业务/技术逻辑单元，被 A 层活动编排
- **T (Test Point)** — 测试点，标准格式 `D{X}-S{X}-A{XX}-T{NN}`

> A/F 注册表权威来源分别为 `openspec/a-registry.md` 和 `openspec/f-registry.md`。

---

## Domain (D) — Top-Level Domains

| Domain ID | 名称 | 缩写 | Responsibility |
|-----------|------|------|----------------|
| **D1** | Communication Domain | COMM | IM Gateway, WebSocket, CLI adapter |
| **D2** | Context Engine Domain | CTX | QueryLoop, 7-step compression, layered memory |
| **D3** | LLM Gateway Domain | LLM | Model adapter, circuit breaker, token counter |
| **D4** | Multi-Agent Domain | AGENT | Agent lifecycle, fork, collaboration modes |
| **D5** | Observability Domain | OBS | Tracing, metrics, logging |
| **D6** | Evolution Domain | EVO | Eval engine, quality probes, runtime orchestration validation |

---

## Scenario (S) — Domain Scenarios

每个 Domain 内部包含多个 Scenario，采用 **D{N}-S{M}** 格式编号。

### D1 Communication Domain

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D1-S1 | Gateway | 消息网关、路由、会话管理 | IMPLEMENTED |
| D1-S2 | Adapters | 飞书、WebSocket、CLI 适配器 | IMPLEMENTED |
| D1-S3 | Commands | CLI 命令解析 (/new, /stop, /help) | PLANNED (code in gateway) |
| D1-S4 | Auth | 认证与授权 | PLANNED |
| D1-S5 | Milestone | 里程碑跟踪 | IMPLEMENTED |
| D1-S6 | RateLimit | 限流控制 | IMPLEMENTED |
| D1-S7 | Metrics | 通信层指标 | IMPLEMENTED |
| D1-S8 | Renderers | 消息渲染器 | IMPLEMENTED |
| D1-S9 | EventBus | 事件总线：背压、Drain、Compact、Reconnect | IMPLEMENTED |
| D1-S10 | Connection | IM 实例连接管理 | IMPLEMENTED |
| D1-S11 | Core | 核心配置解析 | IMPLEMENTED |
| D1-S12 | Instance | 实例注册/注销 | IMPLEMENTED |

### D2 Context Engine Domain

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D2-S1 | PEV | Plan-Execute-Verify 循环引擎（**已退役**，由 D2-S10 替代） | RETIRED |
| D2-S2 | Compression | 七步压缩管道 | IMPLEMENTED |
| D2-S3 | Memory | 分层记忆管理 (Working/LongTerm) | IMPLEMENTED |
| D2-S4 | Token | Token 计数与预算管理 | IMPLEMENTED |
| D2-S5 | Registry | 操作注册表 | IMPLEMENTED |
| D2-S6 | Snapshot | 上下文快照 | IMPLEMENTED |
| D2-S7 | Prompt | Prompt 模板管理 | IMPLEMENTED |
| D2-S8 | Sandbox | 工具沙箱隔离 | PLANNED (test only) |
| D2-S9 | Harness | 会话 Bootstrap、ToolPool、Preflight、System Prompt 装配（V5） | IMPLEMENTED |
| D2-S10 | QueryLoop | QueryLoop 运行时、UserContext、Attachments、PermissionMode、TaskTools（V6） | IMPLEMENTED |
| D2-S11 | Queue | SessionQueue、delegate-progress、task-notification drain（V6） | IMPLEMENTED |
| D2-S12 | Worktree | Delegate 沙箱工作目录 enter/exit（V6） | IMPLEMENTED |
| D2-S13 | Conversation | 会话对话管理 | IMPLEMENTED |
| D2-S14 | Mock | Context Engine Mock（测试辅助） | IMPLEMENTED |

### ORCH Orchestration (Cross-Domain, v2)

> v2 读模型包，非顶层 D1–D6；包路径 `internal/layers/orchestration/`。v3 可升格 D7。

| Module ID | Scenario | Responsibility |
|-----------|----------|----------------|
| ORCH-S1 | WorkPlan | Task + ExecutionFlow 读模型聚合 |
| ORCH-S2 | ExecutionFlowHub | FlowEvent 双通道：Leader Queue + D1 IM |
| ORCH-S3 | WaveScheduler | DAG 调度、ContextPolicy、ConflictGuard |

### D3 LLM Gateway Domain

| Module ID | Scenario | Responsibility |
|-----------|----------|----------------|
| D3-S1 | Adapter | 模型适配器 (DeepSeek, MiniMax) |
| D3-S2 | Gateway | LLM 路由与聚合 |
| D3-S3 | Breaker | 熔断器 |
| D3-S4 | Retry | 重试策略 |
| D3-S5 | Token | Token 计数 |
| D3-S6 | Config | 配置加载 |

### D4 Multi-Agent Domain

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D4-S1 | Factory | Agent 工厂 | IMPLEMENTED |
| D4-S2 | Agent | Agent 生命周期管理 | IMPLEMENTED |
| D4-S3 | ForkJoin | Agent Fork/Join、子 Agent 隔离、结果合并 | IMPLEMENTED |
| D4-S4 | Collaboration | CoT/Iterative-Refinement Prompt 增强 | IMPLEMENTED |
| D4-S5 | Observer | 事件观察者桥接 | IMPLEMENTED |
| D4-S6 | AgentTool | Agent 工具注册表 + CLI 适配器 | IMPLEMENTED |
| D4-S7 | Builtin | 内建 Agent 加载 | IMPLEMENTED |
| D4-S8 | AgentObservability | Agent 可观测性指标 | IMPLEMENTED |
| D4-S9 | SessionView | 会话视图 COW 快照 | IMPLEMENTED |
| D4-S10 | Delegate | Hub-Spoke 委派 Worker、delegate_* 工具、FlowBridge（V6） | IMPLEMENTED |

### D5 Observability Domain

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D5-S1 | Tracer | 分布式追踪 | IMPLEMENTED |
| D5-S2 | Metrics | 指标收集 | IMPLEMENTED |
| D5-S3 | Logger | 日志记录 | IMPLEMENTED |
| D5-S4 | Exporter | 数据导出 | IMPLEMENTED |
| D5-S5 | Coverage | 操作覆盖率 | IMPLEMENTED |
| D5-S6 | Telemetry | 遥测数据 | IMPLEMENTED |
| D5-S7 | Settings | 配置管理 | IMPLEMENTED |
| D5-S8 | Incident | 事件声明与处理 | IMPLEMENTED |
| D5-S9 | Runtime | 运行时指标监控 | IMPLEMENTED |

### D6 Evolution Domain

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D6-S1 | Version | 版本检测与记录 | PLANNED (v2.1.0) |
| D6-S2 | Config | 配置热更新 | PLANNED (v2.2.0) |
| D6-S3 | Eval | 评测引擎：EvalRun、Judge、探针、Delta、Tune、`devrix eval run` | IMPLEMENTED |
| D6-S4 | Orchestration | 运行时决策校验：跨模型判官、干预、Observer | IMPLEMENTED |

Spec: `openspec/specs/eval/spec.md` (D6-S3)

---

## Directory Structure Mapping

```
devrix/
internal/
layers/
├── communication/                  # D1
│   ├── gateway/                   # D1-S1
│   ├── adapters/                  # D1-S2
│   ├── milestone/                 # D1-S5
│   ├── ratelimit/                 # D1-S6
│   ├── metrics/                   # D1-S7
│   ├── renderers/                 # D1-S8
│   ├── eventbus/                  # D1-S9
│   ├── connection/                # D1-S10
│   ├── core/                      # D1-S11
│   └── instance/                  # D1-S12
│   # PLANNED: commands/ (D1-S3), auth/ (D1-S4)
│
├── contextengine/                 # D2
│   ├── compression/               # D2-S2
│   ├── memory/                    # D2-S3
│   ├── token/                     # D2-S4
│   ├── registry/                  # D2-S5
│   ├── snapshot/                  # D2-S6
│   ├── prompt/                    # D2-S7
│   ├── toolrunner/                # D2-S5/D2-S8 工具执行与沙箱
│   ├── harness/                   # D2-S9 (V5)
│   ├── query/                     # D2-S10 QueryLoop
│   ├── usercontext/               # D2-S10 UserContext
│   ├── attachments/               # D2-S10 Attachments
│   ├── permission/                # D2-S10 PermissionMode
│   ├── tasks/                     # D2-S10 TaskTools
│   ├── transcript/                # D2-S10 Sidechain
│   ├── queue/                     # D2-S11 SessionQueue
│   ├── worktree/                  # D2-S12 Worktree
│   ├── conversation/              # D2-S13 Conversation
│   └── mock/                      # D2-S14 Mock Engine
│   # PLANNED: sandbox/ (D2-S8)
│
├── orchestration/                 # ORCH (v2 read model)
│   ├── workplan/                  # ORCH-S1 WorkPlan
│   ├── flow/                      # ORCH-S2 ExecutionFlowHub
│   ├── imsink/                    # ORCH-S2 → D1 Gateway bridge
│   └── wave/                      # ORCH-S3 WaveScheduler
│
├── llmgateway/                    # D3
│   ├── adapter/                   # D3-S1
│   ├── gateway/                   # D3-S2
│   ├── breaker/                   # D3-S3
│   ├── retry/                     # D3-S4
│   ├── token/                     # D3-S5
│   └── config/                    # D3-S6
│
├── multiagent/                    # D4
│   ├── factory/                   # D4-S1
│   ├── agent/                     # D4-S2
│   ├── observer/                  # D4-S5
│   ├── tool/                      # D4-S6 AgentTool
│   ├── builtin/                   # D4-S7 Builtin
│   ├── observability/             # D4-S8 AgentObservability
│   ├── sessionview/               # D4-S9 SessionView
│   ├── delegate/                  # D4-S10 Delegate (V6)
│   ├── collaboration/             # D4-S4 Collaboration
│   └── permission/                # D4-S3 ForkJoin (code in agent/ forkjoin.go)
│
├── observability/                 # D5
│   ├── tracer/                    # D5-S1
│   ├── metrics/                   # D5-S2
│   ├── logger/                    # D5-S3
│   ├── exporter/                  # D5-S4
│   ├── coverage/                  # D5-S5
│   ├── telemetry/                 # D5-S6
│   ├── settings/                  # D5-S7
│   ├── incident/                  # D5-S8
│   └── runtime/                   # D5-S9
│
└── evolution/                     # D6
    ├── eval/                      # D6-S3 Eval engine + probes
    └── orchestration/             # D6-S4 Runtime judge / intervention
    # PLANNED: version/ (D6-S1), config/ (D6-S2)
```

---

## Activity (A) — Domain Activities

A 层定义每个场景下调用方可发起的**具体业务动作**。编号格式: `D{X}-S{X}-A{XX}`。

每个 Activity 具有明确的输入、输出和状态变更。A 层归属于对应的 D-S 场景。

**A 层注册表权威来源**: `openspec/a-registry.md`

当前注册: **77 个活动**，覆盖 7 个领域 + CROSS 跨域活动。

---

## Function Point (F) — Domain Functions

F 层定义可被 A 层活动编排的最小业务/技术逻辑单元。编号格式: `D{X}-S{X}-A{XX}-F{NN}`。

每个 Function Point 具有明确的输入、输出，且可独立测试。F 层归属于对应的 D-S-A 活动。

**F 层注册表权威来源**: `openspec/f-registry.md`

当前注册: **98 个功能点**，覆盖 7 个领域 + CROSS + Bridges。

---

## T 层测试点注册表

T 层测试点标准编号格式: `D{X}-S{X}-A{XX}-T{NN}`（DSAFT 标准）

- **D** = 域编号 (1-6)，与 D1-D6 对应
- **S** = 场景编号
- **A** = 活动编号 (01-99)
- **NN** = 测试序号 (01-99)

> **迁移状态**: ✅ 已完成 (2026-06-12)。所有 130+ 条目已升级为标准格式。遗留 ID 映射见 `openspec/t-registry.md` 文末。

示例:

- `D1-S1-A01-T01` = D1 (Communication) S1 (Gateway) A01 (ManageSession) 测试点 01
- `D2-S1-A01-T01` = D2 (Context Engine) S1 (PEV) A01 (ExecutePEV) 测试点 01
- `D3-S3-A01-T01` = D3 (LLM Gateway) S3 (Breaker) A01 (ManageCircuitBreaker) 测试点 01

完整注册表见 `openspec/t-registry.md`。

---

## Legacy ID Mapping

### L1-L2 → D-S（2026-06-08 迁移）

| Legacy | New | 说明 |
|--------|-----|------|
| L1-1 | D1 (COMM) | 通信域 |
| L1-2 | D2 (CTX) | 上下文域 |
| L1-3 | D3 (LLM) | LLM 网关域 |
| L1-4 | D4 (AGENT) | 多智能体域 |
| L1-5 | D5 (OBS) | 可观测域 |
| L1-6 | D6 (EVO) | 演化域 |
| L1-X-L2-Y | D{X}-S{Y} | 域-场景 |

### L4 Capability Mapping (Deprecated)

> **DEPRECATED**: 旧系统使用 `L4-{LAYER}-{NAME}` 格式，已废弃。请使用 D-S 编号系统。

| Legacy ID | D-S ID | 说明 |
|-----------|--------|------|
| L4-COMM-STORE | D1-S1 (Gateway) | 会话存储 |
| L4-COMM-CMD | D1-S3 (Commands) | 命令处理 |
| L4-CTX-PEV | D2-S1 (PEV) | PEV 引擎 |
| L4-CTX-COMPRESS | D2-S2 (Compression) | 压缩管道 |
| L4-CTX-HARNESS | D2-S9 (Harness) | Bootstrap 编排 |
| L4-CTX-TOOLPOOL | D2-S9 (Harness) | 可见工具集裁剪 |
| L4-CTX-ROUTER | D2-S9 (Harness) | Advisory 路由 hints |
| L4-CTX-PREFLIGHT | D2-S9 (Harness) | Pre-LLM 上下文评分 |
| L4-CTX-WORKSPACE | D2-S9 (Harness) | System Prompt 四层装配 |
| L4-CTX-TRANSCRIPT | D2-S9 (Harness) | Transcript / SessionLog |
| L4-CTX-QUERYLOOP | D2-S10 (QueryLoop) | QueryLoop 主循环 |
| L4-CTX-USERCTX | D2-S10 (QueryLoop) | UserContext prepend |
| L4-CTX-ATTACH | D2-S10 (QueryLoop) | Plan mode attachments |
| L4-CTX-PERM | D2-S10 (QueryLoop) | PermissionMode plan |
| L4-CTX-TASK | D2-S10 (QueryLoop) | Task disk tools |
| L4-CTX-SUBQUERY | D2-S10 (QueryLoop) | SubQuery / Fork |
| L4-CTX-QUEUE | D2-S11 (Queue) | SessionQueue drain |
| L4-CTX-WORKTREE | D2-S12 (Worktree) | Worktree sandbox |
| L4-AGENT-DELEGATE | D4-S10 (Delegate) | Hub-Spoke delegate |
| L4-ORCH-WORKPLAN | ORCH-S1 (WorkPlan) | 读模型聚合 |
| L4-ORCH-FLOWHUB | ORCH-S2 (ExecutionFlowHub) | FlowEvent 双通道 |
| L4-LLM-ADAPTER | D3-S1 (Adapter) | 模型适配器 |
| L4-OBS-TRACING | D5-S1 (Tracer) | 追踪 |

---

## Deprecated Specifications

以下文档已被本规范取代并删除（变更 `devrix-d-layer-rename`，DM-20260608-007）：

| 文件 | 原用途 | 状态 |
|------|--------|------|
| `layering-v2.md` | D-S-A-F-T 五层 ID 提案 | 已删除；A/F/T 层留待后续 |
| `layering-standard.md` | L1-L2 vs D-S-A-F-T 方案对比 | 已删除 |
| `MIGRATION.md` | L1-L2 迁移指南 | 已删除；映射见上文 Legacy 表 |

`openspec/changes/devrix-layering-standard/` 保留为历史提案记录（已搁置）。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-08 | Initial L1-L2 specification |
| 2.0.0 | 2026-06-08 | Renamed to D-S domains (DM-20260608-007); merged redundant architecture docs |
| 2.1.0 | 2026-06-10 | D2-S10~S12, D4-S10, ORCH-S1/S2 QueryLoop v2 (DM-20260610-012) |
| 3.0.0 | 2026-06-12 | DSAFT full A/F layer definition; A-registry (77 activities), F-registry (98 function points) |
| 3.1.0 | 2026-06-12 | Directory/spec sync: +14 new S-IDs, D4 conflict resolution (Permission→ForkJoin, Fork→Collaboration), +Status column on all S tables |
