# Context Engine Specification

**Capability:** context-engine
**Change ID:** devrix-context-engine (archived 2026-06-07)
**Layer:** 2
**Version:** 1.0.0
**Status:** Canonical — source of truth

---

## Overview

上下文引擎负责会话级消息历史、Token 预算、七步压缩与 PEV 执行循环，并通过 `IContextEngine.Process` 向通信层输出 `EngineEvent` 流。

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

系统 MUST 实现七步压缩管道；V1 MUST 执行步骤 1-5 和 7，跳过步骤 6。

**Priority**: P0
**Rationale**: 超长对话必须在 Token 预算内
**L3 映射**: L3-BE-CTX-02
**L4 映射**: L4-CTX-COMPRESS

#### Scenario: Step 1 Tool Result Budget

- GIVEN tool result messages exceed per-result budget
- WHEN compression step 1 runs
- THEN tool results are truncated
- AND truncation marker is appended

#### Scenario: Step 2 Snip

- GIVEN total tokens exceed compression target
- WHEN Snip runs
- THEN oldest messages are removed until within target
- AND minimum recent turns are preserved

#### Scenario: Step 3 Microcompact

- GIVEN consecutive messages share the same role
- WHEN Microcompact runs
- THEN they are merged into one message

#### Scenario: Step 4 Context Collapse

- GIVEN trivial short exchanges exist
- WHEN Collapse runs
- THEN trivial messages are folded
- AND substantive content is preserved

#### Scenario: Step 5 System Prompt Assembly

- GIVEN compressed messages and system prompt
- WHEN Assembly runs
- THEN system prompt is first
- AND messages remain chronological

#### Scenario: Step 6 Autocompact skipped in V1

- GIVEN V1 implementation
- WHEN pipeline reaches step 6
- THEN step is skipped
- AND skip is logged via observer

#### Scenario: Step 7 Token Block

- GIVEN compressed context still exceeds max budget
- WHEN TokenBlock runs
- THEN `ContextExceededError` is returned
- AND LLM is not invoked

#### Scenario: Compression triggered by threshold

- GIVEN token count exceeds `CompressionTarget`
- WHEN user message is processed
- THEN compression pipeline runs before LLM call
- AND compression report is emitted

---

### Requirement: PEV Engine (V1 Simplified)

系统 MUST 实现简化 PEV 循环：Execute → Verify（V1 无 Plan 阶段）。

**Priority**: P0
**Rationale**: 工具调用需要执行与基本验证闭环
**L4 映射**: L4-CTX-PEV
**L5 映射**: L5-CTX-06, L5-CTX-07, L5-CTX-11

#### Scenario: Execute phase invokes LLM

- GIVEN compressed context within budget
- WHEN Execute phase runs
- THEN `ILLMGateway.ChatStream` is called
- AND streaming chunks map to `thinking`/`text` events

#### Scenario: Tool call delegation

- GIVEN LLM response contains tool calls
- WHEN Execute handles tools
- THEN `tool_call` events are emitted for Gateway display
- AND `IPermissionGate.Request` is invoked synchronously before tool execution
- AND `IToolRunner` is invoked only when permission is approved
- AND `tool_result` events are emitted on success

#### Scenario: Tool call permission denied

- GIVEN LLM response contains tool calls
- WHEN `IPermissionGate.Request` returns false
- THEN an `error` EngineEvent is emitted with permission denied
- AND `IToolRunner` is not invoked
- AND partial assistant context is preserved

#### Scenario: Verify phase basic mode

- GIVEN tool execution completed
- WHEN Verify runs in `basic` mode
- THEN verification passes if tool result has no error
- AND on failure PEV may re-execute up to `max_iterations`

#### Scenario: PEV max iterations exceeded

- GIVEN Verify fails repeatedly
- WHEN iteration reaches `max_iterations`
- THEN `PEVMaxIterations` error is returned
- AND partial result is preserved in context

---

### Requirement: Layered Memory

系统 MUST 提供三层记忆模型；V1 MUST 实现 Working 与 Short-Term。

**Priority**: P0
**Rationale**: 分离临时状态与会话持久化
**L4 映射**: L4-CTX-MEMORY

#### Scenario: Working memory not persisted

- GIVEN active `Process` call
- WHEN working memory holds stream buffer and active tools
- THEN data is discarded after `Process` completes

#### Scenario: Short-term memory persisted

- GIVEN session context updated
- WHEN `Process` completes successfully
- THEN `Session.ContextSnapshot` is updated
- AND snapshot can reload on next message

#### Scenario: Long-term memory not available in V1

- GIVEN V1 implementation
- WHEN long-term memory recall is requested
- THEN `FeatureNotImplementedError` is returned

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

#### Scenario: Task flow milestone progress (V3 ready)

- GIVEN milestone progress update
- WHEN PEV Plan is integrated (V3)
- THEN `milestone_progress` events may be emitted
- AND metadata includes milestone_id and progress

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
