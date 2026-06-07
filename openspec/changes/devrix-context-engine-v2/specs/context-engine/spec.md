# Context Engine V2 Delta Specification

**Capability:** context-engine
**Change ID:** devrix-context-engine-v2
**Layer:** 2
**Version:** 2.0.0
**Base Spec:** `openspec/specs/context-engine/spec.md`

---

## Overview

V2 在 V1 基础上增强：压缩步骤 6 Autocompact、PEV Verify `commands` 模式、Token 计数与 LLM Gateway 统一、压缩/验证可观测性。

---

## ADDED Requirements

### Requirement: Autocompact Compression Step

系统 MUST 在压缩管道步骤 6 使用 LLM 对中间消息段生成结构化摘要，以替代硬截断。

**Priority**: P0
**Rationale**: V1 跳过步骤 6 导致超长对话语义丢失
**L3 映射**: L3-BE-CTX-02
**L4 映射**: L4-CTX-COMPRESS
**L5 映射**: L5-CTX-12, L5-CTX-13

#### Scenario: Trigger autocompact when over budget after steps 1-5

- GIVEN `autocompact.enabled` is true
- AND steps 1-5 have been applied
- AND message count is at least `min_messages_for_summary`
- AND token count still exceeds `CompressionTarget`
- WHEN compression step 6 runs
- THEN `ILLMGateway.ChatStream` is called with `LLMRequest.Model` set to `autocompact.model`
- AND middle message segment is replaced by a summary assistant message
- AND `CompressionReport.StepsApplied` includes `autocompact`
- AND total token count decreases

#### Scenario: Autocompact preserves head and tail turns

- GIVEN autocompact is triggered
- WHEN summary is generated
- THEN first `preserve_head_turns` and last `preserve_tail_turns` message groups are unchanged
- AND only the middle segment is summarized

#### Scenario: Autocompact LLM failure degrades gracefully

- GIVEN autocompact is enabled
- WHEN LLM call fails or returns invalid JSON after retry
- THEN step 6 is skipped without panic
- AND `CompressionReport.StepsApplied` includes `autocompact:degraded`
- AND pipeline continues to step 7 TokenBlock

#### Scenario: Autocompact disabled skips step 6

- GIVEN `autocompact.enabled` is false
- WHEN compression pipeline runs
- THEN step 6 is skipped
- AND behavior matches V1 (`autocompact:skipped`)

---

### Requirement: PEV Verify Commands Mode

系统 MUST 支持 `verify_mode: commands`，在工具执行后运行配置化白名单命令验证结果。

**Priority**: P0
**Rationale**: basic 模式无法验证代码变更副作用
**L3 映射**: L3-BE-CTX-01
**L4 映射**: L4-CTX-PEV
**L5 映射**: L5-CTX-14, L5-CTX-15

#### Scenario: All verify commands pass

- GIVEN `verify_mode` is `commands`
- AND `verify_policy` is `all_pass`
- AND tool execution completed without error
- WHEN Verify phase runs
- THEN each configured verify command is executed in `session.WorkDir`
- AND all commands exit with code 0
- AND `VerifyResult.Passed` is true

#### Scenario: Verify command failure triggers retry

- GIVEN `verify_mode` is `commands`
- AND a verify command exits non-zero
- WHEN Verify phase runs
- AND current iteration is below `max_iterations`
- THEN `VerifyResult.Passed` is false
- AND PEV re-enters Execute phase

#### Scenario: Verify command timeout

- GIVEN a verify command exceeds configured timeout
- WHEN Verify phase runs
- THEN the command process is terminated
- AND Verify treats it as failure (non-zero exit)

#### Scenario: Verify command uses argv not shell

- GIVEN `verify_mode` is `commands`
- WHEN a verify command runs
- THEN `exec.CommandContext(ctx, executable, args...)` is used
- AND shell invocation (`sh -c`) is NOT used

#### Scenario: Verify executable or args contain metacharacters

- GIVEN `executable` or any `args` entry contains shell metacharacters (`;`, `|`, `&`, `$`, `` ` ``)
- WHEN configuration is loaded
- THEN configuration validation fails
- AND `CTX_VERIFY_CMD_4012` error is returned

#### Scenario: Verify WorkDir escapes trusted root

- GIVEN `session.WorkDir` resolves outside trusted root after `filepath.Clean`
- WHEN Verify attempts to run commands
- THEN command execution is rejected
- AND `CTX_VERIFY_CMD_4012` error is returned

---

### Requirement: Gateway Token Counter Integration

系统 MUST 通过 `internal/shared/contracts.ITokenCounter` 注入 Gateway 实现的 Token 计数器作为生产默认。

**Priority**: P0
**Rationale**: V1 启发式计数导致压缩触发偏差
**L3 映射**: L3-BE-CTX-01, L3-BE-CTX-02
**L4 映射**: L4-CTX-STATE, L4-CTX-COMPRESS
**L5 映射**: L5-CTX-16

#### Scenario: Token count uses shared contracts interface

- GIVEN `token_counter.source` is `gateway`
- WHEN `CountMessages` or `CountWithSystemPrompt` is called
- THEN the injected `contracts.ITokenCounter` implementation is used
- AND `EncodingForModel` returns the encoding for the configured model

#### Scenario: Compression uses unified counter

- GIVEN messages approach `CompressionTarget`
- WHEN compression trigger is evaluated
- THEN the same `ITokenCounter` instance is used for trigger, steps 1-4, 6, 5, 7, and Autocompact budget

#### Scenario: Heuristic counter for tests only

- GIVEN `token_counter.source` is `heuristic`
- WHEN running in test mode
- THEN char/4 heuristic counter may be used
- AND this mode MUST NOT be the production default

#### Scenario: cl100k_base accuracy on benchmark set

- GIVEN a benchmark text set for cl100k_base-compatible models
- WHEN Gateway counter is compared to reference counts
- THEN per-sample error is within 5%

---

### Requirement: Compression and Verify Observability

系统 MUST 为压缩各步骤、Autocompact 与 Verify 命令发射可观测事件。

**Priority**: P1
**Rationale**: V2 新增 LLM 与 shell 调用，需成本与延迟可追踪
**L4 映射**: L4-CTX-OBS
**L5 映射**: L5-CTX-17

#### Scenario: Compression step spans

- GIVEN compression pipeline runs
- WHEN each step completes
- THEN `ICompressionObserver.EmitCompressionStep` is called with step name and token before/after

#### Scenario: Autocompact observability

- GIVEN autocompact runs successfully or degrades
- WHEN step 6 completes
- THEN `ICompressionObserver.EmitAutocompact` includes model name, latency, and degraded flag

#### Scenario: Verify command observability

- GIVEN verify commands mode runs
- WHEN each command completes
- THEN `ICompressionObserver.EmitVerifyCommand` includes command name, exit code, and duration

---

### Requirement: Real LLM Gateway Wiring

系统 MUST 在主路径（`cmd/devrix`、`devrix-feishu`）注入真实 LLM Gateway，而非 Mock。

**Priority**: P1
**Rationale**: V1 主路径仍用 Mock，V2 需接通 Layer 3
**L3 映射**: L3-BE-CTX-01
**L4 映射**: L4-CTX-STATE
**L5 映射**: L5-CTX-18

#### Scenario: Main entry uses real gateway

- GIVEN `DEVRIX_ENGINE=context` and valid LLM Gateway config
- WHEN devrix processes a user message on main path
- THEN `ILLMGateway.ChatStream` is invoked on the real gateway implementation
- AND Mock LLM is not used in production wiring

#### Scenario: Integration test uses recorded fixture

- GIVEN integration tests without live API keys
- WHEN context gateway flow runs
- THEN recorded LLM fixture or stub gateway may be used
- AND `-tags=live` tests may use real gateway when credentials are present

---

## MODIFIED Requirements

### Requirement: Seven-Step Compression Pipeline

系统 MUST 实现七步压缩管道。V1 执行步骤 1–4 和 7，跳过步骤 6，步骤 5 在步骤 6 之后执行。V2 MUST 在 `autocompact.enabled=true` 且仍超 `CompressionTarget` 时执行步骤 6；否则跳过。

**物理执行顺序（V1/V2 一致）：** 1 → 2 → 3 → 4 → [6] → 5 → 7

**Priority**: P0
**Rationale**: 超长对话必须在 Token 预算内；V2 以 LLM 摘要替代纯硬截断
**L3 映射**: L3-BE-CTX-02
**L4 映射**: L4-CTX-COMPRESS
**L5 映射**: L5-CTX-03, L5-CTX-04, L5-CTX-08, L5-CTX-12

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

#### Scenario: Step 6 Autocompact skipped

- GIVEN `autocompact.enabled` is false **or** V1 implementation
- WHEN pipeline reaches step 6 slot (after step 4, before assembly)
- THEN step is skipped
- AND skip is logged (`autocompact:skipped`)

#### Scenario: Step 6 Autocompact executed

- GIVEN V2 with `autocompact.enabled` true
- AND steps 1-4 applied on message history (no system prompt)
- AND token count still exceeds `CompressionTarget`
- WHEN step 6 runs
- THEN Autocompact per ADDED Requirement replaces middle segment with summary
- AND pipeline proceeds to step 5 Assembly

#### Scenario: Step 5 System Prompt Assembly

- GIVEN compressed messages and system prompt
- WHEN Assembly runs (after step 6 slot)
- THEN system prompt is first
- AND messages remain chronological

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

系统 MUST 实现 PEV 循环：Execute → Verify（无 Plan 阶段）。V1 默认 `verify_mode: basic`；V2 MUST  additionally support `verify_mode: commands` 与 `none`。

**Priority**: P0
**Rationale**: 工具调用需要执行与可配置验证闭环
**L4 映射**: L4-CTX-PEV
**L5 映射**: L5-CTX-06, L5-CTX-07, L5-CTX-11, L5-CTX-14, L5-CTX-15

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

#### Scenario: Verify phase commands mode

- GIVEN `verify_mode` is `commands`
- WHEN Verify runs after tool execution
- THEN verify commands run per ADDED Requirement using executable+args
- AND result feeds PEV retry logic

#### Scenario: PEV max iterations exceeded

- GIVEN Verify fails repeatedly
- WHEN iteration reaches `max_iterations`
- THEN `PEVMaxIterations` error is returned
- AND partial result is preserved in context

---

## REMOVED Requirements

(None — V1 requirements remain valid; V2 extends behavior.)
