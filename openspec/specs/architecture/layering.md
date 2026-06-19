# Devrix Domain Architecture Specification

**Capability:** architecture-layering
**Status:** Active
**Version:** 4.7.0
**Last Updated:** 2026-06-19

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
| **D2** | Context Engine Domain | CTX | Prepare / ToolRound / Persist、七步压缩、分层记忆、会话修复 |
| **D3** | LLM Gateway Domain | LLM | Model adapter, circuit breaker, token counter |
| **D4** | Multi-Agent Domain | AGENT | Agent lifecycle, fork, collaboration modes |
| **D5** | Observability Domain | OBS | Tracing, metrics, logging |
| **D6** | Evolution Domain | EVO | Eval engine, quality probes, runtime orchestration validation |
| **D7** | Orchestration Domain | ORCH | Task/Plan model, session orchestration, DAG scheduling, decision & planning |

---

## Scenario (S) — Domain Scenarios

每个 Domain 内部包含多个 Scenario，采用 **D{N}-S{M}** 格式编号。

### D1 Communication Domain

> **Domain SoT：** `openspec/specs/d1-communication/d1-domain.md`  
> **SoT（v3.4+）：** 价值流 Scenario 以 **D1-S13–S18** 为准（切法 A，DM-20260614-006）。  
> **v2.0（v3.5+）：** 代码包按价值流对齐；Legacy D1-S1–S12 索引已退役，见下方 Package Map。  
> **流程 / 可观测性指南：** `terminal-state-guide.md` · `observability-guide.md`（互补登记表，不重复 A/F/T 全表）。

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

> **Canonical SoT（价值流）：** D2-S15–S20（DM-20260614-009）。**Legacy Module Index：** D2-S1–S14 冻结追溯。详见 `openspec/specs/d2-context-engine/d2-domain.md`。

#### Canonical 价值流 — D2-S15–S20

| Scenario ID | Scenario | 用户/系统目标 | Status |
|-------------|----------|---------------|--------|
| D2-S15 | PrepareExecutionContext | Turn 前：加载、修复、压缩、组装 Prompt | REGISTRY |
| D2-S16 | ~~RunQueryLoop~~ | ~~LLM↔Tool 多轮执行原语~~ | **REMOVED → D7-S2-A06** |
| D2-S17 | PersistSessionState | Turn 后：快照/transcript durable + deferred complete | REGISTRY |
| D2-S18 | EnforceExecutionPolicy | 权限、沙箱、工具面、Plan 写限制 | REGISTRY |
| D2-S19 | NestedExecution | SubQuery / Background / Fork / Sidechain | REGISTRY |
| D2-S20 | ~~LegacyHarnessFallback~~ | ~~Harness 路径~~ | **REMOVED v6.5.0** |

#### Legacy Module Index — 冻结（D2-S1–S14）

| Module ID | Scenario | Responsibility | Status | Canonical |
|-----------|----------|----------------|--------|-----------|
| D2-S1 | PEV | Plan-Execute-Verify（**已退役**） | RETIRED | — |
| D2-S2 | Compression | 七步压缩管道 | IMPLEMENTED | → S15 |
| D2-S3 | Memory | 分层记忆管理 | IMPLEMENTED | → S15, S17 |
| D2-S4 | Token | Token 计数与预算 | IMPLEMENTED | → S15 |
| D2-S5 | Registry | 操作注册表 | IMPLEMENTED | → S18 |
| D2-S6 | Snapshot | 快照 + Main transcript | IMPLEMENTED | → S17 |
| D2-S7 | Prompt | Section 加载 + assembler | IMPLEMENTED | → S15 |
| D2-S8 | Sandbox | 工具沙箱隔离 | IMPLEMENTED | → S18 |
| D2-S9 | Harness | Bootstrap（legacy fallback） | IMPLEMENTED | → S20 |
| D2-S10 | QueryLoop | **REMOVED → D7-S2-A06** | — | → S16/S18/S19 |
| D2-S11 | Queue | SessionQueue、delegate-progress | IMPLEMENTED | → **D7-S4** |
| D2-S12 | Worktree | 沙箱工作目录 | IMPLEMENTED | → S18 |
| D2-S13 | Conversation | Tool chain repair / compact | IMPLEMENTED | → S15 |
| D2-S14 | Mock | 测试辅助 | IMPLEMENTED | Legacy only |

### ORCH Orchestration (Cross-Domain, v2)

> v2 读模型包，非顶层 D1–D6；包路径 `internal/layers/orchestration/`。v3 可升格 D7。

| Module ID | Scenario | Responsibility |
|-----------|----------|----------------|
| ORCH-S1 | WorkPlan | Task + ExecutionFlow 读模型聚合 |
| ORCH-S2 | ExecutionFlowHub | FlowEvent 双通道：Leader Queue + D1 IM |
| ORCH-S3 | WaveScheduler | DAG 调度、ContextPolicy、ConflictGuard |

### D3 LLM Gateway Domain

> **Canonical SoT（v3.8+）：** 价值流 Scenario 以 **D3-S1–S6** 为准（切法 A，5+1 S，DM-20260614-016 / DM-20260614-019 v2.0）。
> **v1.0：** 代码包保留技术角色词目录（`adapter/` `gateway/` `breaker/` `retry/` `token/` `config/` `safety/`）；运行时字符串、配置 key、metric 名 0 行为变更（ACCEPTED commit 199ad18）。
> **v1.1：** 韧性可见性（`llm_breaker_state` metric + D6 3 probe + IAdapter.Protocol() + obs nil fail-fast）ACCEPTED commit 3a6970b。
> **v2.0：** 物理目录 1:1 对齐 5+1 S（DM-20260614-019 / devrix-d3-sa-refine-v2.0 **ACCEPTED** commit d222328）；7 个旧路径保留 re-export 桥接 1 发布周期；scenario-slug 见 `code-layout.md §4.4`。
> **Legacy Index：** D3-S7（Safety）已并入 D3-S5（GuardContent）；历史 T ID 追溯见 `openspec/specs/d3-llm-gateway/t-registry.md §Legacy Archive`。
> 详见 `openspec/archive/2026-06-14-devrix-d3-sa-refine-v2.0/acceptance-report.md`。

#### Canonical 价值流 — D3-S1–S6（5+1 承诺型）

| Module ID | Scenario | 用户/系统目标 | scenario-slug (v2.0) | Status |
|-----------|----------|---------------|----------------------|--------|
| **D3-S1** | **RouteModel** | C1：用户给 model 名（含 tier alias），D3 返回正确 provider + 实际 model | `route/` | ✅ IMPLEMENTED（v2.0） |
| **D3-S2** | **StreamChat** | C2：用户发起流式聊天，D3 返回 OpenAI SSE 协议 chunk 流 | `stream/`（含 `stream/adapter/`） | ✅ IMPLEMENTED（v2.0） |
| **D3-S3** | **ProtectCall** | C3：Provider 故障（5xx/网络/限流），D3 不阻塞用户（Breaker + Retry + Fallback 合并） | `protect/`（两机制独立 .go） | ✅ IMPLEMENTED（v2.0） |
| **D3-S4** | **BudgetTokens** | C4：Token 超预算，D3 截断或报错，不超额调用 | `budget/` | ✅ IMPLEMENTED（v2.0） |
| **D3-S5** | **GuardContent** | C5：用户 prompt 命中危险模式，D3 拒绝（critical）或告警（warning） | `guard/` | ✅ IMPLEMENTED（v2.0） |
| **D3-S6** | **ConfigureGateway** | （横切）配置加载与验证；含 v1.1 3 feature flag | `configure/`（合并 `shared/config/llmgateway.go`） | ✅ IMPLEMENTED（v2.0） |

**Domain Kernel（非 S）：** `contracts.go`（145 行 < 200 行 ✅ AC-09；F9 完整拆分 deferred 至下 release）
**CROSS 锚点：** `internal/bridges/llm/`（D3-X 跨域锚点；R1 D2 决议；v2.0 ✅ 不动）

#### Package Map（v2.0 迁移完成 ✅）

> **v2.0** 6 个 slug 物理迁移完成（ACCEPTED commit d222328）；7 个旧路径保留 re-export 桥接 1 发布周期。

| v1.0 当前路径 | v2.0 目标 scenario-slug | Canonical S | v2.0 状态 |
|--------------|------------------------|-------------|----------|
| `gateway/router.go` (路由解析部分) | `route/router.go` | S1 RouteModel | ✅ 完成 |
| `adapter/` (全部) | `stream/adapter/` | S2 StreamChat | ✅ 完成 |
| `gateway/gateway.go` Stream 主实现 | `stream/gateway.go` | S2 StreamChat | ✅ 完成 |
| `gateway/breaker_observer.go` (v1.1 observer) | `protect/breaker_observer.go` | S3 ProtectCall | ✅ 完成 |
| `breaker/` + `retry/` | `protect/{circuit_breaker,state,observer,retry,retry_jitter}.go` | S3 ProtectCall | ✅ 完成（两机制独立 .go） |
| `token/` | `budget/{counter,bpe_loader}.go` | S4 BudgetTokens | ✅ 完成 |
| `safety/` | `guard/{filter,patterns}.go` | S5 GuardContent | ✅ 完成 |
| `config/` + `shared/config/llmgateway*.go` | `configure/{loader,shared_config}.go` + `llmgateway_features_test.go` | S6 ConfigureGateway | ✅ 完成（跨包合并） |
| 根 `contracts.go` 部分类型 | 根 kernel 保留（145 行 < 200）+ re-export | Domain Kernel | ✅ AC-09 达成（F9 完整拆分 deferred） |

#### Legacy Module Index — 冻结（D3-S1–S7 旧 7 S）

| Module ID | Scenario | Responsibility | Status | Canonical |
|-----------|----------|----------------|--------|-----------|
| D3-S1 (旧) | Adapter | 模型适配器 (DeepSeek, MiniMax) | IMPLEMENTED | → S2 StreamChat |
| D3-S2 (旧) | Gateway | LLM 路由与聚合 | IMPLEMENTED | → S1 RouteModel + S2 StreamChat |
| D3-S3 (旧) | Breaker | 熔断器 | IMPLEMENTED | → S3 ProtectCall |
| D3-S4 (旧) | Retry | 重试策略 | IMPLEMENTED | → S3 ProtectCall |
| D3-S5 (旧) | Token | Token 计数 | IMPLEMENTED | → S4 BudgetTokens |
| D3-S6 (旧) | Config | 配置加载 | IMPLEMENTED | → S6 ConfigureGateway |
| D3-S7 (旧) | Safety | 内容安全过滤 | IMPLEMENTED | → S5 GuardContent |

> **D3-S7 备注**：v2.1 spec.md 中 D3-S7 存在但 layering.md v3.6.0 未登记；v3.7.0 显式归档并指向新 D3-S5。

### D4 Multi-Agent Domain

> **Canonical S11–S16**（DM-20260614-018）。Legacy S1–S10 冻结追溯。Hub-Spoke 归 D7，见 `d4-multi-agent/d7-boundary.md`。

#### Canonical 价值流（SoT）

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D4-S11 | ProvisionAgent | 创建、配额、协作模式、Builtin 注册 | IMPLEMENTED (v2.0-d) |
| D4-S12 | RunAgentLoop | 生命周期、PermissionGate、状态机 | IMPLEMENTED (v2.0-d) |
| D4-S13 | IsolateAndMerge | Fork/Join、SessionView COW、WorkerEngine | IMPLEMENTED (v2.0-d) |
| D4-S14 | ExecuteWorker | Worker fork→run→join（D7 派发） | IMPLEMENTED (v2.0-d) |
| D4-S15 | InvokeExternalAgent | CLI/Cursor Agent Tool | IMPLEMENTED (v2.0-d) |
| D4-S16 | ConfigureAgents | multi_agent 配置 | IMPLEMENTED (v2.0-d) |

#### Legacy Module Index（冻结）

| Module ID | Scenario | Status | Canonical |
|-----------|----------|--------|-----------|
| D4-S1 | Factory | IMPLEMENTED | → S11 |
| D4-S2 | Agent | IMPLEMENTED | → S12, S13 |
| D4-S3 | ForkJoin | IMPLEMENTED | → S13 |
| D4-S4 | Collaboration | IMPLEMENTED | → S11 |
| D4-S5 | Observer | IMPLEMENTED | → kernel |
| D4-S6 | AgentTool | IMPLEMENTED | → S15 |
| D4-S7 | Builtin | IMPLEMENTED | → S11 + D7 |
| D4-S8 | AgentObservability | IMPLEMENTED | → D5 |
| D4-S9 | SessionView | IMPLEMENTED | → S13 |
| D4-S10 | Delegate | IMPLEMENTED | S14 + **D7 Hub-Spoke** |

### D5 Observability Domain

> **2026-06-15 SA Refine v1.0**: S 层从 9 技术模块重切为 4+1 价值流（DM-20260615-001）。Legacy S1–S9 冻结追溯。

**Canonical（v3.0）：**

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D5-S21 | Instrument | 遥测生成：Span + Metric + Log + 属性构建 | IMPLEMENTED |
| D5-S22 | Export | 遥测导出：OTLP / Prometheus / Console | IMPLEMENTED |
| D5-S23 | Diagnose | 诊断辅助：Coverage + Incident + Health | IMPLEMENTED |
| D5-S24 | Configure | 配置与运行时管理 | IMPLEMENTED |

**Legacy（D5-S1–S9，冻结追溯）：**

| Module ID | Scenario | Responsibility | Status | → Canonical |
|-----------|----------|----------------|--------|-------------|
| D5-S1 | Tracer | 分布式追踪 | LEGACY | → S21 |
| D5-S2 | Metrics | 指标收集 | LEGACY | → S21 |
| D5-S3 | Logger | 日志记录 | LEGACY | → S21 |
| D5-S4 | Exporter | 数据导出 | LEGACY | → S22 |
| D5-S5 | Coverage | 操作覆盖率 | LEGACY | → S23 |
| D5-S6 | Telemetry | 遥测数据 | LEGACY | → S21 |
| D5-S7 | Settings | 配置管理 | LEGACY | → S24 |
| D5-S8 | Incident | 事件声明与处理 | LEGACY | → S23 |
| D5-S9 | Runtime | 运行时指标监控 | LEGACY | → S24 |

### D6 Evolution Domain

> **2026-06-15 SA Refine v1.0**: S 层重切为 4 价值流（DM-20260615-002）。S4 "Orchestration" → S12 GuardRuntime 消除 D7 命名冲突。Legacy S1–S4 冻结追溯。

**Canonical（v3.0）：**

| Module ID | Scenario | Responsibility | Status |
|-----------|----------|----------------|--------|
| D6-S11 | RunEvaluation | 评测执行：EvalRun、Judge、探针、Delta、Tune、Dataset | IMPLEMENTED |
| D6-S12 | GuardRuntime | 运行时守护：Validator + Intervention + Observer | IMPLEMENTED |
| D6-S13 | TrackVersion | 版本检测与报告 | PLANNED |
| D6-S14 | ReloadConfig | 配置热更新 | PLANNED |

**Legacy（D6-S1–S4，冻结追溯）：**

| Module ID | Scenario | Responsibility | Status | → Canonical |
|-----------|----------|----------------|--------|-------------|
| D6-S1 | Version | 版本检测与记录 | LEGACY（PLANNED） | → S13 |
| D6-S2 | Config | 配置热更新 | LEGACY（PLANNED） | → S14 |
| D6-S3 | Eval | 评测引擎 | LEGACY | → S11 |
| D6-S4 | Orchestration | 运行时决策校验（**D7 命名冲突已消除**） | LEGACY | → S12 |

### D7 Orchestration Domain

> **2026-06-13**: 升格自 ORCH v2 读模型包。D7 作为编排域，是**横向协调层**，编排 D2+D4 跨域执行；**D1 仍拥有 ingress**，D7 不替代 D1 Gateway。

| Module ID | Scenario | Responsibility | Status | 来源 |
|-----------|----------|----------------|--------|------|
| D7-S1 | Work Model | Task/Plan 数据模型单一权威来源，生命周期管理 | DESIGN | D2 tasks/ + 新增 |
| D7-S2 | Session Orchestrator | 用户消息主入口，快速路径 + 编排路径路由 | DESIGN | D2 engine.go 入口上移 |
| D7-S3 | Wave Scheduler | DAG 调度、WorkerPool、ConflictGuard、ContextPolicy | DESIGN | ORCH-S3 升格 |
| D7-S4 | Execution Flow | FlowEvent 聚合、WorkPlan 快照、IM 进度广播 | DESIGN | ORCH-S1/S2 升格 |
| D7-S5 | Decision & Planning | 意图分类（规则+LLM）、任务拆解、执行器选择 | DESIGN | 新增 |

Spec: `openspec/specs/d7-orchestration/d7-domain.md` · Guides: `terminal-state-guide.md` · `observability-guide.md` (D7)

> **迁移说明（ORCH → D7）**: ORCH-S3 → D7-S3, ORCH-S1/S2 → D7-S4。迁移期间 `internal/layers/orchestration/` 保留作为 re-export 桥接包，D7 稳定后移除。

### D2/D7 Turn 双轨（DM-020 v1.0 Registry）

> **DM-020（D7 Turn 编排上移）：** Turn 主循环 SoT 从 D2-S16 迁移至 D7-S2-A06。v1.0 仅规格登记（零 Go 变更）；v2.0 物理迁移。

| 概念 | v1.0 Legacy（现行） | v2.0 Target（DM-020） |
|------|---------------------|----------------------|
| Turn 主循环 | D2-S16-A01 RunQueryLoop | **D7-S2-A06 RunTurnLoop** |
| LLM 调用 | D2 → D3（via bridges/llm） | **D7-S2-A07 InvokeLLM → D3**（D7 直调） |
| D2 角色 | Context Engine（含 LLM 调用） | **Context Follower**（Prepare / ToolRound / Persist） |
| D7 角色 | Session Orchestrator（路由 + Wave） | **Session Orchestrator + Turn Leader**（LLM 调用权 + Hub-Spoke） |
| D2→D3 import | ✅（现行） | **❌ 禁止**（v2.0-d CI 硬阻断） — **v2.3 已通过拆面闭合** |
| D2 拆面契约 | n/a | `shared/contracts.LLMCaller` + `shared/contracts.Summarizer`（D7 `turn.QueryLLMCaller` / `turn.CompressionSummarizer` 实现） |
| Autocompact | D2 调 D3 摘要 | **D7 调 D3**，D2 发出 CompressHint + 合并结果 |
| SubQuery LLM | D2 nested 内循环 | **D7 包装 TurnScopeSubQuery** |

**Stackelberg 博弈角色：**
- **D7 = Leader（先手）**：拥有 LLM 调用权 + Hub-Spoke 编排权，先选路径与 executor
- **D2 = Follower（后手）**：在给定参数下执行 Prepare / ToolRound / Persist，不拥有 LLM 调用权
- **D4 = Follower（后手，对称）**：执行 Worker，不拥有 Spoke 派发决策权

**Legacy 兼容（v2.0-f，已闭合 DM-20260618-010）：** `TurnExecutor` 适配 `TurnOrchestrator`；~~`QueryLoopExecutor`~~ 已删除。

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
| `type D2Executor interface` | `type TurnExecutor interface` | D7 契约：Turn 运行时入口 |
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
├── contextengine/                 # D2 (v2.2 final)
│   ├── prepare/                   # D2-S15 (orchestrator + adapters)
│   │   ├── memory/                # S15-A01+A02 read-side: recall.go (port only)
│   │   ├── compression/           # S15-A03
│   │   ├── conversation/          # S15 fork + repair
│   │   ├── prompt/                # S15-A04
│   │   ├── attachments/           # S10 attachments
│   │   ├── token/                 # S4 + windowanalyzer
│   │   ├── usercontext/           # S10
│   │   └── adapters/              # P1-b concrete port implementations
│   ├── persist/                   # D2-S17 (orchestrator + commit)
│   │   ├── snapshot/              # S6 snapshot
│   │   ├── transcript/            # S6 main + S10 sidechain
│   │   ├── commit.go              # S17-A04 CommitWindow
│   │   ├── orchestrator.go        # S17 wiring
│   │   └── memory/                # S17-A03 StoreLongTerm (P4 split)
│   ├── enforce/                   # D2-S18 (P3 物理归位)
│   │   ├── tools/                 # S5/S8 工具执行（was toolrunner/, P3-T2 rename）
│   │   │   └── surface/           # TOOL-SURFACE-1 v3 (DM-20260618-002)
│   │   ├── permission/            # S10 PermissionMode
│   │   ├── sandbox/               # S8 sandbox (P3-T1: was sandbox/)
│   │   ├── registry/              # S5 tool registry
│   │   ├── background_task_tools.go
│   │   ├── planmode_tools.go
│   │   ├── tool_filter.go
│   │   └── agent_role_filter.go
│   ├── kernel/                    # D2 contracts + span registry
│   ├── legacy/                    # P5 deprecated Process() entry (D2-STRUCT-T07)
│   │   └── engine.go              # Process() with slog.Warn
│   ├── mock/                      # D2 mockctx (test-only ports, kept in domain for cmd/obs-verify)
│   ├── contracts.go               # D2 root public API
│   ├── aliases.go                 # ContextEngine/EngineDeps deprecated aliases → legacy/
│   └── tool_context.go            # type alias → enforce/tools/context.go
│
├── orchestration/                # D7 Orchestration
│   ├── coordinator/               # D7 Session Orchestrator (package coordinator)
│   │   ├── orchestrator.go        # D7-S2 SessionOrchestrator + ProcessMessage (4-case switch → 4 独立执行链, DM-20260615-004)
│   │   ├── command_handler.go     # D7-S2 IntentCommand 显式分发（零 LLM, DM-20260615-004）
│   │   ├── orchestrate_path.go    # D7-S2 IntentOrchestrate 显式调 SynthesizeTaskGraph + WaveScheduler (DM-20260615-004)
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
│   ├── contracts.go              # Domain Kernel (Request/Chunk/TokenUsage/ToolCall)
│   ├── adapter/                   # D3-S1 旧 / S2 新 (v1.0 保留；v2.0 → stream/)
│   ├── gateway/                   # D3-S2 旧 (routing 部分 → S1 新；stream 部分 → S2 新)
│   ├── breaker/                   # D3-S3 旧 / S3 新 (v1.0 保留；v2.0 → protect/)
│   ├── retry/                     # D3-S4 旧 / S3 新 (v1.0 保留；v2.0 → protect/)
│   ├── token/                     # D3-S5 旧 / S4 新 (v1.0 保留；v2.0 → budget/)
│   ├── config/                    # D3-S6 旧 / S6 新 (v1.0 保留；v2.0 → configure/)
│   └── safety/                    # D3-S7 旧 / S5 新 (v1.0 保留；v2.0 → guard/)
│
├── bridges/                       # 跨域锚点
│   └── llm/                       # D3 → D2 Bridge (D3-X)
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
│   ├── observability.go           # Facade
│   ├── instrument/                # D5-S21
│   │   ├── tracer/
│   │   ├── metrics/
│   │   ├── logger/
│   │   └── telemetry/
│   ├── export/                    # D5-S22
│   ├── diagnose/                  # D5-S23
│   │   ├── coverage/
│   │   └── incident/
│   ├── configure/                 # D5-S24
│   │   ├── settings/
│   │   └── runtime/
│   ├── tracer/bridge.go           # (deprecated → instrument/tracer)
│   ├── metrics/bridge.go          # (deprecated → instrument/metrics)
│   ├── logger/bridge.go           # (deprecated → instrument/logger)
│   ├── telemetry/bridge.go        # (deprecated → instrument/telemetry)
│   ├── exporter/bridge.go         # (deprecated → export)
│   ├── coverage/bridge.go         # (deprecated → diagnose/coverage)
│   ├── incident/bridge.go         # (deprecated → diagnose/incident)
│   ├── settings/bridge.go         # (deprecated → configure/settings)
│   └── runtime/bridge.go          # (deprecated → configure/runtime)
│
└── evolution/                     # D6
    ├── evaluate/                  # D6-S11 RunEvaluation
    ├── guard/                     # D6-S12 GuardRuntime
    ├── eval/bridge.go             # (deprecated → evaluate)
    └── orchestration/bridge.go    # (deprecated → guard)
    # PLANNED: version/ (D6-S13), reload/ (D6-S14)
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
| 3.3.0 | 2026-06-13 | D7 Orchestration 升格自 ORCH v2 读模型包；D7-S1~S5 编排域 |
| 3.4.0 | 2026-06-14 | D1 价值流化（D1-S13~S18）；A/F/T 注册表增量 |
| 3.5.0 | 2026-06-14 | Naming Policy 增补：包名/文件名/导出类型/函数名禁用 D{N} 前缀（PR #33 反例表落地） |
| 3.6.0 | 2026-06-14 | D2 价值流化（D2-S15~S20）；D2-S1~S14 冻结追溯；Naming Policy 续补 |
| 3.7.0 | 2026-06-14 | **D3 5+1 S 价值流化**（DM-20260614-016 / devrix-d3-sa-refine）：D3-S1~S7 旧技术角色词 → D3-S1~S6 新 5+1 价值流承诺（RouteModel/StreamChat/ProtectCall/BudgetTokens/GuardContent/ConfigureGateway）；Legacy Index 冻结追溯；v1.0 物理路径保留 + v2.0 物理迁移目标 scenario-slug 注册（`route/` `stream/` `protect/` `budget/` `guard/` `configure/`）；D3 CROSS 锚点声明 `internal/bridges/llm/` |
| **3.8.0** | **2026-06-14** | **D3 v2.0 物理路径迁移完成**（DM-20260614-019 / devrix-d3-sa-refine-v2.0 ACCEPTED commit d222328）：6 个价值流 slug 物理迁移完成（`route/` `stream/` `stream/adapter/` `protect/` `budget/` `guard/` `configure/`）；8 个 re-export 桥接文件（旧路径保留 1 发布周期）；build/vet/test 全绿；contracts.go 145 行 AC-09 达成 |
| **3.9.0** | **2026-06-15** | **D4 v2.0-d 物理路径迁移完成**（commit 3905c6a）：S11–S16 + Kernel 物理迁移（`provision/` `run/` `isolate/` `execute/` `external/` `configure/` `kernel/`）；D7 Hub-Spoke bridge/dispatch/subquery 搬迁（`hubspoke/agent_bridge.go` `hubspoke/dispatch.go` `hubspoke/subquery_bridge.go`）；旧路径保留 re-export legacy.go；execute(9)+hubspoke(23) 测试新增；build/vet/test(71包) 全绿 |
| **4.0.0** | **2026-06-15** | **D4 v2.0-e 最终**（commits e30fe72..ffd6c56）：5 个 re-export shim 删除 + delegate/ 死代码删除（720a4b1）；E-e3 预存集成测试错误修复（60939db）；observer 引用迁移至 `kernel.NoOpAgentObserver`；根 `contracts.go` + `shared/config/multiagent.go` re-export 保留；S7 归档至 `openspec/archive/2026-06-15-devrix-d4-sa-refine/` |
| **4.1.0** | **2026-06-15** | **D7 Turn 编排上移 v1.0 Registry（DM-020）：** D2/D7 Turn 双轨声明；D2-S16 Legacy Freeze → D7-S2-A06 RunTurnLoop；LLM 调用权 D2→D7 产权转移；D2→D3 禁止；Follower 对称性（D2 Context / D4 Execution） |
| **4.2.0** | **2026-06-15** | **D5+D6 SA Refine v2.0 物理路径迁移完成**（DM-20260615-003）：D5 4 个 scenario 物理迁移（instrument/export/diagnose/configure）+ D6 2 个 scenario 物理迁移（evaluate/guard）；目录树更新；11 个 deprecated bridge.go |
| **4.3.0** | **2026-06-15** | **DM-020 D2→D3 拆面闭合**：D2 拆面契约上提至 `internal/shared/contracts/llm_facade.go`（`LLMCaller` + `Summarizer` 接口 + 辅助类型）；D7 `turn.QueryLLMCaller` / `turn.CompressionSummarizer` 实现并由 `bootstrap/context_engine.go` 单一注入点注入 `EngineDeps.QueryLLMCaller` / `EngineDeps.Summarizer`；D2 production wiring 零 D3 import；D2 `query/adapters.go` `NewLLMCaller(llmgateway.ILLMGateway)` 与 `compression/llm_summarizer.go` 标 Deprecated fallback（保留供内部测试用 mockctx.LLMGateway）；D7 turn/ 新增 9 个单元测试；build/vet/test -short 全工程绿；`lint/layer::TestD2_D3Ban` 通过（4 个白名单 = 4 个实际 fallback 路径） |
| **4.4.0** | **2026-06-15** | **DM-20260615-004 D7 Intent 路径正交化（v1.1.0 闭合）**：(1) `coordinator.command_handler.go` 新增 — IntentCommand 显式分发到 `PlanCLICommands` / `CLICommands`（零 LLM 成本）；(2) `coordinator.orchestrate_path.go` 新增 — IntentOrchestrate 显式调 `TaskDecomposer.SynthesizeTaskGraph` + `WaveScheduler.Start` + `WaitForCompletion`；(3) `coordinator.orchestrator.go::ProcessMessage` switch 4 case 改为 4 独立执行链（`CommandHandler` / `FastPath` / `OrchestratePath` + `IntentSkip` 内联），删除 v1.0 `handleCommand` / `orchestrate` 占位实现（system_prompt 字符串前缀让 LLM 自解释）；(4) D7 v1.0 "1 fastPath 占位 3 hint 前缀"临时妥协彻底关闭；(5) 9 个 P0 单测覆盖 3 新路径（3 + 5 + 1）；(6) `internal/layers/orchestration/workmodel/cli_commands.go` 导出 `Help()`（被 `CommandHandler.dispatch` 用于 `/help`）；(7) `NewSessionOrchestrator` 增加 lazy default（CommandHandler → `workmodel.GlobalTaskManager` + 新 PlanMode；OrchestratePath → 新 `TaskDecomposer` + 新 `WaveScheduler`），bootstrap 不必显式 wire 仍可启动；(8) build/vet/test -race 全工程绿，`TestD2_D3Ban` 不回归 |
| **4.5.0** | **2026-06-15** | **DM-20260615-004 跨文档同步**：`d2-context-engine/d2-domain.md` 状态表 3 项 ⬜ PLANNED → ✅ IMPLEMENTED（D2-S16 Legacy Freeze / D2→D3 import lint / S18 ExecuteToolRound 拆面，commit 41aec47 + `TestD2_D3Ban` 4 whitelist）；D7 目录树新增 `coordinator/command_handler.go` 与 `coordinator/orchestrate_path.go` 两文件（PR #35 引入但目录树尚未登记） |
| **4.6.0** | **2026-06-16** | **D1 领域文档同步**：新增 `d1-domain.md`（North Star + Out of Scope + 文档索引）；新增互补指南 `terminal-state-guide.md`（终态流程/时序）与 `observability-guide.md`（Span↔T/必达 Runbook）；D1 § 增加 Domain SoT 与 Guides 交叉引用 |
