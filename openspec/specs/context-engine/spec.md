# Context Engine Specification

**Capability:** context-engine
**Change ID:** devrix-context-engine (archived 2026-06-07), devrix-context-engine-v2 (archived 2026-06-07), devrix-context-engine-v3 (archived 2026-06-07)
**Layer:** 2
**Version:** 3.0.0
**Status:** Canonical — source of truth

---

## Overview

上下文引擎负责会话级消息历史、Token 预算、七步压缩与 PEV 执行循环，并通过 `IContextEngine.Process` 向通信层输出 `EngineEvent` 流。

V2（DM-20260607-003）增强：Autocompact 步骤 6、PEV Verify `commands` 模式、Gateway `ITokenCounter` 统一、压缩/验证可观测性、主路径真实 LLM Gateway 接线。

V3（DM-20260607-006）增强：PEV Plan 阶段、Milestone DAG 驱动执行、`milestone_progress` 事件生产、LongTerm SQLite 跨 Session 记忆；`plan.enabled=false` 时保持 V2 Execute→Verify 路径。

---

## ADDED Requirements

### Requirement: Context Engine Core

系统 MUST 提供可替换 `StubContextEngine` 的真实上下文引擎实现。

**Priority**: P0
**Rationale**: 当前仅 Echo stub，无法支撑对话与工具循环
**L3 映射**: L3-BE-CTX-01

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
**L5 映射**: L5-CTX-01

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
**L5 映射**: L5-CTX-02

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
**L5 映射**: L5-CTX-03, L5-CTX-04, L5-CTX-08, L5-CTX-12

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
**L5 映射**: L5-CTX-12, L5-CTX-13

#### Scenario: Autocompact LLM failure degrades gracefully

- GIVEN autocompact is enabled
- WHEN LLM times out (>10s), fails, or returns invalid JSON after 1 retry
- THEN step 6 is skipped (`autocompact:degraded`)
- AND pipeline continues without panic

---

### Requirement: PEV Verify Commands Mode

系统 MUST 支持 `verify_mode: commands`，运行白名单 `executable`+`args[]` 命令（禁止 shell）。

**Priority**: P0
**L4 映射**: L4-CTX-PEV
**L5 映射**: L5-CTX-14, L5-CTX-15

---

### Requirement: Gateway Token Counter Integration

系统 MUST 通过 `shared/contracts.ITokenCounter` 注入 Gateway 计数器作为生产默认（`token_counter.source: gateway`）。

**Priority**: P0
**L4 映射**: L4-CTX-STATE, L4-CTX-COMPRESS
**L5 映射**: L5-CTX-16

---

### Requirement: PEV Engine (V3 Enhanced)

系统 MUST 实现 PEV 循环。`plan.enabled=true` 时支持 Plan → Execute → Verify（按 Milestone 拓扑序）；否则保持 V2 Execute → Verify。支持 `verify_mode: basic | commands | none`。

**Priority**: P0
**L4 映射**: L4-CTX-PEV, L4-CTX-PLAN
**L5 映射**: L5-CTX-06, L5-CTX-07, L5-CTX-11, L5-CTX-14, L5-CTX-15, L5-CTX-19, L5-CTX-20, L5-CTX-24

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

### Requirement: PEV Plan Phase

系统 MUST 在 `plan.enabled=true` 时支持 PEV Plan 阶段，将用户意图分解为 Milestone DAG。

**Priority**: P0
**L4 映射**: L4-CTX-PLAN, L4-CTX-PEV
**L5 映射**: L5-CTX-19, L5-CTX-25

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
**L5 映射**: L5-CTX-20, L5-CTX-21

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
**L5 映射**: L5-CTX-19

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
**L5 映射**: L5-CTX-17

---

### Requirement: Real LLM Gateway Wiring

系统 MUST 在主路径注入真实 LLM Gateway（`bridges/llm.WireContextLLM`），配置缺失时降级 Mock。

**Priority**: P1
**L4 映射**: L4-CTX-STATE
**L5 映射**: L5-CTX-18

---

### Requirement: Layered Memory

系统 MUST 提供三层记忆模型：Working、Short-Term（快照）、Long-Term（SQLite）。

**Priority**: P0
**Rationale**: 分离临时状态、会话持久化与跨 Session 项目知识
**L4 映射**: L4-CTX-MEMORY
**L5 映射**: L5-CTX-10, L5-CTX-22, L5-CTX-23

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
**L5 映射**: L5-CTX-09, L5-CTX-11

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

## REMOVED Requirements

(None)
