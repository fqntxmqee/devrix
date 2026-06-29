# Delta: Domain D2 (CTX)

**Change ID:** devrix-foundation → devrix-queryloop-context (archived) → devrix-d2-queryloop-dismantle (archived) → **devrix-d2-structure-closure** (v8.0.0 → **v8.2.0**) → **devrix-d2-dsaft-restructuring** (v8.2.0 → v8.3.0)
**Canonical spec:** `openspec/specs/d2-context-engine/spec.md` (v8.3.0)
**Status:** v8.3.0 — D2 v2.2 structure closure + DSAFT restructuring (DM-20260629-002 PR-1..PR-5)
**Affects:** D7 turn runtime, compression pipeline, layered memory, conversation repair, main transcript, **enforce/tools/ rename (P3-T2)**, **memory split (P4)**, **legacy retirement (P5)**, **god fn split (PR-2/PR-3)**, **registry sync (PR-4)**, **value flow rename (PR-5)**

---

## Canonical S → ValueFlow Alias (DM-20260629-002 PR-5)

| Canonical S | ValueFlow Alias (用户感知) |
|-------------|------------------------------|
| D2-S15 PrepareExecutionContext | `D2_Context_Loading_Compression` |
| D2-S16 RunQueryLoop | (REMOVED DM-20260618-010 → 归 D7 ValueFlow) |
| D2-S17 PersistSessionState | `D2_Session_State_Persistence` |
| D2-S18 EnforceExecutionPolicy | `D2_Tool_Permission_Sandbox` |

> 与 `d2-domain.md` §North Star + `a-registry.md` §Canonical S + `f-registry.md` §D2-S15/S17/S18 + `t-registry.md` §Canonical T 映射 一致。

---

## REMOVED (DM-20260618-010)

- `query.Loop.Run` and `query_loop.enabled` config
- Runtime label renamed: `PathD7Turn` (`d7_turn`) replaces `PathQueryLoop`

---

## ADDED

### Requirement: D7 Turn Primary Path

`ContextEngine.Process` MUST delegate LLM↔Tool rounds to D7 `PreparedTurnRunner` and record `obsruntime.PathD7Turn`.

#### Scenario: Multi-turn tool loop via D7
- GIVEN LLM returns tool_use until final text
- WHEN Process runs
- THEN D7 RunTurn continues until no pending tool calls or `max_turns` reached

#### Scenario: Per-turn compression
- GIVEN `turn_runtime.compress_per_turn=true` (default)
- WHEN Process starts
- THEN entry compression is skipped (`skipEntryCompress`)
- AND `commitActiveWindow` runs messages-only seven-step pipeline after each successful turn when budget exceeded

#### Scenario: Deferred complete event
- GIVEN D7 turn completes successfully
- WHEN snapshot and `sc.Messages` are persisted
- THEN `complete` EngineEvent is emitted only after durable writes

### Requirement: Conversation Integrity

#### Scenario: Repair tool message chain before LLM
- GIVEN snapshot messages contain orphan tool results or incomplete tool rounds
- WHEN Process prepares API messages
- THEN `conversation.RepairToolMessageChain` removes invalid pairs

#### Scenario: Compact boundary preserves tail
- GIVEN prior compaction inserted a `compact_boundary` system marker
- WHEN messages are read for LLM or compression
- THEN only messages after the last boundary are used

### Requirement: Main Thread Transcript

When `context_engine.main_transcript.enabled=true`, full message deltas MUST append to `{base_dir}/{sessionId}/transcript.jsonl`.

### Requirement: System Prompt Assembly

Four-layer assembly via `prompt.SystemPromptAssembler`.

### Requirement: Seven-Step Compression Pipeline

Physical order: 1 → 2 → 3 → 4 → [6] → 5 → 7. Operates on **messages only**.

---

## Historical (pre-20260618)

<details>
<summary>Legacy QueryLoop / Harness requirements (superseded)</summary>

Prior versions routed Process through `query.Loop.Run` when `query_loop.enabled=true`. Harness bootstrap ran when `query_loop.enabled=false`. Both paths removed in v8.0.0.

</details>

---

## v8.0.0 → v8.2.0 (DM-20260619-007 devrix-d2-structure-closure)

### Requirement: P3 enforce 归位

- `enforce/toolrunner/` → `enforce/tools/` (scenario slug rename, package `package tools`)
- `enforce/sandbox/` 物理迁入 `enforce/` 子树（保留 `sandbox/` 子目录名）
- 删除 `enforce/orchestrator.go`（92 行 PolicyOrchestrator stub，由 `turn_adapter.ExecuteRound` 接管）
- D2-STRUCT-T01..T03 layout guards activated in `internal/lint/layer/d2_layout_test.go`

#### Scenario: 旧 toolrunner package name 残留
- GIVEN `enforce/tools/` 目录下任一 `.go` 文件首行 `package toolrunner`
- WHEN `go test ./internal/lint/layer/...`
- THEN D2-STRUCT-T03 fails

### Requirement: P4 memory 读写分离

- `prepare/memory/longterm.go` (combined ILongTermMemory) → `prepare/memory/recall.go` (S15-A02 read-side port) + `persist/memory/store.go` (S17-A03 SQLite impl)
- `MemoryEntry` 提升至 `internal/shared/types/memory.go`（共享类型，无域拥有）
- `LongTermRecaller` / `LongTermStore` 端口提升至 `internal/shared/contracts/memory.go`
- `prepare/memory.Manager` 字段从 `longTerm ILongTermMemory` 拆为 `recaller` + `writer` 两个独立注入
- D2-STRUCT-T04 cyclic-import guard activated

#### Scenario: prepare/memory ↔ persist/memory 循环导入
- GIVEN 任一 `prepare/memory/*.go` 文件 import `persist/memory` (反之亦然)
- WHEN `go test ./internal/lint/layer/...`
- THEN D2-STRUCT-T04 fails

### Requirement: P5 legacy 退役

- `facade/` → `legacy/` (物理迁移 + 包名 `package legacy`)
- `legacy.ContextEngine.Process()` 标记 `// Deprecated:` 注释 + 运行时 `slog.Warn`
- `contextengine.ContextEngine` / `EngineDeps` / `NewContextEngine` 重新导出为 `legacy.*` 的 type aliases，保留 `// Deprecated:` 注释
- D2-STRUCT-T07 no-new-legacy-Process-callers guard activated (allowlist: cmd/llm-smoke, multiagent/run, tests/*, communication mocks)
- legacy/ 目录删除触发条件: 所有 Process caller 已迁 D7 路径 + 集成测试全绿持续 ≥7 天 (AC-P5-4)

#### Scenario: 新增生产代码调用 legacy.Process()
- GIVEN 任意 `cmd/` / `internal/` 下非白名单生产文件包含 `.Process(ctx` 调用
- WHEN `go test ./internal/lint/layer/...`
- THEN D2-STRUCT-T07 fails，提示迁移到 D7 SessionOrchestrator 或 turn_adapter.ExecuteRound
