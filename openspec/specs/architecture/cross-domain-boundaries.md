# Cross-Domain Boundaries Specification

**Capability:** architecture-cross-domain
**Status:** Active
**Version:** 1.6.0
**Last Updated:** 2026-06-16
**Parent:** `openspec/specs/architecture/layering.md`
**Change:** devrix-d3-sa-refine（DM-20260614-016 / R2 命题 E 决议落地）+ devrix-d3-sa-refine-v1.1（DM-20260614-017 / D6-A 决议固化 Breaker 事件命名 + §2.4.4 新增 D3→D5 metric 命名边界）+ DM-20260615-004 D7 Intent 路径正交化（§2.4.4 D7 ↔ D1/D2/D4 跨域锚点登记）

---

## 0. 目的

本文档明确 Devrix 6 域架构中**跨域 SoT（Source of Truth）边界**，避免：

- 同一概念在多个域内重复实现（重复契约）
- 同一职责在跨域边界的"踢皮球"（归责模糊）
- 长期边界演化的灰区被运行时具体决策模糊化（灰区声明）

> **何时更新本文件**：任何域内 spec 涉及**跨域 SoT**变更、新增/修改**跨域契约**、声明**灰区处理契约**时，必须同步更新本文档对应章节。

---

## 1. 跨域锚点总览

| 跨域路径 | 锚点 SoT | 涉及域 | 性质 |
|----------|----------|--------|------|
| `internal/bridges/llm/` | **D3 LLM Gateway** → D2 Context Engine | D3 + D2 | 跨域契约实现（Bridge 不属于任一域内） |
| `internal/bridges/orchestration/` | D7 Orchestration ↔ D1/D2 | D7 + D1 + D2 | 跨域锚点（同型） |
| `internal/shared/contracts/` | 跨域共享类型 | ALL | 全局类型 |
| `internal/shared/config/` | 跨域共享配置 | ALL | 全局配置 |

> **跨域锚点原则**（playbook 原则 4「跨域问题在 D 边界决策」）：跨域契约的实现**不属于任一域内部 A**，归属 `internal/bridges/` 或 `internal/shared/` 锚点。

---

## 2. D3 LLM Gateway 跨域边界

> D3 作为**公共域**（horizontal capability），被 D1/D2/D4/D5/D6/D7 任意域消费；本节明确 D3 与其他域的 SoT 划分。

### 2.1 D3 vs D2 (Context Engine) — DM-020 修订

#### 2.1.1 跨域锚点

| 锚点 | 路径 | D3 责任 | D2 责任 |
|------|------|---------|--------|
| `internal/bridges/llm/bridge.go` | D3 → **D7** 契约实现（DM-020） | D3 暴露 `IGateway` / `Request` / `Chunk` 类型 | — |

> **DM-020（D7 Turn 编排上移）修订：** `ILLMGateway` 主消费方从 D2 变更为 **D7**。D2 不再持有 `ILLMGateway` 依赖。Bridge 仍留 `internal/bridges/llm/`，跨域问题在 D 边界决策。

#### 2.1.2 SoT 划分

| 概念 | D3 SoT | D2 SoT | 备注 |
|------|--------|--------|------|
| `Request` / `Chunk` 类型 | **D3**（根 `contracts.go`） | D2 不重复定义，import 即可 | kernel 性质，跨域共享 |
| `ILLMGateway` 接口 | D3 实现（`bridge.go`） | ~~D2 定义~~ → **D7 定义**（DM-020） | 消费方迁移 |
| Turn 主循环 | D3 不参与 | ~~D2-S16 RunQueryLoop~~ → **D7-S2-A06 RunTurnLoop**（DM-020） | Turn SoT 归 D7 |
| LLM 调用编排 | **D3**（`D3-S2 StreamChat` + `D3-S3 ProtectCall`） | D2 **禁止** 调用 LLM（DM-020 import lint） | D7 编排 D3 调用 |
| 上下文组装 / 压缩 | D3 不参与 | **D2**（`D2-S15 PrepareExecutionContext`） | D2 SoT（保留） |
| CompressHint | D3 不参与 | **D2** 发出提议 | **D7** 执行压缩（调 D3），D2 合并 |
| 工具执行 | D3 不参与 | **D2**（`D2-S18 ExecuteToolRound`） | D2 SoT（保留） |
| 会话持久化 | D3 不参与 | **D2**（`D2-S17 PersistSessionState`） | D2 SoT（保留） |

> **禁止边（DM-020）：** `D2 → D3` 任何 import 或调用。v1.0 规格登记，v2.0 import lint CI 硬阻断。

#### 2.1.3 灰区声明：D3-S5 GuardContent vs D2-S18 PermissionMode（R2 命题 E 决议 / P0 #5）

> **场景 X**：用户 prompt 包含「用 curl 调用内部 API 拿 token」
> **场景 Y**：用户 prompt 包含「用 read_file 读 ~/.ssh/id_rsa」
> 这既是 prompt 内容（D3 决策）也是 tool execution policy（D2 决策），但**实际拒绝**只能发生一次。

**灰区处理契约**（R2 命题 E 决议）：

| 灰区场景 | D3 责任 | D2 责任 | 拒绝方 |
|----------|---------|---------|--------|
| 内容本身是 dangerous pattern | **D3-S5 GuardContent 拒绝**（前置过滤） | — | D3 优先 |
| 内容合法但 tool schema 不暴露 | — | **D2-S18 PermissionMode 拒绝**（tool schema 不暴露） | D2 兜底 |
| 内容合法 + tool 合法 + 实际执行违规 | — | **D2-S18 tool execution 拒绝**（运行时拦截） | D2 兜底 |

> **契约规则**：
> 1. **D3 优先拒**（前置过滤）：当 prompt 内容命中 D3 pattern，**先由 D3 拒**；不再流转到 D2 tool execution
> 2. **D2 兜底**（tool schema 不暴露）：即便 D3 通过，D2-S18 仍保留「tool schema 不暴露」作为兜底（防止 prompt injection 绕过 D3 后获取 tool 调用能力）
> 3. **不重复拒绝**：D3 拒后，D2 不会再次拒绝（避免双重错误码）；D2 拒后，D3 已通过的 `Filter.Check` 结果保留作为 audit log
> 4. **错误码归属**：`LLM_SAFETY_1011 ContentRejected` 属 D3；`LLM_PERM_2001 ToolNotAllowed` 属 D2

**为什么 D3 优先**：
- 前置过滤降低 latency（D3 拒比 D2 拒更早）
- 单一拒绝方简化错误归因
- D3 pattern 集合可独立扩展（D2 不依赖 D3 升级）

**为什么不放 D2**：
- D2-S18 PermissionMode 关注 tool schema，不关注 prompt 内容
- 内容过滤是 D3 网关边界（流式调用前最后一道门），属 D3 域内职责
- D2 没有 prompt 内容过滤的实现基础（依赖 D3 提供的 system prompt + messages）

**长期边界演化**（R3 命题 C NQ 衍生）：

- 若 V2+ 出现"D2 拒绝 → D3 需要重做 Filter.Check"反向需求时，重新评估 SoT
- v1.0 阶段契约稳定，不引入新跨域依赖

### 2.2 D3 vs D5 (Observability)

#### 2.2.1 跨域锚点

| 锚点 | 路径 | D3 责任 | D5 责任 |
|------|------|---------|--------|
| `internal/layers/observability/diagnose/coverage/registry.go` | D5 暴露 `Meter()` / `Tracer()` API | D3 调用 D5 API emit span / metric | D5 提供 OTel SDK 包装 |

#### 2.2.2 SoT 划分

| 概念 | D3 SoT | D5 SoT | 备注 |
|------|--------|--------|------|
| Span 名 / Attribute 名 | **D3**（`span-registry.md`） | D5 注册 OTel tracer / meter | D3 是命名 owner |
| Metric 名 / Label 维度 | **D3**（`span-registry.md §Metrics`） | D5 注册 Prometheus exporter | D3 是命名 owner |
| OTel SDK 选型 / Exporter 配置 | D3 不参与 | **D5** | D5 公共能力 |
| `d3_metric_emit_total{status=ok\|missing}` | **D3 emit** | D5 持久化 | v1.1 增量（R3 P1 #14） |

#### 2.2.3 灰区声明：Breaker state 可见性（R1 Q6 + R2 命题 B 决议）

> **场景**：D3 Breaker 状态切换（Closed ↔ Open ↔ HalfOpen）需要被 SRE 观测

**灰区处理契约**：

| 灰区点 | D3 责任 | D5 责任 | 通知方式 |
|--------|---------|---------|----------|
| Breaker 状态 metric | D3 emit `llm_breaker_state{provider, state}` | D5 写入 observability 持久化 | v1.1 启用（`d3_resilience_emit_enabled` feature flag，**默认 ON**） |
| Breaker 状态 → D7 通知 | D3 emit `flow.breaker.opened` / `closed` / `halfopened` | — | **不新增 D3→D7 直接契约**（R1 Q6 决议；D6-A 命名固化） |
| 启动期 obs 缺失 | D3 fail-fast（`ErrObservabilityRequired`，R3 P0 #8） | D5 readiness probe 失败时返回 error | D3 启动 fail；D5 不静默 |

**为什么 Breaker 状态 metric 走 D5 而非 D7 通知**：
- 状态是低频事件，但需要 dashboard 持久化查询（走 D5）
- 状态切换本身是 L4 事件流，复用 D7 EngineEvent 而非新增契约（R1 Q6 决议）
- D7 只在编排决策时订阅（无需高频轮询）

#### 2.2.4 v1.1 增量：D3 → D5 Metric 命名边界（D1-A 决议固化）

> **场景**：v1.1 新增 3 metric 的命名权与 label 维度约束

**灰区处理契约**（v1.1 落地）：

| Metric | D3 SoT（命名 + 维度） | D5 SoT（持久化 + 导出） | Cardinality 上限 |
|--------|----------------------|------------------------|------------------|
| `llm_breaker_state{provider, state}` | **D3 定义**（D1-A 决议） | D5 注册 Prometheus exporter | 2 provider × 3 state = 6 series（**provider 字段必须来自配置文件，不可动态生成**） |
| `llm_breaker_transitions_total{provider, from, to}` | **D3 定义**（D6 probe #2 配合） | D5 注册 Counter | 2 provider × 6 transitions = 12 series |
| `llm_tier_resolve_total{outcome}` | **D3 定义**（D6 probe #1 配合） | D5 注册 Counter | 3 outcomes = 3 series |
| `d3_safety_check_duration_ms` | **D3 定义**（D5-A 决议） | D5 注册 Histogram | 1 series |

**Cardinality 约束**：
- provider 字段来源：v1.1 release 时仅 2 provider（deepseek / minimax），配置驱动；不可运行时动态添加
- state 字段来源：3 个枚举值（closed / half_open / open），固定
- transitions 字段：6 种组合（closed→open, open→half_open, half_open→closed, half_open→open, open→closed, closed→half_open）

**为什么不放 D5**：
- D5 仅提供 OTel SDK 包装 + Prometheus exporter，不参与命名
- Metric 名是 D3 业务语义（breaker 状态、tier 解析、safety 延迟），D3 是 owner
- 长期：若 V2+ 出现 D5 需自定义 metric 维度（如 multi-cluster），由 D5 申请新 metric，不修改 D3 既有命名

### 2.3 D3 vs D6 (Evolution)

#### 2.3.1 跨域锚点

| 锚点 | 路径 | D3 责任 | D6 责任 |
|------|------|---------|--------|
| D6 Eval probe 注册 | `openspec/specs/d6-evolution/spec.md` §Probes | D3 暴露内部 metric / span 给 D6 探针 | D6 实施 probe 监控 |

#### 2.3.2 SoT 划分

| 概念 | D3 SoT | D6 SoT | 备注 |
|------|--------|--------|------|
| Probe 接入点（span / metric） | **D3 暴露** | D6 探针读 | 双方约定 |
| Eval 引擎选型 | D3 不参与 | **D6**（`D6-S3 Eval`） | D6 公共能力 |
| D6 probe #1：Tier 解析正确性 ≥ 99% | D3 metric `llm_tier_resolve_total{outcome=hit\|fallback\|error}` | D6 探针统计覆盖率 | **v1.1 实施**（D2-B 决议，FR-6） |
| D6 probe #2：Breaker 状态切换异常告警 | D3 metric `llm_breaker_transitions_total{provider, from, to}` | D6 探针统计跳变 | **v1.1 实施**（FR-7） |
| D6 probe #3：Token 预算触发率 | D3 span event `budget.check.exceeded` | D6 探针统计 | **v1.2 推迟**（D2-B 决议；与 D2-S4 Token 跨域协同） |
| D6 probe #4：Safety filter latency P99 < 1ms | D3 span event `safety.check.duration_ms` | D6 探针统计 P99 | **v1.1 实施**（D5-A 决议，FR-8） |

#### 2.3.3 灰区声明：probe 配置化 vs 硬编码

> **场景**：D6 probe 增删是否需要 D3 同步发布？

**灰区处理契约**：
- D6 probe 列表与告警阈值属 D6 域内（`d6-evolution/spec.md` 维护）
- D3 不在 `code-layout.md` 暴露 D6 probe 路径
- D3 仅暴露**稳定的 metric / span attribute**（R1 Q3 + R3 命题 B 衍生）
- D6 probe 接入需要 D3 配合时（如 v1.1 翻 `d3_resilience_emit_enabled`），**通过 D6 change 包**申请，不在 D3 域内

### 2.4 D3 vs D7 (Orchestration) — DM-020 修订

#### 2.4.1 D3 消费方迁移

> **DM-020（D7 Turn 编排上移）修订：** D7 替代 D2 成为 ILLMGateway 的**主消费方**。D7 直调 D3 进行 LLM 推理，D2→D3 禁止。

| 锚点 | 路径 | D3 责任 | D7 责任 |
|------|------|---------|--------|
| `internal/bridges/llm/bridge.go` | D3↔D7 契约实现 | D3 暴露 `IGateway` / `Request` / `Chunk` 类型 | D7-S2-A07 InvokeLLM 消费；D7-S2-A06 RunTurnLoop 编排 |
| D7 EngineEvent 复用 | `internal/layers/orchestration/coordinator/contracts.go` | D3 复用 `FlowStarted` / `FlowFailed` | D7 维护 EngineEvent 类型 |

#### 2.4.2 SoT 划分

| 概念 | D3 SoT | D7 SoT | 备注 |
|------|--------|--------|------|
| **LLM 调用权** | D3 提供 LLM 能力 | **D7** 唯一有权决定何时/如何调 D3（DM-020） | D2→D3 禁止（import lint） |
| 编排决策 | D3 不参与 | **D7**（`D7-S5 Decision & Planning`） | D7 SoT |
| RouteModel (tier 选择) | D3-S1 暴露 `ITierResolver` | **D7** InvokeLLM 前调用（DM-020 OQ4） | D7 决策，D3 执行 |
| Breaker 状态通知 D7 | D3 emit `flow.breaker.opened` / `closed` / `halfopened`（D6-A 决议） | D7 订阅并根据 Breaker 状态选择路由/fallback | DM-020 后 D7 是第一知情者 |
| 编排调度 | D3 不参与 | **D7**（`D7-S3 Wave Scheduler`） | D7 SoT |
| Turn 循环 | D3 不参与 | **D7-S2-A06** RunTurnLoop（DM-020） | D7 SoT |

#### 2.4.3 灰区声明：Breaker 事件命名（D6-A 决议固化 / v1.1 实施）

> **场景**：D3 通知 D7 "Breaker 状态切换"时，事件名是什么？

**灰区处理契约**（**D6-A 决议固化**，v1.1 实施）：

- **事件名**：`flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened`（3 事件分开）
- 复用现有 EngineEvent 字段（`SessionID` / `FlowID` / `Timestamp`）
- D3 通过 D3-S3-A01 F09 ReuseEngineEvent emit；D7 订阅方式不变
- 命名风格与现有 `FlowStarted` / `FlowFailed` 一致（`<noun>.<action>` 格式）
- D3 不持有"Breaker 事件命名 SoT"；命名由 D6 / D7 联合 review 后固化

**v1.1 R1 议题 D6 决议**：倾向 D6-A（3 事件分开）；Owner 评审通过；不再保留"v1.1 第一个 issue 决定"占位声明。

#### 2.4.4 D7 Intent 路径正交化（DM-20260615-004 跨域锚点登记）

> **场景**：D1 把 `InboundMessage` 路由至 D7 `coordinator.Entry.ProcessMessage` 后，D7 按 `IntentKind` 选择执行链。该选择决定了 **D3 是否被调用** 以及 **D2 / D4 参与方式**。

**4 IntentKind × 跨域 SoT（DM-20260615-004 v1.1.0+ 闭合）：**

| IntentKind | 执行链 | D3 调用 | D2 参与 | D4 参与 | 备注 |
|------------|--------|---------|---------|---------|------|
| `IntentSkip` | 内联 `close(ch)` | ❌ 零 LLM | ❌ | ❌ | 空消息或重复消息 |
| `IntentCommand` | `CommandHandler.Handle` → `PlanCLICommands` / `CLICommands` | ❌ **零 LLM** | ❌（plan 决策通过 D7-S5） | ❌ | `/plan` `/task` `/help` `/stop` 等显式命令 |
| `IntentFast` | `FastPath.Run` → `TurnOrchestrator` | ✅ 单轮 LLM↔Tool | ✅ Prepare / ToolRound / Persist | ❌ | 普通对话 |
| `IntentOrchestrate` | `OrchestratePath.Run` → `TaskDecomposer.SynthesizeTaskGraph` + `WaveScheduler.Start` + `WaitForCompletion` | ✅（per-task LLM） | ✅（per-worker Prepare / ToolRound / Persist） | ✅（per-worker ExecuteWorker + WorkerEngine） | 多任务长目标 |

**v1.0 → v1.1.0 关键变化（禁止退化）：**

- v1.0：`ProcessMessage` 4 case 共享 1 个 `FastPath` 占位 + `system_prompt` 字符串 hint（"[command:xxx]" / "[orchestrate]"），LLM 自解释
- v1.1.0+：4 case = **4 独立执行链**，`IntentCommand` **不调 LLM**（由 `CommandHandler` 显式分发），`IntentOrchestrate` **不经过 FastPath**（由 `OrchestratePath` 显式调 WaveScheduler）
- **禁止**：合并 case 4 → 1；重启 v1.0 占位 + hint 实现；`IntentFast` 改走 `OrchestratePath`（语义不一致）

**跨域契约影响（v1.1.0+）：**

- **D3 LLM 调用次数 metric**：`intent_kind` 标签必须分别采集 4 IntentKind；v1.0 混在 `intent_kind=fast` 单一桶的 metric 已废弃
- **D5 metric**：`intent_dispatch_total{kind=skip|command|fast|orchestrate}` 新增（`coordinator/orchestrator.go` 内 emit）
- **D2 拆面契约**：仅 `IntentFast` 与 `IntentOrchestrate` 触发 `QueryLLMCaller` / `CompressionSummarizer` 调用；`IntentCommand` 与 `IntentSkip` 路径不经过 D2
- **D4 拆面契约**：仅 `IntentOrchestrate` 触发 D4 worker dispatch；`IntentFast` 路径不直接 dispatch worker

> 详见 `d7-orchestration/spec.md` v2.5.0 §意图分类 + `d7-orchestration/dsaft-architecture.md` §意图分类。

### 2.5 D3 vs D1 (Communication)

> D1 作为入口与展示域的完整 North Star 与 Out of Scope 见 `openspec/specs/d1-communication/d1-domain.md`。

| 概念 | D3 SoT | D1 SoT | 备注 |
|------|--------|--------|------|
| LLM 调用结果展示 | D3 返回 `Chunk` / `Result` | **D1**（`D1-S16 DeliverConclusion` 渲染） | D1 SoT |
| Stream chunk → IM 渲染 | D3 透传原始 chunk | **D1** 适配（`D1-S17 ConnectChannel` 编码） | D1 SoT |
| User intent 解析 | D3 不参与 | **D7**（`D7-S5 ClassifyIntent`）；D1 仅 **Dispatch** | DM-007 修订 |

> **无跨域锚点**：D3 与 D1 间的传输是 D1 inbound → D7 orchestrator → D2 QueryLoop → D3 LLM Gateway，无直接契约。D1 不直接调用 D3。

### 2.6 D3 vs D4 (Multi-Agent)

| 概念 | D3 SoT | D4 SoT | 备注 |
|------|--------|--------|------|
| Agent 模型选择 | D3 暴露 `ResolveModel` API | **D4**（`D4-S4 Collaboration` 选 model） | D4 决策；D3 提供 |
| Sub-agent 隔离 | D3 不参与 | **D4**（`D4-S3 ForkJoin`） | D4 SoT |
| 委派调用 (Delegate) | D3 暴露 `IGateway` 供 D4 调 | D4 调 D3 | 无跨域锚点；通过 D2 / D7 编排 |

> **无跨域锚点**：D4 调用 D3 通过 D2 / D7 编排路径，不直连。

---

## 3. D4 Multi-Agent 跨域边界

> D4 作为 **Delegation Execution Follower**，Hub-Spoke **全归 D7**（DM-20260614-018 R1）。详表见 `openspec/specs/d4-multi-agent/d7-boundary.md`。

### 3.1 D4 vs D7 (Orchestration)

| 概念 | D4 SoT | D7 SoT | 备注 |
|------|--------|--------|------|
| delegate_* 工具路由 | ❌ | **D7-S2** `delegatetools/` | DM-011 已迁 |
| Spoke 派发 / fallback | ❌ | **D7-S2** DispatchWorker | v2.0 `hubspoke/dispatch.go` |
| Worker fork/run/join | **D4-S14** ExecuteWorker | 派发 | D7 → D4 契约 |
| FlowEvent 发布 | ❌（v2.0 后） | **D7-S4** SpokeBridge | v1.0 临时：D4 bridge + D2 flow_report |
| WorkPlan / delegate-progress | ❌ | **D7-S4** | `flow/`, `sessionqueue/` |
| Wave SubAgent 调度 | ❌ | **D7-S3** | 经 Dispatch 统一 |

### 3.2 D4 vs D2 (Context Engine)

| 概念 | D4 SoT | D2 SoT | 备注 |
|------|--------|--------|------|
| QueryLoop 执行 | ❌ | **D2-S16** | D4 Worker 经 IEngine |
| SubQuery 嵌套机制 | ❌ | **D2-S19** | 纯执行，无 Publish |
| SubQuery Flow 发布 | ❌ | ❌（迁 D7） | v2.0 `hubspoke/subquery_bridge.go` |
| SessionView / Fork COW | **D4-S13** | ❌ | D4 SoT |
| Builtin fallback 执行 | D4 `builtin/` 能力 | D2 SubQuery 执行体 | **路由在 D7** |

### 3.3 D4 vs D1 (Communication)

| 概念 | D4 SoT | D1 SoT | 备注 |
|------|--------|--------|------|
| Permission UI | ❌ | **D1** Gateway | D4 PermissionGate 阻塞 |
| worker_progress 展示 | ❌ | **D1-S15** | 数据来自 D7 FlowEvent |
| Agent Tool 子进程清理 | **D4-S15** | D1 Session 生命周期触发 | 协作清理 |

### 3.4 D4 vs D5 (Observability)

| 概念 | D4 SoT | D5 SoT | 备注 |
|------|--------|--------|------|
| agent.* span/metric 定义 | ❌ | **D5** | D4-S8 迁出 |
| Fork policy 计数 emit | D4 hook（v1.0） | D5 持久化 | v1.1 闭合 |

### 3.5 灰区：Hub-Spoke 写侧双头（v1.0 → v2.0 收敛）

| 灰区 | v1.0 现状 | v2.0 决议 |
|------|----------|----------|
| D4 FlowBridge vs D7 Hub | D4 直接 Publish | 迁 D7 `agent_bridge` |
| D2 flow_report vs D7 Hub | D2 直接 Publish | 迁 D7 `subquery_bridge` |
| DelegateOrFallback 在 D4 | D4 选 Spoke | 迁 D7 `dispatch` |

### 3.6 Follower 对称性声明（双边共识 G-02）

> **D2 和 D4 作为 Stackelberg Follower，享有对等的角色约束。** 该声明确保两个 Follower 的"瘦身"程度在每次架构审计中对称评估。

| 对称轴 | D2 Context Follower | D4 Execution Follower |
|--------|-------------------|---------------------|
| **不拥有编排决策权** | 不选 LLM 路径（DM-020） | 不选 Spoke 路径（DM-018） |
| **不直接 Publish FlowEvent** | 经 D7 PersistTurn 后 emit | 经 D7 SpokeBridge |
| **保留域内执行比较优势** | Prepare / ToolRound / Persist | Provision / Run / Isolate / ExecuteWorker |
| **硬约束（v2.0）** | import lint D2→D3 | import lint D4→orchestration/flow |
| **Follower Veto（合理拒绝权）** | D2-S18 Tool Permission Gate | D4-S12 PermissionGate |
| **禁止** | 直连 D3（ILLMGateway） | 直连 Hub（hub.Publish）/ 选择 Spoke |

> **影子编排风险**：即使 Hub-Spoke 归 D7，D4 仍可通过 Prompt 注入、Builtin 选择性注册、错误吞掉等方式间接影响编排。详表见 `openspec/specs/d4-multi-agent/d7-boundary.md` §9。

---

## 3. 跨域 SoT 决策原则

> **playbook 原则 4「跨域问题在 D 边界决策」**

1. **跨域 SoT 唯一**：每个概念只能有一个 SoT 域，其他域 import 而非重复定义
2. **跨域锚点不留域内**：跨域契约实现归属 `internal/bridges/` 或 `internal/shared/`，不留任一域内部
3. **灰区声明契约化**：跨域灰区处理必须写进本文档，运行时不能临时决策
4. **跨域变更同步**：任一域 spec 变更涉及跨域 SoT 时，必须同步更新本文档
5. **D{N} 编号不入代码**：跨域契约类型名遵循 `layering.md §命名规约`，禁止 `D3Config` 等

---

## 4. 灰区处理总览

| # | 灰区 | 涉及域 | 决议 | 文档位置 |
|---|------|--------|------|----------|
| 1 | D3-S5 vs D2-S18 内容/工具双重拒绝 | D3 + D2 | **D3 优先拒**，D2 兜底 | §2.1.3 |
| 2 | Breaker 状态可见性 | D3 + D5 + D7 | 走 D5 metric + 复用 EngineEvent | §2.2.3 + §2.4.3 |
| 3 | D6 probe 接入 | D3 + D6 | D6 维护 probe 列表；D3 暴露稳定 metric | §2.3.3 |
| 4 | Breaker 事件命名 | D3 + D7 | **D6-A 决议固化**：`flow.breaker.opened` / `closed` / `halfopened`（v1.1 实施） | §2.4.3 |
| 5 | D3→D5 metric 命名边界 | D3 + D5 | **v1.1 新增**：D3 定义命名 + 维度；D5 仅持久化 | §2.2.4 |
| 6 | **D7 Intent 路径正交化（DM-20260615-004）** | D7 + D2 + D4 | **4 IntentKind = 4 独立执行链**：IntentSkip 内联 / IntentCommand 走 `CommandHandler`（零 LLM）/ IntentFast 走 `FastPath.Run` → D3 + D2 / IntentOrchestrate 走 `OrchestratePath.Run` → SynthesizeTaskGraph + WaveScheduler（→D2/D4） | §2.4.4 + `d7-orchestration/spec.md` v2.5.0 + `d7-orchestration/dsaft-architecture.md` §意图分类 |

---

## 5. 关联文档

| 文档 | 路径 | 关系 |
|------|------|------|
| 分层 | `openspec/specs/architecture/layering.md` | D 域 SoT 划分 |
| 代码布局 | `openspec/specs/architecture/code-layout.md` | scenario-slug 与跨域锚点路径 |
| D3 spec | `openspec/specs/d3-llm-gateway/spec.md` | D3 域内 SoT（含 5+1 S） |
| D3 design | `openspec/specs/d3-llm-gateway/design.md` | D3 编排时序与跨域 emit |
| D3 span-registry | `openspec/specs/d3-llm-gateway/span-registry.md` | 跨 D5 span / metric 名 SoT |
| D7 编排 spec | `openspec/specs/d7-orchestration/d7-domain.md` | EngineEvent 类型 SoT |
| D5 可观测性 spec | `openspec/specs/d5-observability/` | OTel / Prometheus 配置 |
| D6 演化 spec | `openspec/specs/d6-evolution/spec.md` | probe 列表与告警阈值 |
| D2 上下文引擎 spec | `openspec/specs/d2-context-engine/d2-domain.md` | PermissionMode SoT |
| D1 通信 spec | `openspec/specs/d1-communication/d1-domain.md` | EngineEvent 展示、ingress D7-only SoT |
| D4 多智能体 spec | `openspec/specs/d4-multi-agent/d4-domain.md` | D4 Follower SoT |
| D4↔D7 边界 | `openspec/specs/d4-multi-agent/d7-boundary.md` | Hub-Spoke 全归 D7 |

---

## 6. 变更记录

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0.0 | 2026-06-14 | 初版：v3.7.0 layering.md 配套；D3 vs D1/D2/D4/D5/D6/D7 跨域 SoT 划分；4 项灰区契约化（D3-S5/D2-S18 / Breaker 状态 / D6 probe / Breaker 事件命名）；D3-X 跨域锚点声明 `internal/bridges/llm/` |
| 1.1.0 | 2026-06-14 | v1.1 子 change 落地：§2.4.3 Breaker 事件命名 D6-A 决议固化；§2.4.4 D3→D5 metric；§2.3.2 D6 probe；§4 灰区第 5 项 |
| 1.2.0 | 2026-06-14 | §3 D4 跨域边界（Hub-Spoke 全归 D7；D4 vs D7/D2/D1/D5）；灰区 Hub-Spoke 双头收敛表（DM-20260614-018） |
| 1.3.0 | 2026-06-15 | 双边共识落盘：§2.1 D3 vs D2 DM-020 修订（D2→D3 禁止、ILLMGateway 消费方 D7）；§2.4 D3 vs D7 DM-020 修订（D7 直调 D3）；§3.6 Follower 对称性声明 + 影子编排风险交叉引用 |
| 1.4.0 | 2026-06-15 | D5 SA Refine v1.0（DM-001 4+1 价值流 S21–S24）+ D6 SA Refine v1.0（DM-002 S11–S14；S4 Orchestration → S12 GuardRuntime）；现有 D5/D6 跨域边界（§2.2/§2.3）已在 D3 视角下覆盖，无需修改 |
| 1.5.0 | 2026-06-15 | DM-20260615-004 D7 Intent 路径正交化跨域登记：§2.4.4 新增 4 IntentKind × 跨域 SoT 表（D3 调用 / D2 参与 / D4 参与 + v1.0 → v1.1.0 关键变化 + 禁止退化）；§4 灰区新增第 6 项（D7 + D2 + D4 涉及）；意图分类合约化，禁止重启 v1.0 占位 + hint 实现 |
| 1.6.0 | 2026-06-16 | §5 关联文档新增 `d1-communication/d1-domain.md`（D1 Trusted Intermediary、ingress D7-only） |