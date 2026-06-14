# Devrix Domain Architecture Specification

**Capability:** architecture-layering
**Status:** Active
**Version:** 3.6.0
**Last Updated:** 2026-06-14

---

## Overview

本文档定义 Devrix 的正式分层架构，使用 **DSAFT 五层编号系统**：

- **D (Domain)** — 顶层领域，对应 `internal/layers/` 下的一级目录
- **S (Scenario)** — 域内场景，对应 **`{domain-slug}/{scenario-slug}/`** 二级目录（见 `code-layout.md` §4 注册表）

> **代码路径 SoT：** `openspec/specs/architecture/code-layout.md` — 领域名 / 场景名 → 目录映射、迁移状态、新文件决策树。
- **A (Activity)** — 可发起的业务动作，归属于 D-S 场景
- **F (Function Point)** — 最小业务/技术逻辑单元，被 A 层活动编排
- **T (Test Point)** — 测试点，标准格式 `D{X}-S{X}-A{XX}-T{NN}`

> A/F 注册表权威来源分别为 `openspec/a-registry.md` 和 `openspec/f-registry.md`。

---

## Domain (D) — Top-Level Domains

| Domain ID | 名称 | 缩写 | Responsibility |
|-----------|------|------|----------------|
| **D1** | Communication Domain | COMM | IM Gateway, WebSocket, CLI adapter |
| **D2** | Context Engine Domain | CTX | QueryLoop 主路径、七步压缩、分层记忆、会话修复 |
| **D3** | LLM Gateway Domain | LLM | Model adapter, circuit breaker, token counter |
| **D4** | Multi-Agent Domain | AGENT | Agent lifecycle, fork, collaboration modes |
| **D5** | Observability Domain | OBS | Tracing, metrics, logging |
| **D6** | Evolution Domain | EVO | Eval engine, quality probes, runtime orchestration validation |
| **D7** | Orchestration Domain | ORCH | Task/Plan model, session orchestration, DAG scheduling, decision & planning |

---

## Scenario (S) — Domain Scenarios

每个 Domain 内部包含多个 Scenario，采用 **D{N}-S{M}** 格式编号。

### D1 Communication Domain

> **SoT（v3.4+）：** 价值流 Scenario 以 **D1-S13–S18** 为准（切法 A，DM-20260614-006）。  
> **v2.0（v3.5+）：** 代码包按价值流对齐；Legacy D1-S1–S12 索引已退役，见下方 Package Map。  
> 详见 `openspec/changes/devrix-d1-sa-refine/design.md` §2.2。

#### Canonical — 价值流（D1-S13–S18）

| Module ID | Scenario | 用户目标 | Status |
|-----------|----------|----------|--------|
| D1-S13 | CaptureUserIntent | 我的指令一定进系统、查得到、能接着聊 | IMPLEMENTED |
| D1-S14 | PresentThinking | 我能看到它在想什么（信号①） | IMPLEMENTED |
| D1-S15 | PresentTaskProgress | 我能看到它在做什么任务（信号②） | IMPLEMENTED |
| D1-S16 | DeliverConclusion | 我能拿到针对我指令的总结（信号③ costly） | IMPLEMENTED |
| D1-S17 | ConnectChannel | 换 IM 平台，三类信息结构一致 | IMPLEMENTED |
| D1-S18 | GuaranteeDelivery | 弱网也不丢结论和错误 | IMPLEMENTED |

**Domain Kernel（非 S）：** `core.Card`、`types.Session`、`types.InboundMessage`

#### Package Map（v2.0 — 迁移中）

> **目标布局** 见 `code-layout.md` §5–§6。下表为 **当前实现路径** → **目标 scenario-slug** 速查。

| 当前路径 | 目标 scenario-slug | Canonical S |
|----------|-------------------|-------------|
| `gateway/` | `capture/` | S13, S18 overlay |
| `present/` | `thinking/` `taskprogress/` `conclusion/` | S14–S16 |
| `signal/` | `capture/signal/` | S14–S16 锚点 |
| `adapters/` … `renderers/` | `channel/` | S17 |
| `eventbus/` | `delivery/` | S18 |
| `core/` | `kernel/` | Domain Kernel |
| `orchestration/milestone/` | `orchestration/milestone/` | D7-S1 + S15-F03 |

#### Legacy Module Index — RETIRED（v2.0）

D1-S1–S12 已于 DM-20260614-006 Phase 3 退役。历史 T ID 追溯见 `openspec/specs/d1-communication/t-registry.md` §Legacy Archive。

### D2 Context Engine Domain

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D2-S1 | PEV | Plan-Execute-Verify 循环引擎（**已退役**，由 D2-S10 替代） | RETIRED |
| D2-S2 | Compression | 七步压缩管道 | IMPLEMENTED |
| D2-S3 | Memory | 分层记忆管理 (Working/LongTerm) | IMPLEMENTED |
| D2-S4 | Token | Token 计数与预算管理 | IMPLEMENTED |
| D2-S5 | Registry | 操作注册表 | IMPLEMENTED |
| D2-S6 | Snapshot | 上下文快照 + Main transcript JSONL | IMPLEMENTED |
| D2-S7 | Prompt | Section 加载 + `prompt/assembler` 四层装配 | IMPLEMENTED |
| D2-S8 | Sandbox | 工具沙箱隔离（`toolrunner/sandbox`） | IMPLEMENTED |
| D2-S9 | Harness | Bootstrap、Preflight、ToolPool（**legacy fallback**，`query_loop.enabled=false`） | IMPLEMENTED |
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

Spec: `openspec/specs/d6-evolution/spec.md` (D6-S3)

### D7 Orchestration Domain

> **2026-06-13**: 升格自 ORCH v2 读模型包。D7 作为编排域，是**横向协调层**，编排 D2+D4 跨域执行；**D1 仍拥有 ingress**，D7 不替代 D1 Gateway。

| Module ID | Scenario | Responsibility | Status | 来源 |
|-----------|----------|----------------|--------|------|
| D7-S1 | Work Model | Task/Plan 数据模型单一权威来源，生命周期管理 | DESIGN | D2 tasks/ + 新增 |
| D7-S2 | Session Orchestrator | 用户消息主入口，快速路径 + 编排路径路由 | DESIGN | D2 engine.go 入口上移 |
| D7-S3 | Wave Scheduler | DAG 调度、WorkerPool、ConflictGuard、ContextPolicy | DESIGN | ORCH-S3 升格 |
| D7-S4 | Execution Flow | FlowEvent 聚合、WorkPlan 快照、IM 进度广播 | DESIGN | ORCH-S1/S2 升格 |
| D7-S5 | Decision & Planning | 意图分类（规则+LLM）、任务拆解、执行器选择 | DESIGN | 新增 |

Spec: `openspec/specs/d7-orchestration/d7-domain.md` (D7)

> **迁移说明（ORCH → D7）**: ORCH-S3 → D7-S3, ORCH-S1/S2 → D7-S4。迁移期间 `internal/layers/orchestration/` 保留作为 re-export 桥接包，D7 稳定后移除。

---

## Naming Policy (Mandatory)

> **原则**：包名、文件名、导出类型/函数名 **一律使用语义命名**，**禁止**用领域 ID（D1–D7）作为代码标识符。`D{N}` 仅用于 DSAFT 跨团队架构对齐、文档与评审沟通，不进入代码命名空间。

### 命名映射表

| 域 ID | 语义名（用于包/文件/类型） | 严禁出现的位置 |
|-------|--------------------------|----------------|
| D1 | `communication` | ❌ `package d1` / `d1.go` / `type D1Config` |
| D2 | `contextengine` | ❌ `package d2` / `d2.go` / `type D2Config` |
| D3 | `llmgateway` | ❌ `package d3` / `d3.go` / `type D3Config` |
| D4 | `multiagent` | ❌ `package d4` / `d4.go` / `type D4Config` |
| D5 | `observability` | ❌ `package d5` / `d5.go` / `RegisterD5` / `D5Registered` |
| D6 | `evolution` | ❌ `package d6` / `d6.go` / `D6ValidationMetrics` |
| D7 | `orchestration`（协调层）/ `coordinator`（D7 入口子包） | ❌ `package d7` / `d7.go` / `D7Config` |

### 正例（Recommended）

```go
// 域 D7 — semantic path + semantic type
package coordinator                       // ✓ package = semantic role

type CoordinatorConfig struct { ... }      // ✓ type = semantic role
func NewSessionOrchestrator(...) ...       // ✓ function = semantic action

// 域 D5 — semantic file + semantic function
package runtime

type RuntimeMetric struct { ... }         // ✓ not "D5"
func RegisterRuntimeMetric(...) error      // ✓ not "RegisterD5"
func IncRuntimeMetric(p PathKind)          // ✓ not "IncD5"

// 域 D6 — semantic struct
package coordinator

type ValidationMetrics struct { ... }     // ✓ not "D6ValidationMetrics"
func NewValidationMetrics(...) *ValidationMetrics

// 域 D5 — semantic registry function
package metrics

func RegisterMultiAgentMetrics(...) *MultiAgentMetrics  // ✓ not "RegisterD5MultiAgent"
```

### 反例（Anti-Examples — 已废弃）

> 以下写法在历史归档与变更前出现过，**均已迁移**。未来代码评审见到任一项应直接拒绝。

| ❌ 反例 | ✅ 修正 | 说明 |
|---------|---------|------|
| `package d7` | `package coordinator` | 包名带域 ID，无法体现「coordinator of orchestration」语义 |
| `internal/layers/d7/` | `internal/layers/orchestration/coordinator/` | 路径带域 ID，与同层 `communication/contextengine/...` 不一致 |
| `d7.go`（共享配置） | `coordinator.go` | 文件名带域 ID，掩盖其作为「coordinator 包配置」的语义 |
| `d6_metrics.go` | `validation_metrics.go` | 文件名带域 ID，未体现 D6 validation metrics 的语义 |
| `d5_metric.go` | `runtime_metric.go` | 文件名带域 ID，应直接表达 runtime path metric 角色 |
| `d7_integration_test.go` | `coordinator_integration_test.go` | 测试文件名带域 ID，应表述其测试对象 |
| `type D7Config struct` | `type CoordinatorConfig struct` | 类型名带域 ID，Go 标识符严禁 D{N} 前缀 |
| `type D6ValidationMetrics struct` | `type ValidationMetrics struct` | 类型名带域 ID，应使用语义角色 |
| `func RegisterD5(...)` | `func RegisterRuntimeMetric(...)` | 函数名带域 ID，调用方应直接看到对象 |
| `func NewD6ValidationMetrics(...)` | `func NewValidationMetrics(...)` | 构造函数带域 ID |
| `func IncD5(...)` | `func IncRuntimeMetric(...)` | 公开函数带域 ID |
| `ConfigFile.D7` 字段 | `ConfigFile.Coordinator` 字段 | 字段名带域 ID（YAML tag 保持 `d7` 以兼容存量配置） |
| `type D2Executor interface` | `type QueryLoopExecutor interface` | D2 契约名带域 ID；语义「query loop 入口」更清晰 |
| `type D4Executor interface` | `type DelegateExecutor interface` | D4 契约名带域 ID；语义「delegate 委派」更清晰 |
| `type D1EventSink interface` | `type EventPublisher interface` | D1 契约名带域 ID；语义「事件发布者」已足够 |
| `type D6Validator interface` | `type AdvisoryValidator interface` | D6 契约名带域 ID；语义「advisory 验证器」更清晰 |
| `type D5Sink func(...)` | `type ObservabilitySink func(...)` | 包已经在 `observability/`，前缀 D5 冗余 |
| `func SetD7Entry(...)` | `func SetOrchestrationEntry(...)` | 网关 setter 名带域 ID；语义「orchestration entry」更清晰 |
| `func SetD5Sink(...)` | `func SetObservabilitySink(...)` | setter 名带域 ID |
| `func callD6Validator(...)` | `func callAdvisoryValidator(...)` | 内部方法名带域 ID |
| `field d6Metrics *ValidationMetrics` | `field validationMetric *ValidationMetrics` | 结构体字段名带域 ID |
| `field d7Entry / d7Enabled` | `field orchestrationEntry / orchestrationEnabled` | 字段名带域 ID |
| `InterruptOptions.D4Canceler` | `InterruptOptions.DelegateCanceler` | 字段名带域 ID |
| `Config.D6ValidationTimeoutMs` | `Config.AdvisoryValidationTimeoutMs` | 字段名带域 ID（YAML tag 保持 `d6_validation_timeout_ms`） |

### 例外（YAML tag / 运行时契约）

为兼容存量用户配置文件与 Prometheus 仪表盘，**运行时序列化字符串**可以保留 `D{N}` 前缀：

| 例外项 | 保留形式 | 原因 |
|--------|---------|------|
| `ConfigFile.D7 yaml:"d7"` 字段 yaml tag | `yaml:"d7"` | 存量 `devrix.yaml` 的 `d7:` 段不能改；v1.1 可迁移到 `coordinator:` |
| `orchestration.d6.validation.{pass,fail,timeout,error}` metric 名 | 完整 metric 名 | 仪表盘 / 告警规则已 grep 在该名 |
| `runtime_path_resolved_total` 等 metric 名 | 完整 metric 名 | 同上 |

> Go 标识符**总是**语义化；运行时字符串**暂时**可以保留 `D{N}`，并在迁移窗口关闭后清理。

### 历史反例归档

| Change | 反例文件/标识符 | 修正 | PR |
|--------|----------------|------|-----|
| `refactor/naming-semantic-no-domain-ids` | `d5_metric.go`, `d6_metrics.go`, `d7.go`, `d7_*_test.go` + `D5/D6/D7*` 标识符 | → `runtime_metric.go`, `validation_metrics.go`, `coordinator.go`, `coordinator_*_test.go` + `RuntimeMetric/ValidationMetrics/Coordinator*` | (本 PR) |
| `refactor/d7-to-orchestration-coordinator` | `internal/layers/d7/`, `package d7` | → `internal/layers/orchestration/coordinator/`, `package coordinator` | #31 |
| `fix/d7-package-coordinator-rename` | PR #31 的 sed 回退 | → 恢复 `package coordinator` | #32 |

---

```
devrix/
internal/
layers/
├── communication/                  # D1
│   ├── kernel/                    # Domain Kernel (Card, metadata)
│   ├── capture/                   # S13 CaptureUserIntent
│   │   └── signal/                # turn tracker
│   ├── thinking/                  # S14
│   ├── taskprogress/              # S15
│   ├── conclusion/                # S16
│   ├── channel/                   # S17 ConnectChannel
│   │   ├── adapters/
│   │   ├── connection/
│   │   ├── instance/
│   │   ├── ratelimit/
│   │   ├── renderers/
│   │   └── metrics/
│   └── delivery/                  # S18 GuaranteeDelivery
│       └── eventbus/
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
│   ├── transcript/                # D2-S6 Main + D2-S10 Sidechain
│   ├── queue/                     # D2-S11 SessionQueue
│   ├── worktree/                  # D2-S12 Worktree
│   ├── conversation/              # D2-S13 Conversation repair / compact boundary
│   └── mock/                      # D2-S14 Mock Engine
│   # engine.go 根包：Process 编排、commitActiveWindow、delegate_tools
│
├── orchestration/                # D7 Orchestration
│   ├── coordinator/               # D7 Session Orchestrator (package coordinator)
│   │   ├── orchestrator.go        # D7-S2 SessionOrchestrator + ProcessMessage
│   │   ├── workmodel.go           # D7-S1 WorkModel facade (v1.1 接管 storage)
│   │   ├── classifier.go          # D7-S5 RuleClassifier
│   │   ├── shadow_classifier.go   # D7-S5 Tail-only LLM Shadow (v1.1 兜底)
│   │   ├── fastpath.go            # D7-S2 FastPath proxy
│   │   ├── interrupt.go           # D7-S2 HandleInterrupt (Wave→D4→Process→stopped)
│   │   ├── contracts.go           # D7 Shared contracts (D7Entry seam)
│   │   ├── types.go               # D7 Intent/Event types
│   │   ├── config.go              # D7 Config (v1.0 defaults)
│   │   ├── d6_metrics.go          # D7-D6 4-counter + 滑窗告警
│   │   └── helpers.go             # D7 内部工具
│   ├── wave/                      # D7-S3 Wave Scheduler (升格自 ORCH-S3)
│   │   ├── scheduler.go
│   │   ├── pool.go
│   │   ├── taskgraph.go
│   │   ├── context.go
│   │   ├── conflict.go
│   │   ├── artifact.go
│   │   ├── types.go
│   │   ├── errors.go
│   │   └── runners/
│   │       ├── subagent.go
│   │       └── agent_tool.go
│   ├── flow/                      # D7-S4 ExecutionFlowHub (升格自 ORCH-S1/S2)
│   │   └── hub.go
│   ├── imsink/                    # D7-S4 IM 卡渲染 sink
│   │   └── gateway.go
│   └── workplan/                  # D7-S4 WorkPlan 快照
│       └── service.go
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

当前注册: **94 个活动**（含 3 个 D2-S1 RETIRED），覆盖 7 个领域 + CROSS 跨域活动。

---

## Function Point (F) — Domain Functions

F 层定义可被 A 层活动编排的最小业务/技术逻辑单元。编号格式: `D{X}-S{X}-A{XX}-F{NN}`。

每个 Function Point 具有明确的输入、输出，且可独立测试。F 层归属于对应的 D-S-A 活动。

**F 层注册表权威来源**: `openspec/f-registry.md`

当前注册: **169 个功能点**，覆盖 7 个领域 + CROSS + Bridges。

---

## T 层测试点注册表

T 层测试点标准编号格式: `D{X}-S{X}-A{XX}-T{NN}`（DSAFT 标准）

- **D** = 域编号 (1-6)，与 D1-D6 对应
- **S** = 场景编号
- **A** = 活动编号 (01-99)
- **NN** = 测试序号 (01-99)

> **迁移状态**: ✅ 已完成 (2026-06-12)。域级合计 **195** 条 T 点（186 IMPLEMENTED · 7 PLANNED · 1 PARTIAL）。遗留 ID 映射见 `openspec/t-registry.md` 文末。

示例:

- `D1-S1-A01-T01` = D1 (Communication) S1 (Gateway) A01 (ManageSession) 测试点 01
- `D2-S10-A01-T34` = D2 (Context Engine) S10 (QueryLoop) A01 测试点 34（原 PEV Execute 能力）
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
| L4-CTX-WORKSPACE | D2-S7 (Prompt) | System Prompt 四层装配（`prompt/assembler.go`） |
| L4-CTX-TRANSCRIPT | D2-S6 / D2-S9 | Main JSONL + Harness Transcript |
| L4-CTX-CONV | D2-S13 (Conversation) | Tool chain repair / compact boundary |
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
| 3.2.0 | 2026-06-13 | D2 文档与代码对齐：QueryLoop 默认主路径、PEV 退役、`prompt/assembler`、Main transcript、conversation repair；A 94 / F 169 / T 195 |
