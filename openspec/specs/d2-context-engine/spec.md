# Context Engine Specification

**Capability:** context-engine
**Change ID:** devrix-context-engine (archived 2026-06-07), devrix-context-engine-v2 (archived 2026-06-07), devrix-context-engine-v3 (archived 2026-06-07), devrix-context-engine-v4 (archived 2026-06-08), devrix-harness-bootstrap (archived 2026-06-10), devrix-queryloop-context (archived 2026-06-10)
**Layer:** 2
**Version:** 7.2.0
**Status:** Canonical — source of truth
**Last Updated:** 2026-06-14
**Domain SoT:** `openspec/specs/d2-context-engine/d2-domain.md` (D2-S15–S20, DM-20260614-009)  
**D7 Boundary:** `openspec/specs/d2-context-engine/d7-boundary.md`

---

## Overview

上下文引擎负责会话级消息历史、Token 预算、七步压缩与 **QueryLoop** 多轮工具执行，并通过 `IContextEngine.Process` 向通信层输出 `EngineEvent` 流。

> **⚠️ LEGACY 标记（2026-06-17，DM-20260617-001）**：D2-S10 QueryLoop 主循环（`internal/layers/contextengine/query/loop.go::Loop.Run`）在 `loopFirst=false` 路径下被标 Deprecated。**canonical 主路径是 D7-S2-A06 RunTurnLoop**（`internal/layers/orchestration/turn/orchestrator.go`），`loopFirst=true` 是默认。Loop.Run 函数体逻辑保留（紧急回滚兜底），仅顶部加 metric 递增 + 一次性 slog.Warn。本 spec 章节（D2-S10）所有 Requirement 与 Scenario **保留** 用于回滚兼容，新能力**不得**依赖本路径。详见 `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC) 与 `openspec/changes/devrix-queryloop-legacy-decommission/`。

> **V7（2026-06-13，DM-20260611-004）**：`query_loop.enabled` 默认 `true`；Harness Bootstrap 降为 legacy fallback（仅 `query_loop.enabled=false` 时）。PEV（D2-S1）已退役。主路径为 QueryLoop（D2-S10）+ per-turn `commitActiveWindow` 压缩 + `conversation.RepairToolMessageChain`。

V2（DM-20260607-003）增强：Autocompact 步骤 6、PEV Verify `commands` 模式、Gateway `ITokenCounter` 统一、压缩/验证可观测性、主路径真实 LLM Gateway 接线。

V3（DM-20260607-006）增强：PEV Plan 阶段、Milestone DAG 驱动执行、`milestone_progress` 事件生产、LongTerm SQLite 跨 Session 记忆；`plan.enabled=false` 时保持 V2 Execute→Verify 路径。

V4（DM-20260608-003）增强：Autocompact 异步化（占位摘要 + 后台 LLM 摘要 + `OnAutocompactComplete`）、快照 Snappy 压缩（魔数头 + legacy JSON 兼容）。

V5（DM-20260609-004）增强：Harness Bootstrap 分阶段启动、ToolPool 过滤、SystemPromptAssembler 四层组装、Transcript 双轨、Preflight warn-only；`harness.enabled=false` 时保持 V4 bit-identical 行为。

V6（DM-20260610-012）增强：QueryLoop 运行时（`query_loop.enabled`）、UserContext API 边界 prepend、Plan Mode 附件与写过滤、Task 磁盘持久化、SubQuery/Fork/Background/Sidechain；v2.0 Hub-Spoke 增加 ExecutionFlowHub 双通道、WorkPlan 读模型、Worktree 沙箱目录；`query_loop.enabled=false` 时保持 V5 bit-identical 行为。

**Archive:** `openspec/archive/2026-06-08-devrix-context-engine-v4/`, `openspec/archive/2026-06-10-devrix-harness-bootstrap/`, `openspec/archive/2026-06-10-devrix-queryloop-context/`

---

## ADDED Requirements

### Requirement: Context Engine Core

The system MUST provide a real context engine implementation. When harness is enabled, Process MUST follow: LoadOrInit → Bootstrap (first Process) → LongTerm recall → compress messages only → SystemPromptAssembler.Build → QueryLoop.Run. When `harness.enabled=false`, behavior MUST remain bit-identical to V4.

系统 MUST 提供可替换 `StubContextEngine` 的真实上下文引擎；harness 启用时 MUST 按 design §1.3 时序执行。

**Priority**: P0
**Rationale**: Harness 正交注入不改变 QueryLoop 推理核，但需明确与压缩/组装的时序
**L3 映射**: L3-BE-CTX-01

#### Scenario: V4 compatibility with harness disabled

- GIVEN `context_engine.harness.enabled=false`
- WHEN any V4 T 层 scenario for Process runs
- THEN all V4 expectations still pass

#### Scenario: Process pipeline order when harness enabled

- GIVEN `context_engine.harness.enabled=true`
- WHEN Process runs one turn
- THEN execution order is LoadOrInit → Bootstrap (first time) → LongTerm Recall → append user message → compress messages → SystemPromptAssembler.Build → QueryLoop.Run → transcript append
- AND LongTerm recall MUST NOT mutate sc.SystemPrompt via string append (entries passed to Assembler)

#### Scenario: Process user message

- GIVEN an active session and non-empty user message
- WHEN `ContextEngine.Process` is called
- THEN a channel of `EngineEvent` is returned
- AND at least one `text` or `thinking` event is emitted before `complete`
- AND session context is updated in memory

#### Scenario: Process cancellation

- GIVEN `Process` is running
- WHEN the parent context is cancelled
- THEN event emission stops
- AND no panic occurs

#### Scenario: Error propagation

- GIVEN an unrecoverable error (e.g. context exceeded)
- WHEN processing fails
- THEN an `error` EngineEvent is emitted
- AND error code is included in Metadata

---

### Requirement: Session Context Initialization

系统 MUST 在新会话或空快照时初始化上下文。

**Priority**: P0
**Rationale**: 对话需要一致的 system prompt 与空历史基线
**L3 映射**: L3-BE-CTX-01
**T 映射**: D2-CTX-T01

#### Scenario: Initialize new session context

- GIVEN a session without `ContextSnapshot`
- WHEN context is loaded for the first time
- THEN working memory and short-term memory are created
- AND system prompt is loaded from configured sources
- AND message history is empty

#### Scenario: Restore session from snapshot

- GIVEN a session with valid `ContextSnapshot` (format `ctx-v1`, camelCase JSON per `ContextSnapshotV1`)
- WHEN context is loaded
- THEN message history is restored
- AND token budget state is restored
- AND processing can continue

---

### Requirement: Message History Update

系统 MUST 在每条用户/助手/工具消息后更新短期记忆。

**Priority**: P0
**Rationale**: 压缩与 LLM 调用依赖完整历史
**T 映射**: D2-CTX-T02

#### Scenario: Append user message

- GIVEN initialized session context
- WHEN user sends a message
- THEN message is appended with role `user`
- AND `UpdatedAt` is refreshed

#### Scenario: Append assistant and tool messages

- GIVEN PEV execute produces assistant text and tool results
- WHEN messages are recorded
- THEN assistant and tool messages are appended in order

---

### Requirement: Seven-Step Compression Pipeline

系统 MUST 实现七步压缩管道。V2 在 `autocompact.enabled=true` 且仍超 `CompressionTarget` 时执行步骤 6；否则跳过。

**物理执行顺序：** 1 → 2 → 3 → 4 → [6] → 5 → 7

**Priority**: P0
**L4 映射**: L4-CTX-COMPRESS
**T 映射**: D2-CTX-T03, D2-CTX-T04, D2-CTX-T08, D2-CTX-T12

#### Scenario: Step 6 Autocompact skipped

- GIVEN `autocompact.enabled` is false
- WHEN pipeline reaches step 6 slot (after step 4, before assembly)
- THEN step is skipped (`autocompact:skipped`)

#### Scenario: Step 6 Autocompact executed

- GIVEN V2 with `autocompact.enabled` true
- AND steps 1-4 applied on message history (no system prompt)
- AND token count still exceeds `CompressionTarget`
- WHEN step 6 runs
- THEN middle segment is replaced by LLM summary assistant message
- AND pipeline proceeds to step 5 Assembly

#### Scenario: Step 7 Token Block

- GIVEN compressed context still exceeds max budget
- WHEN TokenBlock runs
- THEN `ContextExceededError` is returned

---

### Requirement: Autocompact Compression Step

系统 MUST 在步骤 6 使用 LLM 对中间消息段生成结构化 JSON 摘要。

**Priority**: P0
**L4 映射**: L4-CTX-COMPRESS
**T 映射**: D2-CTX-T12, D2-CTX-T13

#### Scenario: Autocompact LLM failure degrades gracefully

- GIVEN autocompact is enabled
- WHEN LLM times out (>10s), fails, or returns invalid JSON after 1 retry
- THEN step 6 is skipped (`autocompact:degraded`)
- AND pipeline continues without panic

---

### Requirement: PEV Verify Commands Mode (RETIRED)

> PEV Verify 已随 D2-S1 退役。QueryLoop 不使用 verify commands 模式。

系统 MUST 支持 `verify_mode: commands`，运行白名单 `executable`+`args[]` 命令（禁止 shell）。

**Priority**: P0
**L4 映射**: L4-CTX-PEV
**T 映射**: D2-CTX-T14, D2-CTX-T15

---

### Requirement: Gateway Token Counter Integration

系统 MUST 通过 `shared/contracts.ITokenCounter` 注入 Gateway 计数器作为生产默认（`token_counter.source: gateway`）。

**Priority**: P0
**L4 映射**: L4-CTX-STATE, L4-CTX-COMPRESS
**T 映射**: D2-CTX-T16

---

### Requirement: PEV Engine (V3 Enhanced) (RETIRED)

> PEV 循环已移除；`query_loop.enabled=true`（默认）时使用 QueryLoop。

系统 MUST 实现 PEV 循环。`plan.enabled=true` 时支持 Plan → Execute → Verify（按 Milestone 拓扑序）；否则保持 V2 Execute → Verify。支持 `verify_mode: basic | commands | none`。

**Priority**: P0
**L4 映射**: L4-CTX-PEV, L4-CTX-PLAN
**T 映射**: D2-CTX-T06, D2-CTX-T07, D2-CTX-T11, D2-CTX-T14, D2-CTX-T15, D2-CTX-T19, D2-CTX-T20, D2-CTX-T24

#### Scenario: Verify phase basic mode

- GIVEN tool execution completed
- WHEN Verify runs in `basic` mode
- THEN verification passes if tool result has no error

#### Scenario: Verify phase commands mode

- GIVEN `verify_mode` is `commands`
- WHEN Verify runs after tool execution
- THEN whitelisted commands run in `session.WorkDir` via `exec.CommandContext`

#### Scenario: PEV max iterations exceeded

- GIVEN Verify fails repeatedly
- WHEN iteration reaches `max_iterations`
- THEN `PEVMaxIterations` error is returned

#### Scenario: Plan disabled preserves V2 behavior

- GIVEN `plan.enabled=false`
- WHEN Process handles user message
- THEN Plan phase is skipped
- AND Execute→Verify loop runs as V2

---

### Requirement: PEV Plan Phase (RETIRED)

> PEV Plan / Milestone DAG 已移除。D1 Milestone 模块仍独立存在。

系统 MUST 在 `plan.enabled=true` 时支持 PEV Plan 阶段，将用户意图分解为 Milestone DAG。

**Priority**: P0
**L4 映射**: L4-CTX-PLAN, L4-CTX-PEV
**T 映射**: D2-CTX-T19, D2-CTX-T25

#### Scenario: Plan generates milestone DAG

- GIVEN `plan.enabled=true` and user message requires multi-step work
- WHEN PEVEngine enters Plan phase
- THEN LLM produces structured milestone JSON
- AND milestones are validated (id format, dependency refs, acyclic DAG)
- AND milestones are created via `IMilestonePlanner`

#### Scenario: Plan validation failure degrades to V2

- GIVEN Plan LLM output fails validation
- WHEN validation detects cycle or invalid refs
- THEN error `CTX_PLAN_4020` is logged
- AND execution continues as V2 Execute→Verify without DAG

---

### Requirement: Milestone-Driven Execution

系统 MUST 按 Milestone DAG 拓扑序驱动 Execute→Verify，并更新进度。

**Priority**: P0
**L4 映射**: L4-CTX-PEV
**T 映射**: D2-CTX-T20, D2-CTX-T21

#### Scenario: Milestones execute in dependency order

- GIVEN a valid Milestone DAG for a task
- WHEN PEV runs after Plan
- THEN milestones execute in topological order
- AND blocked milestones wait until dependencies complete

#### Scenario: Milestone progress events emitted

- GIVEN milestone progress changes during PEV
- WHEN UpdateProgress or Complete is called
- THEN `milestone_progress` EngineEvent is emitted
- AND metadata includes `milestone_id`, `progress`, `task`

#### Scenario: Milestone verify failure fail-fast

- GIVEN verify fails after max iterations for a milestone
- WHEN retry budget is exhausted
- THEN `IMilestonePlanner.Fail` is called with reason
- AND subsequent milestones are skipped (fail_fast)

---

### Requirement: IMilestonePlanner Contract

Layer 2 MUST depend on `shared/contracts.IMilestonePlanner` rather than Communication layer internals.

**Priority**: P0
**L4 映射**: L4-CTX-PLAN
**T 映射**: D2-CTX-T19

#### Scenario: Context engine uses planner contract

- GIVEN context engine Plan phase
- WHEN milestones are created or updated
- THEN only `IMilestonePlanner` interface methods are invoked
- AND communication `milestone` package is not imported by L2

---

### Requirement: Compression and Verify Observability

系统 MUST 通过 `ICompressionObserver` 与 `IPEVObserver` 发射压缩步骤与 Verify 命令事件。

**Priority**: P1
**L4 映射**: L4-CTX-OBS
**T 映射**: D2-CTX-T17

---

### Requirement: Real LLM Gateway Wiring

系统 MUST 在主路径注入真实 LLM Gateway（`bridges/llm.WireContextLLM`），配置缺失时降级 Mock。

**Priority**: P1
**L4 映射**: L4-CTX-STATE
**T 映射**: D2-CTX-T18

---

### Requirement: Layered Memory

系统 MUST 提供三层记忆模型：Working、Short-Term（快照）、Long-Term（SQLite）。

**Priority**: P0
**Rationale**: 分离临时状态、会话持久化与跨 Session 项目知识
**L4 映射**: L4-CTX-MEMORY
**T 映射**: D2-CTX-T10, D2-CTX-T22, D2-CTX-T23

#### Scenario: Working memory not persisted

- GIVEN active `Process` call
- WHEN working memory holds stream buffer and active tools
- THEN data is discarded after `Process` completes

#### Scenario: Short-term memory persisted

- GIVEN session context updated
- WHEN `Process` completes successfully
- THEN `Session.ContextSnapshot` is updated
- AND snapshot can reload on next message

#### Scenario: LongTerm recall injects context

- GIVEN `longterm.enabled=true` and entries exist for query topic
- WHEN Process starts
- THEN Recall returns matching entries
- AND recalled content is injected into system prompt within token budget

#### Scenario: LongTerm store on completion

- GIVEN `longterm.auto_store=true` and topic in whitelist
- WHEN Process completes successfully
- THEN a memory entry is persisted to SQLite

#### Scenario: LongTerm disabled returns not implemented

- GIVEN `longterm.enabled=false`
- WHEN Recall is called on disabled backend
- THEN `CTX_MEMORY_4005` FeatureNotImplemented is returned

---

### Requirement: Gateway Event Contract

上下文引擎产出的事件 MUST 与通信层四流消费契约兼容。

**Priority**: P0
**Rationale**: UI/Adapter 已按 EngineEvent 类型渲染
**T 映射**: D2-CTX-T09, D2-CTX-T11

#### Scenario: Event flow types

- GIVEN normal processing
- WHEN events are emitted
- THEN types include `thinking`, `text`, `tool_call`, `tool_result`, `complete`
- AND optional `error` on failure

#### Scenario: EngineEvent field compatibility

- GIVEN the context engine emits events
- WHEN Gateway consumes events
- THEN `text` events use Metadata `is_complete` (`"false"` streaming, `"true"` final)
- AND `tool_call` events populate `ToolName`, `ToolInput` and Metadata `tool_name`, `input`, `risk_level`
- AND `complete` events include Metadata `usage` and `duration`
- AND Gateway does not block on `tool_call` for permission (display only)

#### Scenario: Process stop via context cancel

- GIVEN `Process` is running for a session
- WHEN Gateway `Stop(sessionID)` is called (e.g. `/stop` command)
- THEN the Process context is cancelled
- AND event emission stops without panic

#### Scenario: Task flow milestone progress

- GIVEN milestone progress update from PEV Plan/Execute
- WHEN milestone state changes
- THEN `milestone_progress` events are emitted
- AND metadata includes `milestone_id`, `progress`, and `task`

#### Scenario: Info flow

- GIVEN compression or PEV informational messages
- WHEN info should be shown to user
- THEN `info` type events may be emitted

---

## MODIFIED Requirements

### Requirement: IContextEngine Ownership

`IContextEngine` 接口 SHOULD 在 V1 实现完成后迁移至 `internal/shared/contracts` 或 `contextengine` 包导出，通信层仅依赖接口。

**Priority**: P1
**Rationale**: 避免 gateway 包承载引擎领域定义

#### Scenario: Communication imports engine interface

- GIVEN context engine is implemented
- WHEN communication gateway compiles
- THEN it imports only the interface package
- AND does not import compression or pev internals

---

## ADDED Requirements (V4 Performance)

### Requirement: Async Autocompact

当 `AsyncAutocompacter` 启用时，压缩管道 Step 6 MUST 在 50ms 内返回 head + 占位摘要 + tail；真实 LLM 摘要 MUST 在后台 goroutine 执行。同 session 重复触发 MUST 取消先前任务；仅最新 async token 的结果通过 `OnAutocompactComplete` 交付。`Shutdown` MUST 取消所有 pending goroutine。

**Priority**: P1  
**L4**: L4-CTX-COMPRESS  
**T**: D2-CTX-T31, D2-CTX-T33

#### Scenario: Placeholder returns immediately

- GIVEN a conversation exceeding compression target with autocompact enabled
- WHEN step 6 runs with async compactor wired
- THEN placeholder summary is returned within 50ms
- AND PEV loop is not blocked on LLM summarization

#### Scenario: Async failure degrades gracefully

- GIVEN an async autocompact is in progress
- WHEN the LLM call fails or times out
- THEN the placeholder remains in the conversation
- AND observer receives a degraded autocompact event

### Requirement: Snappy Snapshot Compression

`SnapshotConfig.compression` 启用时，大于 `compression_threshold` 的快照 MUST 以魔数 `\xfe\x53` + Snappy 编码存储；小于阈值或未启用时 MUST 保持 raw JSON。`Deserialize` MUST 兼容 legacy 未压缩 JSON。

**Priority**: P2  
**L4**: L4-CTX-MEMORY  
**T**: D2-CTX-T32

#### Scenario: Large snapshot compressed

- GIVEN compression enabled and payload exceeds threshold
- WHEN Serialize is called
- THEN output begins with snappy magic bytes
- AND compressed size is materially smaller than raw JSON

#### Scenario: Legacy snapshot readable

- GIVEN a legacy raw JSON snapshot without magic header
- WHEN Deserialize is called
- THEN session context is restored correctly

---

## ADDED Requirements (V5 Harness Bootstrap)

### Requirement: Harness Bootstrap Orchestration

系统 MUST 在 Context Engine 域提供分阶段 Harness Bootstrap 编排，对标 Claude Code Harness 启动模式。

**Priority**: P1
**Rationale**: Devrix V4 缺少 trust-gated 启动与工具面预装配，扩展 plugin/MCP 时无明确接入点
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**T 映射**: D2-S9-T01, D2-S9-T03, D2-S9-T08

#### Scenario: Bootstrap runs on first Process when enabled

- GIVEN `context_engine.harness.enabled=true`
- AND session has not completed harness initialization
- WHEN `ContextEngine.Process` is called
- THEN HarnessBootstrap runs stages prefetch → guards → setup → deferred_init → tool_pool
- AND bootstrap `info` events are emitted with stage metadata
- AND session is marked harness-initialized

#### Scenario: Bootstrap skipped when disabled

- GIVEN `context_engine.harness.enabled=false`
- WHEN `ContextEngine.Process` is called
- THEN no harness stages run
- AND behavior matches Context Engine V4

#### Scenario: Bootstrap idempotent per session

- GIVEN harness already initialized for session
- WHEN subsequent `Process` calls occur
- THEN full bootstrap is not re-run
- AND ToolPool visible tool set is reused from `HarnessSessionState`

---

### Requirement: Workspace Context Scan

系统 MUST 在 bootstrap prefetch 阶段扫描 session WorkDir 并构建 WorkspaceContext。

**Priority**: P1
**Rationale**: PEV 与 Assembler 需要工作区元信息用于 system 注入与调试
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**T 映射**: D2-S9-T02

#### Scenario: Scan work directory metadata

- GIVEN a valid session WorkDir
- WHEN prefetch stage runs
- THEN WorkspaceContext includes work dir path
- AND source/test file counts are populated
- AND AGENTS.md presence is detected

#### Scenario: Invalid work directory

- GIVEN WorkDir is missing or not readable
- WHEN prefetch stage runs
- THEN bootstrap continues with degraded WorkspaceContext
- AND a recoverable `info` event describes the degradation

---

### Requirement: Trust-Gated Deferred Initialization

系统 MUST 支持 trust-gated deferred init；非 trusted 模式下 MUST NOT 标记 plugin/skill/MCP 为已初始化。

**Priority**: P1
**Rationale**: IM/非受信入口不应预加载扩展面
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-HARNESS
**T 映射**: D2-S9-T04

#### Scenario: Trusted deferred init

- GIVEN `context_engine.harness.trusted=true` and `harness.deferred_init.enabled=true`
- WHEN deferred_init stage runs
- THEN DeferredInitResult marks plugin_init, skill_init, mcp_prefetch, session_hooks as enabled

#### Scenario: Untrusted deferred init skipped

- GIVEN `context_engine.harness.trusted=false`
- WHEN deferred_init stage runs
- THEN all DeferredInitResult flags are false
- AND no extension surfaces are loaded (V5a: stub only)

---

### Requirement: Tool Pool Filtering

系统 MUST 在 bootstrap 阶段按配置裁剪可见工具集，先于 PEV 工具 schema 暴露。

**Priority**: P0
**Rationale**: 启动期缩小工具面，减少误调用与 schema token 开销
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-TOOLPOOL
**T 映射**: D2-S9-T05, D2-S9-T14

#### Scenario: Simple mode restricts tool surface

- GIVEN `context_engine.harness.tool_pool.simple_mode=true`
- WHEN tool_pool stage runs
- THEN only Devrix builtin tools `bash`, `read_file`, `write_file` remain visible
- AND PEV receives filtered ToolSchema list (not full registry ListTools)

#### Scenario: MCP tools excluded

- GIVEN `context_engine.harness.tool_pool.include_mcp=false`
- WHEN tool_pool stage runs
- THEN tools whose name or description contains "mcp" are excluded

#### Scenario: Deny list filters tools

- GIVEN `deny_names` contains "bash"
- WHEN tool_pool stage runs
- THEN bash tool is not in visible tool set
- AND IPermissionGate is never invoked for denied tools

---

### Requirement: Advisory Prompt Routing

The system MUST support optional pre-LLM command/tool matching when routing is enabled, and MUST keep V4 behavior when routing is disabled. Routing hints MUST be advisory only and MUST NOT force PEV to invoke specific tools.

系统 MUST 在 routing 启用时支持 pre-LLM 匹配；禁用时 MUST 保持 V4 行为。

**Priority**: P2
**Rationale**: 高频命令可选缩短 LLM 选工具成本，不替代 tool calling
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-ROUTER
**T 映射**: D2-S9-T06

#### Scenario: Routing enabled produces hints

- GIVEN `context_engine.harness.routing.enabled=true`
- AND user message contains tool-related keywords
- WHEN Process prepares LLM context
- THEN RoutingHint lists matched tools with scores
- AND system prompt includes advisory routing block

#### Scenario: Routing disabled

- GIVEN `context_engine.harness.routing.enabled=false`
- WHEN Process runs
- THEN no routing hint is appended
- AND LLM tool selection is unchanged from V4

---

### Requirement: Transcript Lifecycle Separation

系统 MUST 维护独立于 CompressedView 的 TranscriptStore，用于 turn 级审计与 replay。

**Priority**: P1
**Rationale**: 压缩视图丢失原始 turn 细节，审计需要独立序列
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-TRANSCRIPT
**T 映射**: D2-S9-T07

#### Scenario: Append transcript on Process completion

- GIVEN `context_engine.harness.transcript.enabled=true`
- WHEN Process completes successfully
- THEN user message and assistant summary are appended to TranscriptStore
- AND Messages history remains compression pipeline input

#### Scenario: Transcript compact after threshold

- GIVEN transcript entries exceed `compact_after_turns`
- WHEN compact is triggered
- THEN only the most recent N entries are retained
- AND replay returns compacted sequence

#### Scenario: Messages and transcript on error path

- GIVEN Process fails before PEV completes
- WHEN error event is emitted
- THEN TranscriptStore MUST NOT append partial assistant turn
- AND Messages state matches V4 error semantics

---

### Requirement: Context Preflight Scoring

系统 MUST 在 PEV 前对上下文做规则 Preflight 评分（对齐 AgentScope agentscope-agent），默认 warn-only。

**Priority**: P1
**Rationale**: 在 reasoning 前发现 token/安全/工具面问题
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-PREFLIGHT
**T 映射**: D2-S9-T09

#### Scenario: Preflight warn-only emits warnings

- GIVEN `context_engine.preflight.enabled=true` and `preflight.mode=warn-only`
- WHEN provisional context (agentsRaw + memory entries) exceeds warn_ratio of token_budget
- THEN PreflightScores include degraded tokenBudget score
- AND warnings are emitted via info event
- AND Process continues to QueryLoop

#### Scenario: Tool filter auto-repair

- GIVEN `preflight.tool_filter.mode=auto-repair`
- AND user message is unrelated to tool "bash"
- WHEN preflight evaluates visible tools
- THEN bash is excluded from QueryLoop ToolSchema list
- AND info event records excluded tool names
- AND visible tool count MUST NOT drop below configured `preflight.tool_filter.min_tools` (default 1)

#### Scenario: Preflight block mode rejected in V5a

- GIVEN `context_engine.preflight.mode=block`
- WHEN application config is loaded
- THEN config validation MUST fail with clear error (V5b 再支持 block)

---

### Requirement: Workspace Structured Injection

系统 MUST 按 `design.md` §十 System Prompt Assembly Spec 组装最终 system prompt。

**Priority**: P0
**Rationale**: 分层 system prompt 分离人格、会话事实与 loaded context
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-WORKSPACE
**T 映射**: D2-S9-T10, D2-S9-T12, D2-S9-T13

#### Scenario: Four-layer prompt with loaded_context XML

- GIVEN harness enabled and AGENTS.md present
- WHEN SystemPromptAssembler.Build runs after message compression
- THEN output includes Layer 0 devrix_core, Layer 1 session_context, Layer 2 workspace_guidance
- AND Layer 3 contains `<loaded_context>` with non-empty `<agents_context>`
- AND sc.SystemPrompt is set before QueryLoop.Run

#### Scenario: Session context dynamic fields

- GIVEN a session with WorkDir and SessionID
- WHEN Build runs
- THEN Session Context section includes today's date, WorkDir absolute path, SessionID, and Model
- AND date reflects current Process time (not snapshot stale date)

#### Scenario: Memory token budget truncation

- GIVEN memory_context raw size exceeds remaining Layer 3 budget
- WHEN Build allocates tokens per §10.8 algorithm
- THEN memory_context is truncated with Chinese truncation notice
- AND BuildReport.memory_truncated is true

#### Scenario: V4 legacy build when harness disabled

- GIVEN harness.enabled=false
- WHEN Build runs
- THEN output equals AgentsRaw plus LongTerm appendix format from V4
- AND no XML or Session Context sections appear

#### Scenario: CompressedView system matches Assembler output

- GIVEN harness enabled and compression ran on messages only
- WHEN Build completes and CompressedView is assembled
- THEN CompressedView system message content equals Assembler output
- AND compression token budget excluded final XML system size (messages-only compression)

---

### Requirement: Dual Session Log Files

Transcript 分离 MUST 支持 compact view 与 append-only full log 双轨（对齐 AgentScope sessions 布局）。

**Priority**: P2
**Rationale**: 审计需要 append-only 完整 log，LLM 仅需 compact view
**L3 映射**: L3-BE-CTX-04
**L4 映射**: L4-CTX-TRANSCRIPT
**T 映射**: D2-S9-T07

#### Scenario: Append-only session log

- GIVEN `context_engine.harness.transcript.session_log_enabled=true`
- WHEN Process completes
- THEN user and assistant turns are appended to session log
- AND log is not replaced by compression pipeline

---

## ADDED Requirements (V6 QueryLoop — v1.0/v1.1)

### Requirement: QueryLoop Runtime

When `context_engine.query_loop.enabled=true`, PEV MUST delegate LLM↔Tool rounds to `query.Loop` instead of the legacy fixed-iteration execute loop. When `enabled=false`, behavior MUST remain bit-identical to V5.

**Priority:** P0  
**L3:** L3-BE-CTX-QueryLoop  
**L4:** query_loop  
**T:** D2-CTX-T34, D2-CTX-T39

#### Scenario: Multi-turn tool loop

- GIVEN `query_loop.enabled=true` and LLM returns tool_use until final text
- WHEN Process runs
- THEN Loop continues until no pending tool calls or `max_turns` reached
- AND `TurnCount` reflects tool rounds executed

#### Scenario: V5 regression with query loop disabled

- GIVEN `query_loop.enabled=false`
- WHEN any V5 T 层 scenario runs
- THEN behavior is unchanged from V5

---

### Requirement: UserContext Prepend Boundary

AGENTS.md and runtime user context MUST be injected via `usercontext.PrependForAPI` at the API boundary only when `user_context.mode=prepend`. They MUST NOT appear in persisted snapshot `Messages`.

**Priority:** P0  
**L4:** user_context  
**T:** D2-CTX-T35

#### Scenario: Prepend not in snapshot

- GIVEN prepend mode and non-empty AGENTS.md
- WHEN Loop builds API messages
- THEN prepend block is present in API call only
- AND snapshot messages exclude the prepend meta-user block

---

### Requirement: Permission Plan Mode

Plan mode MUST restrict writable paths to `PlanFilePath` and filter ToolPool to read-only tools plus plan file writes.

**Priority:** P0  
**L4:** permission_mode  
**T:** D2-CTX-T36, D2-CTX-T37

#### Scenario: Write denied outside plan file

- GIVEN `PermissionMode=plan` and configured plan file path
- WHEN `write_file` targets another path
- THEN tool returns plan mode denial without writing

---

### Requirement: Task Disk Persistence

When `tasks.mode=v2`, task_create/update/get/list MUST persist to `tasks.store_dir` and survive process restart.

**Priority:** P0  
**L4:** task_tools  
**T:** D2-CTX-T38

---

### Requirement: SubQuery and Sidechain Transcript

SubQuery MUST run nested agents via the same `query.Loop` with incremented `QueryDepth` and optional `AgentID`. Sidechain transcript MUST append JSONL under `{sessions}/{sessionId}/subagents/{agentId}.jsonl` when enabled.

**Priority:** P1  
**L4:** subquery, sidechain_transcript  
**T:** D2-CTX-T40, D2-CTX-T42

#### Scenario: Explore read-only sub-agent

- GIVEN builtin Explore agent invocation
- WHEN SubQuery runs
- THEN `OmitClaudeMd` applies and write tools are excluded

#### Scenario: Sidechain resume

- GIVEN existing sidechain JSONL for agentId
- WHEN SubQuery runs with `Resume=true`
- THEN initial messages include loaded sidechain history

---

### Requirement: Fork Subagent Cache Prefix

When `subquery.fork_subagent_enabled=true`, fork children MUST share identical assistant + placeholder tool_result prefixes; only the per-child directive differs.

**Priority:** P1  
**L4:** subquery  
**T:** D2-CTX-T41

#### Scenario: Identical placeholder tool results

- GIVEN parent assistant message with multiple tool_use blocks
- WHEN two fork children are built with different directives
- THEN placeholder tool_result text is identical across children

---

### Requirement: Streaming Tool Execution

When `query_loop.streaming_tools=true`, concurrency-safe tools in the same batch MAY execute in parallel; results MUST remain ordered in the transcript.

**Priority:** P1  
**L4:** query_loop

---

### Requirement: Background Task Notifications

Background SubQuery completion MUST enqueue `task-notification` commands scoped to the sub-agent's `agentId`; Loop MUST drain matching notifications each iteration.

**Priority:** P1  
**L4:** background_tasks

---

## ADDED Requirements (V6 QueryLoop v2 — ExecutionFlow & Worktree)

> Delegate Worker 约束与 `delegate_*` 工具见 `openspec/specs/multi-agent/spec.md` (D4-S10)。  
> WorkPlan / ExecutionFlowHub 契约见 `openspec/specs/orchestration/spec.md`。

### Requirement: ExecutionFlow Dual Channel

When `context_engine.execution_flow.enabled=true`, SubQuery and D4 Worker runtime events MUST publish unified `FlowEvent` records via `ExecutionFlowHub` to both Leader `SessionQueue` (`ModeDelegateProgress`) and D1 Gateway (`worker_progress`).

**Priority:** P0  
**L4:** execution_flow  
**T:** D4-S10-T04, D4-S10-T05, D4-S10-T06

#### Scenario: Leader-only delegate-progress drain

- GIVEN an in-progress Worker or SubQuery flow
- WHEN FlowEvent is published
- THEN Leader queue receives `delegate-progress` with empty AgentID
- AND Worker session queue does NOT drain delegate-progress for itself

#### Scenario: IM worker_progress projection

- GIVEN `execution_flow.im_progress=true`
- WHEN FlowEvent is published
- THEN Gateway emits `worker_progress` EngineEvent with flow metadata
- AND user remains in the same session thread (no second chat entry)

#### Scenario: D4 disabled fallback still emits flow events

- GIVEN `multi_agent.delegate.enabled=false`
- WHEN Leader invokes explore via SubQuery fallback
- THEN FlowEvents use `ExecutionSourceSubQuery`
- AND IM progress remains visible

---

### Requirement: Task Binding on Flow Start

When `execution_flow.link_tasks=true` and `FlowStarted` includes or resolves a `task_id`, Hub MUST set task owner to WorkerID and status to `in_progress`; completion events MUST update task status.

**Priority:** P0  
**L4:** execution_flow, task_tools  
**T:** D4-S10-T07

---

### Requirement: Worktree Sandbox Directory

When `context_engine.worktree.enabled=true`, delegate or implement workers MAY bind `WorkDir` to an isolated worktree under `worktree.base_dir`. Writes in worktree MUST NOT mutate the session's primary WorkDir.

**Priority:** P0  
**L4:** worktree  
**T:** D4-S12-T01

#### Scenario: Enter worktree isolates writes

- GIVEN worktree enabled and slug provided
- WHEN Worker runs write tools
- THEN files are created under worktree path only
- AND primary session WorkDir is unchanged after Worker completes

---

## ADDED Requirements (V7 Harness Unification)

### Requirement: QueryLoop Default Primary Path

`context_engine.query_loop.enabled` MUST default to `true`. Production Process MUST record `obsruntime.PathQueryLoop` unless operator explicitly sets `query_loop.enabled=false`.

**Priority:** P0  
**L4:** query_loop  
**T:** D2-S11-A01-T01, D2-S11-A01-T02, D2-S11-A01-D6PR

#### Scenario: Legacy harness path not taken by default

- GIVEN default `ContextEngineConfig`
- WHEN Process runs
- THEN legacy harness bootstrap branch is not executed
- AND PathRegressionProbe legacy_harness counter remains 0

### Requirement: Per-Turn Active Window Compression

When `query_loop.compress_per_turn=true`, Process MUST skip entry compression and MUST run `commitActiveWindow` after successful turns when message count or token budget exceeds limits.

**Priority:** P1  
**L4:** compression  
**T:** D2-S11-A01-T04

### Requirement: Conversation Tool Chain Repair

Before LLM API calls, Process MUST run `conversation.RepairToolMessageChain` on messages after the last compact boundary.

**Priority:** P0  
**L4:** conversation  
**T:** D2-S13-A01-T01

### Requirement: Main Thread Transcript

When `main_transcript.enabled=true`, Process MUST append message deltas to append-only JSONL at `{base_dir}/{sessionId}/transcript.jsonl` after successful turns (excluding worker overlay sessions).

**Priority:** P1  
**L4:** transcript  
**T:** D2-S6-A02-T01

### Requirement: Deferred Complete Event

Process MUST emit `complete` only after `sc.Messages` and `session.ContextSnapshot` persistence completes (or after user cancel clean exit).

**Priority:** P0  
**L4:** query_loop

---

## REMOVED Requirements

(None)
