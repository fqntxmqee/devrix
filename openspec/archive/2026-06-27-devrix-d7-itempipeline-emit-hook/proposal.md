# Proposal: D7 ItemPipelineRunner emit hook (hotfix)

**Change ID:** `devrix-d7-itempipeline-emit-hook`
**Demand ID:** DM-20260627-001
**Priority:** P0
**Sprint:** d7-hotfix-2026-06-27
**PR Count:** 1
**Status:** S7_Archived
**SoT:** 用户飞书指令 "review d2领域代码" 触发的 hotfix investigation

---

## 1. Background

D7 编排层在 DM-20260626-009 把 ItemPipelineRunner 设为 per-WorkItem ReAct loop 默认开启路径。该路径下 DefaultWorkItemExecutor 直接驱动 LLM↔Tool 迭代，但 emit 链路在 ItemPipelineRunner 层断掉：

- SessionOrchestrator 层有 emit（写 D1 gateway 的 EngineEvent）
- Wave path 有 emit（OrchestratePath.Run → subagent.streamEmit）
- ItemPipelineRunner path **没有 emit** → DefaultWorkItemExecutor.Emit = nil → 静默丢事件

## 2. Problem Statement

飞书卡片看不到 tools 列表，也看不到 LLM 文本响应（其实是 tool_call/text/thinking 事件全被丢）。D7 orchestration 的所有 instrumentation 都失效。

## 3. Proposed Solution

补 ItemPipelineRunner emit chain，3 个改动点 + 2 个测试 + 1 个 coverage 配套 fix：

### 3.1 代码改动

1. `DefaultWorkItemExecutor` 加 `Emit func(*EngineEvent)` 字段 + emit 助手方法
2. `ItemPipelineRunner` 加 `Emit func(*EngineEvent)` 字段 + propagation
3. `session_turn_loop.go` goroutine 内 emitFn wrapper 设置 SessionID

### 3.2 测试

- `TestWorkItemExecutor_EmitHook_Hotfix_2026_06_27` — happy path sequence (text→thinking→tool_call→tool_result→text)
- `TestWorkItemExecutor_NilEmit_NoOp` — legacy safety

### 3.3 Coverage 配套

`spans.go` 加 3 个 inner observability span (`D7_Worktree_Op` / `D7_SubWorktree_Run` / `D7_SubTurn_Iteration`)，但忘了同步：

- `coverage/registry_test.go` expected 列表（+3 → 84）
- `telemetry/names.go` LayerAndComponent 前缀匹配（worktree + executor 组件）

## 4. Scope

- **改了**：5 file +300 (initial) → 7 file +318 (after coverage fix)
- **没改**：runtime 逻辑、API 契约、t-registry（D5 coverage fix 仅期望列表）

## 5. Risk

低：纯增量字段 + 显式 nil bridge；emit hook 缺失路径 = nil → no-op（与现状一致），新路径 = 沿用 session-level emit。

## 6. Related Work

- 前序 PR #251-255：per-WorkItem ReAct loop + 5-node MUPS span + 3 inner span + finishReason thread（缺 emit hook）
- DM-20260626-009：default-on 切换

## 7. Hotfix Path Rationale

按 `feedback-devrix-bugfix-skip-openspec.md`：小 bug 跳过 S1-S6 完整流程；code+tests+commit → **立即 build+restart**让用户验收 → 验收后再补归档。

本次：
1. 09:15 — commit (5d74a4b) → build + restart
2. 09:32 — 用户飞书确认 "tools有了"（emit hook 生效）
3. 09:45 — coverage test fix 后续 commit (da7d1b6 → 8251a18) + rebase + auto-merge
4. 09:46 — S7_Archived 补归档

PR #257 走的也是 hotfix 路径，跳过 S1-S6 完整流程。